// Package azurlanelua loads the restricted Lua data dialect used by
// AzurLaneLuaScripts. It intentionally does not execute Lua code.
package azurlanelua

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"
)

// OrderedObject retains Lua table field order for serializers that need it.
// Transformations explicitly convert it to plain maps before sorting records.
type OrderedObject struct {
	Keys           []string
	Values         map[string]any
	HasImplicitKey bool
}

func (o OrderedObject) Set(key string, value any) {
	if o.Values == nil {
		o.Values = map[string]any{}
	}
	if _, exists := o.Values[key]; !exists {
		o.Keys = append(o.Keys, key)
	}
	o.Values[key] = value
}

func (o OrderedObject) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, key := range o.Keys {
		if i > 0 {
			b.WriteByte(',')
		}
		keyBytes, _ := json.Marshal(key)
		b.Write(keyBytes)
		b.WriteByte(':')
		valueBytes, err := marshalNoHTML(o.Values[key])
		if err != nil {
			return nil, err
		}
		b.Write(valueBytes)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

func marshalNoHTML(value any) ([]byte, error) {
	if _, ok := value.(OrderedObject); ok {
		return json.Marshal(value)
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(b.Bytes(), []byte{'\n'}), nil
}

func ToPlain(value any) any {
	switch v := value.(type) {
	case OrderedObject:
		if v.HasImplicitKey {
			if len(v.Values) == 0 {
				return []any{}
			}
			max := len(v.Values)
			arr := make([]any, max)
			valid := max > 0
			for i := 1; i <= max; i++ {
				child, ok := v.Values[strconv.Itoa(i)]
				if !ok {
					valid = false
					break
				}
				arr[i-1] = ToPlain(child)
			}
			if valid {
				return arr
			}
		}
		m := make(map[string]any, len(v.Values))
		for key, child := range v.Values {
			if child == nil {
				continue
			}
			if function, ignored := child.(ignoredLuaFunction); ignored {
				m[key] = function.comment
				continue
			}
			m[key] = ToPlain(child)
		}
		return m
	case []any:
		out := make([]any, len(v))
		for i, child := range v {
			out[i] = ToPlain(child)
		}
		return out
	default:
		return value
	}
}

// ToPlainOrdered converts loader values while retaining explicit Lua field
// order for serializers that reproduce aggregate GameCfg JSON byte-for-byte.
func ToPlainOrdered(value any) any {
	switch v := value.(type) {
	case OrderedObject:
		if v.HasImplicitKey {
			if len(v.Values) == 0 {
				return []any{}
			}
			max := len(v.Values)
			arr := make([]any, max)
			valid := true
			for i := 1; i <= max; i++ {
				child, ok := v.Values[strconv.Itoa(i)]
				if !ok {
					valid = false
					break
				}
				arr[i-1] = ToPlainOrdered(child)
			}
			if valid {
				return arr
			}
		}
		out := OrderedObject{Values: map[string]any{}}
		for _, key := range v.Keys {
			child := v.Values[key]
			if child == nil {
				continue
			}
			if function, ignored := child.(ignoredLuaFunction); ignored {
				out.Keys = append(out.Keys, key)
				out.Values[key] = function.comment
				continue
			}
			out.Keys = append(out.Keys, key)
			out.Values[key] = ToPlainOrdered(child)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, child := range v {
			out[i] = ToPlainOrdered(child)
		}
		return out
	default:
		return value
	}
}

// RestoreOrder applies source table order to a transformed value while
// retaining fields introduced by the transformation (for example id).
func RestoreOrder(original, transformed any) any {
	switch source := original.(type) {
	case OrderedObject:
		if target, ok := transformed.(map[string]any); ok {
			out := OrderedObject{Values: map[string]any{}}
			for _, key := range source.Keys {
				if child, exists := target[key]; exists {
					out.Set(key, RestoreOrder(source.Values[key], child))
				}
			}
			keys := make([]string, 0, len(target))
			for key := range target {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				if _, exists := out.Values[key]; !exists {
					out.Set(key, target[key])
				}
			}
			return out
		}
		if target, ok := transformed.([]any); ok {
			out := make([]any, len(target))
			for i, child := range target {
				key := ""
				if rec, ok := child.(map[string]any); ok {
					if id, ok := rec["id"].(float64); ok {
						key = strconv.FormatFloat(id, 'f', -1, 64)
					}
				}
				if sourceChild, exists := source.Values[key]; exists {
					out[i] = RestoreOrder(sourceChild, child)
				} else {
					out[i] = child
				}
			}
			return out
		}
	case []any:
		if target, ok := transformed.([]any); ok {
			out := make([]any, len(target))
			for i, child := range target {
				if i < len(source) {
					out[i] = RestoreOrder(source[i], child)
				} else {
					out[i] = child
				}
			}
			return out
		}
	}
	return transformed
}

type token struct {
	kind, text string
	pos        int
}

type lexer struct {
	src []rune
	pos int
}

func lex(src []byte) ([]token, error) {
	l := &lexer{src: []rune(string(src))}
	var out []token
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		if unicode.IsSpace(c) {
			l.pos++
			continue
		}
		if c == '-' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '-' {
			l.pos += 2
			for l.pos < len(l.src) && l.src[l.pos] != '\n' {
				l.pos++
			}
			continue
		}
		if c == '[' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '[' {
			start := l.pos + 2
			l.pos = start
			for l.pos+1 < len(l.src) && !(l.src[l.pos] == ']' && l.src[l.pos+1] == ']') {
				l.pos++
			}
			if l.pos+1 >= len(l.src) {
				return nil, fmt.Errorf("unterminated Lua long string")
			}
			long := strings.ReplaceAll(string(l.src[start:l.pos]), "\r\n", "\n")
			long = strings.TrimPrefix(long, "\n")
			out = append(out, token{kind: "string", text: long, pos: start})
			l.pos += 2
			continue
		}
		if unicode.IsDigit(c) || (c == '.' && l.pos+1 < len(l.src) && unicode.IsDigit(l.src[l.pos+1])) {
			start := l.pos
			l.pos++
			for l.pos < len(l.src) && (unicode.IsDigit(l.src[l.pos]) || strings.ContainsRune(".eE+-", l.src[l.pos])) {
				if (l.src[l.pos] == '+' || l.src[l.pos] == '-') && l.pos > start && l.src[l.pos-1] != 'e' && l.src[l.pos-1] != 'E' {
					break
				}
				l.pos++
			}
			out = append(out, token{kind: "number", text: string(l.src[start:l.pos]), pos: start})
			continue
		}
		if strings.ContainsRune("{}[](),.=-~<>!&|+*/%;:#", c) {
			out = append(out, token{kind: string(c), text: string(c), pos: l.pos})
			l.pos++
			continue
		}
		if c == '\'' || c == '"' {
			s, err := l.string(c)
			if err != nil {
				return nil, err
			}
			out = append(out, token{kind: "string", text: s, pos: l.pos})
			continue
		}
		if unicode.IsLetter(c) || c == '_' {
			start := l.pos
			l.pos++
			for l.pos < len(l.src) && (unicode.IsLetter(l.src[l.pos]) || unicode.IsDigit(l.src[l.pos]) || l.src[l.pos] == '_') {
				l.pos++
			}
			out = append(out, token{kind: "ident", text: string(l.src[start:l.pos]), pos: start})
			continue
		}
		return nil, fmt.Errorf("unsupported Lua character %q at byte-rune offset %d", c, l.pos)
	}
	return out, nil
}

func (l *lexer) string(quote rune) (string, error) {
	l.pos++
	var b strings.Builder
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		l.pos++
		if c == quote {
			return b.String(), nil
		}
		if c != '\\' {
			b.WriteRune(c)
			continue
		}
		if l.pos >= len(l.src) {
			break
		}
		e := l.src[l.pos]
		l.pos++
		switch e {
		case 'a':
			b.WriteByte('\a')
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case 'v':
			b.WriteByte('\v')
		case '\\':
			b.WriteByte('\\')
		case '\'':
			b.WriteByte('\'')
		case '"':
			b.WriteByte('"')
		default:
			if unicode.IsDigit(e) {
				digits := []rune{e}
				for len(digits) < 3 && l.pos < len(l.src) && unicode.IsDigit(l.src[l.pos]) {
					digits = append(digits, l.src[l.pos])
					l.pos++
				}
				n, err := strconv.Atoi(string(digits))
				if err != nil {
					return "", err
				}
				b.WriteByte(byte(n))
				continue
			}
			b.WriteRune(e)
		}
	}
	return "", fmt.Errorf("unterminated Lua string")
}

type parser struct {
	tokens    []token
	i         int
	constants map[string]any
	source    []rune
}

var constantsCache sync.Map

type ignoredLuaFunction struct{ comment string }

func stringifyKey(v any) string {
	if n, ok := v.(float64); ok {
		return strconv.FormatFloat(n, 'f', -1, 64)
	}
	if n, ok := v.(json.Number); ok {
		if n == "-0" || n == "-0.0" {
			return "0"
		}
		if parsed, err := strconv.ParseFloat(string(n), 64); err == nil && math.Trunc(parsed) == parsed {
			return strconv.FormatFloat(parsed, 'f', -1, 64)
		}
		return string(n)
	}
	return fmt.Sprint(v)
}

func (p *parser) parseValue() (any, error) {
	value, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	if p.i < len(p.tokens) {
		next := p.tokens[p.i]
		if next.kind == "ident" && next.text == "or" {
			p.i++
			right, rightErr := p.parseValue()
			if rightErr != nil {
				return nil, rightErr
			}
			if luaTruthy(value) {
				return value, nil
			}
			return right, nil
		}
		if isExpressionOperator(next) || (next.kind == "ident" && next.text == "and") {
			return nil, p.errorAt(next, "unsupported Lua expression operator %q", next.text)
		}
	}
	return value, nil
}

func (p *parser) parsePrimary() (any, error) {
	if p.i >= len(p.tokens) {
		return nil, fmt.Errorf("expected Lua value")
	}
	t := p.tokens[p.i]
	switch t.kind {
	case "-":
		p.i++
		v, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		if n, ok := v.(json.Number); ok {
			return json.Number("-" + string(n)), nil
		}
		n, ok := v.(float64)
		if !ok {
			return nil, fmt.Errorf("unary minus requires number")
		}
		return -n, nil
	case "string":
		p.i++
		return t.text, nil
	case "number":
		p.i++
		n, err := strconv.ParseFloat(t.text, 64)
		if err != nil {
			return nil, fmt.Errorf("number %q: %w", t.text, err)
		}
		if json.Valid([]byte(t.text)) {
			return json.Number(t.text), nil
		}
		return n, nil
	case "ident":
		p.i++
		if t.text == "function" {
			return p.parseFunction(t)
		}
		switch t.text {
		case "true":
			return true, nil
		case "false":
			return false, nil
		case "nil":
			return nil, nil
		}
		name := t.text
		for p.i+1 < len(p.tokens) && p.tokens[p.i].kind == "." && p.tokens[p.i+1].kind == "ident" {
			name += "." + p.tokens[p.i+1].text
			p.i += 2
		}
		if value, ok := p.constants[name]; ok && structuredConstant(value) {
			return value, nil
		}
		if value, ok := p.constants[name]; ok {
			return value, nil
		}
		if strings.HasPrefix(name, "slot0.") {
			if value, ok := p.constants["ShipType."+strings.TrimPrefix(name, "slot0.")]; ok && structuredConstant(value) {
				return value, nil
			}
		}
		if p.i < len(p.tokens) && p.tokens[p.i].kind == "(" {
			return p.parseCall(name, t)
		}
		if isBootstrapReference(name) && (p.nextIsOr() || isLocalAlias(name)) {
			return nil, nil
		}
		return nil, p.errorAt(t, "unresolved Lua identifier %q", name)
	case "{":
		return p.parseTable()
	default:
		return nil, fmt.Errorf("unsupported Lua value token %q", t.text)
	}
}

type luaBlock struct {
	kind   string
	loopDo bool
}

func (p *parser) parseFunction(start token) (any, error) {
	blocks := []luaBlock{{kind: "function"}}
	firstEnd := -1
	for p.i < len(p.tokens) {
		current := p.tokens[p.i]
		if current.kind == "ident" {
			switch current.text {
			case "function", "if", "for", "while", "repeat":
				blocks = append(blocks, luaBlock{kind: current.text})
			case "do":
				last := &blocks[len(blocks)-1]
				if (last.kind == "for" || last.kind == "while") && !last.loopDo {
					last.loopDo = true
				} else {
					blocks = append(blocks, luaBlock{kind: "do"})
				}
			case "until":
				if len(blocks) == 0 || blocks[len(blocks)-1].kind != "repeat" {
					return nil, p.errorAt(current, "unexpected Lua %q", current.text)
				}
				blocks = blocks[:len(blocks)-1]
			case "end":
				if len(blocks) == 0 {
					return nil, p.errorAt(current, "unexpected Lua end")
				}
				if firstEnd < 0 {
					firstEnd = current.pos + len("end")
				}
				blocks = blocks[:len(blocks)-1]
				p.i++
				if len(blocks) == 0 {
					return ignoredLuaFunction{comment: formatFunctionComment(p.source, start.pos, firstEnd)}, nil
				}
				continue
			}
		}
		p.i++
	}
	return nil, p.errorAt(start, "unterminated Lua function expression")
}

func (p *parser) parseCall(name string, call token) (any, error) {
	switch name {
	case "Vector3":
		return p.parseVector3(call)
	case "rawget":
		p.i++
		if err := p.skipCallArguments(name, call); err != nil {
			return nil, err
		}
		return nil, nil
	case "setmetatable":
		return p.parseSetMetatable(call)
	default:
		return nil, p.errorAt(call, "unsupported Lua function call %q", name)
	}
}

func (p *parser) parseVector3(call token) (any, error) {
	p.i++
	args := []any{}
	if p.i < len(p.tokens) && p.tokens[p.i].kind == ")" {
		p.i++
		return args, nil
	}
	for {
		if p.i >= len(p.tokens) || p.tokens[p.i].kind == ")" {
			return nil, p.errorAt(call, "incomplete Vector3 call")
		}
		arg, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
		if p.i >= len(p.tokens) {
			return nil, p.errorAt(call, "unterminated Vector3 call")
		}
		if p.tokens[p.i].kind == ")" {
			p.i++
			return args, nil
		}
		if p.tokens[p.i].kind != "," {
			return nil, p.errorAt(p.tokens[p.i], "expected ',' or ')' in Vector3 call")
		}
		p.i++
		if p.i < len(p.tokens) && p.tokens[p.i].kind == ")" {
			p.i++
			return args, nil
		}
	}
}

func (p *parser) parseSetMetatable(call token) (any, error) {
	p.i++
	_, err := p.parseValue()
	if err != nil {
		return nil, err
	}
	if p.i >= len(p.tokens) {
		return nil, p.errorAt(call, "unterminated setmetatable call")
	}
	if p.tokens[p.i].kind == ")" {
		return nil, p.errorAt(call, "setmetatable requires a metatable argument")
	}
	if p.tokens[p.i].kind != "," {
		return nil, p.errorAt(p.tokens[p.i], "expected ',' in setmetatable call")
	}
	p.i++
	if p.i >= len(p.tokens) || p.tokens[p.i].kind == ")" {
		return nil, p.errorAt(call, "setmetatable requires a metatable argument")
	}
	if err := p.skipCallArguments("setmetatable", call); err != nil {
		return nil, err
	}
	// The metatable wrapper initializes a pg dataset; it must not be mistaken for
	// generated records before the actual fields are assigned.
	return nil, nil
}

func (p *parser) skipCallArguments(name string, call token) error {
	stack := []string{"("}
	for p.i < len(p.tokens) {
		current := p.tokens[p.i]
		switch current.kind {
		case "(", "{", "[":
			stack = append(stack, current.kind)
		case ")", "}", "]":
			expected := map[string]string{")": "(", "}": "{", "]": "["}[current.kind]
			if len(stack) == 0 || stack[len(stack)-1] != expected {
				return p.errorAt(current, "mismatched delimiter in %s call", name)
			}
			stack = stack[:len(stack)-1]
			p.i++
			if len(stack) == 0 {
				return nil
			}
			continue
		}
		p.i++
	}
	return p.errorAt(call, "unterminated %s call", name)
}

func luaTruthy(value any) bool {
	if value == nil {
		return false
	}
	if boolean, ok := value.(bool); ok {
		return boolean
	}
	return true
}

func isExpressionOperator(token token) bool {
	switch token.kind {
	case "+", "-", "*", "/", "%", "^", "~", "<", ">", "=", "&", "|", ".", ":", "#":
		return true
	default:
		return false
	}
}

func isBootstrapReference(name string) bool {
	for _, prefix := range []string{"pg", "_G", "uv0"} {
		if name == prefix || strings.HasPrefix(name, prefix+".") {
			return true
		}
	}
	return isLocalAlias(name)
}

func isLocalAlias(name string) bool {
	if !strings.HasPrefix(name, "slot") {
		return false
	}
	suffix := strings.TrimPrefix(name, "slot")
	if suffix == "" {
		return false
	}
	for _, char := range suffix {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func (p *parser) nextIsOr() bool {
	return p.i < len(p.tokens) && p.tokens[p.i].kind == "ident" && p.tokens[p.i].text == "or"
}

func (p *parser) errorAt(token token, format string, args ...any) error {
	return fmt.Errorf("%s: %s", p.position(token.pos), fmt.Sprintf(format, args...))
}

func (p *parser) errorAtCurrent(format string, args ...any) error {
	if p.i < len(p.tokens) {
		return p.errorAt(p.tokens[p.i], format, args...)
	}
	return fmt.Errorf("%s: %s", p.position(len(p.source)), fmt.Sprintf(format, args...))
}

func (p *parser) currentToken() string {
	if p.i >= len(p.tokens) {
		return "<eof>"
	}
	return fmt.Sprintf("%q", p.tokens[p.i].text)
}

func (p *parser) position(pos int) string {
	if pos < 0 {
		pos = 0
	}
	if pos > len(p.source) {
		pos = len(p.source)
	}
	line, column := 1, 1
	for _, char := range p.source[:pos] {
		if char == '\n' {
			line, column = line+1, 1
		} else {
			column++
		}
	}
	return fmt.Sprintf("line %d, column %d", line, column)
}

func structuredConstant(value any) bool {
	switch value.(type) {
	case []any, map[string]any, OrderedObject:
		return true
	default:
		return false
	}
}

func (p *parser) parseTable() (any, error) {
	p.i++
	arr := []any{}
	obj := OrderedObject{Values: map[string]any{}}
	hasKey := false
	for p.i < len(p.tokens) && p.tokens[p.i].kind != "}" {
		if p.tokens[p.i].kind == "," {
			p.i++
			continue
		}
		keyKind := "array"
		key := ""
		if p.tokens[p.i].kind == "[" {
			keyToken := p.tokens[p.i]
			p.i++
			v, err := p.parseValue()
			if err != nil {
				return nil, p.errorAt(keyToken, "table key: %v", err)
			}
			if p.i >= len(p.tokens) || p.tokens[p.i].kind != "]" {
				return nil, p.errorAtCurrent("missing ]")
			}
			p.i++
			if p.i >= len(p.tokens) || p.tokens[p.i].kind != "=" {
				return nil, p.errorAtCurrent("missing = after bracket key")
			}
			p.i++
			keyKind = "bracket"
			key = stringifyKey(v)
		} else if (p.tokens[p.i].kind == "ident" || p.tokens[p.i].kind == "string") && p.i+1 < len(p.tokens) && p.tokens[p.i+1].kind == "=" {
			keyKind = "field"
			key = p.tokens[p.i].text
			p.i += 2
		}
		v, err := p.parseValue()
		if err != nil {
			return nil, fmt.Errorf("table value at token %d (%s): %w", p.i, p.currentToken(), err)
		}
		if keyKind == "array" {
			arr = append(arr, v)
			obj.HasImplicitKey = true
		} else {
			hasKey = true
			obj.Set(key, v)
		}
	}
	if p.i >= len(p.tokens) || p.tokens[p.i].kind != "}" {
		return nil, p.errorAtCurrent("missing }")
	}
	p.i++
	if !hasKey {
		for _, value := range arr {
			if value == nil {
				sparse := OrderedObject{Values: map[string]any{}, HasImplicitKey: true}
				for i, child := range arr {
					if child != nil {
						sparse.Set(strconv.Itoa(i+1), child)
					}
				}
				return sparse, nil
			}
		}
		return arr, nil
	}
	for i, v := range arr {
		obj.Set(strconv.Itoa(i+1), v)
	}
	return obj, nil
}

// LoadFile reads assignments below _G.pg.base or pg.base and returns the
// named dataset. Numeric keys are retained as decimal string keys so the
// converter can apply deterministic id-list transformations.
func LoadFile(path string) (any, error) {
	return loadFile(path, "")
}

func LoadFileWithConstantsRoot(path, constantsRoot string) (any, error) {
	if strings.TrimSpace(constantsRoot) == "" {
		return nil, fmt.Errorf("constants root is required")
	}
	return loadFile(path, constantsRoot)
}

func loadFile(path, constantsRoot string) (any, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return []any{}, nil
	}
	ts, err := lex(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	constants, err := loadConstants(constantsRoot)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	aliasDataset := ""
	if i := strings.Index(string(b), "slot0."); i >= 0 {
		rest := string(b)[i+6:]
		if j := strings.Index(rest, " = {}"); j > 0 {
			aliasDataset = strings.TrimSpace(rest[:j])
		}
	}
	root := map[string]any{}
	p := &parser{tokens: ts, constants: constants, source: []rune(string(b))}
	for p.i < len(ts) {
		start := p.i
		if ts[p.i].kind != "ident" || (ts[p.i].text != "_G" && ts[p.i].text != "pg" && !(ts[p.i].text == "uv0" && aliasDataset != "")) {
			p.i++
			continue
		}
		p.i++
		parts := []string{ts[start].text}
		if ts[start].text == "uv0" {
			parts = []string{"pg", aliasDataset}
		}
		for p.i < len(ts) && ts[p.i].kind == "." {
			p.i++
			if p.i >= len(ts) || ts[p.i].kind != "ident" {
				p.i = start + 1
				break
			}
			parts = append(parts, ts[p.i].text)
			p.i++
		}
		if len(parts) < 2 || (parts[0] != "pg" && (len(parts) < 2 || parts[1] != "pg")) {
			p.i = start + 1
			continue
		}
		datasetIndex := 1
		if (parts[0] == "_G" && len(parts) > 2 && parts[2] == "base") || (parts[0] == "pg" && len(parts) > 1 && parts[1] == "base") {
			datasetIndex = 3
			if parts[0] == "pg" {
				datasetIndex = 2
			}
		}
		for p.i < len(ts) && ts[p.i].kind == "." {
			p.i++
			if p.i >= len(ts) || ts[p.i].kind != "ident" {
				break
			}
			parts = append(parts, ts[p.i].text)
			p.i++
		}
		for p.i < len(ts) && ts[p.i].kind == "[" {
			keyToken := ts[p.i]
			p.i++
			v, e := p.parseValue()
			if e != nil {
				return nil, fmt.Errorf("%s: assignment near token %d (%s): table key at %s: %v", path, start, p.position(ts[start].pos), p.position(keyToken.pos), e)
			}
			if p.i >= len(ts) || ts[p.i].kind != "]" {
				return nil, fmt.Errorf("%s: assignment near token %d (%s): missing ]", path, start, p.position(ts[start].pos))
			}
			p.i++
			parts = append(parts, stringifyKey(v))
		}
		if p.i >= len(ts) || ts[p.i].kind != "=" {
			p.i = start + 1
			continue
		}
		p.i++
		if p.i < len(ts) && ts[p.i].kind == "ident" && ts[p.i].text == "pg" && p.i+1 < len(ts) && ts[p.i+1].kind == "." {
			p.i = start + 1
			continue
		}
		v, e := p.parseValue()
		if e != nil {
			return nil, fmt.Errorf("%s assignment near token %d: %w", path, start, e)
		}
		if len(parts) > datasetIndex {
			dataset := parts[datasetIndex]
			if dataset == "base" {
				continue
			}
			if len(parts) == datasetIndex+1 {
				root[dataset] = v
			} else {
				m, ok := root[dataset].(OrderedObject)
				if !ok {
					m = OrderedObject{Values: map[string]any{}}
				}
				m.Set(parts[len(parts)-1], v)
				root[dataset] = m
			}
		}
	}
	if len(root) == 0 {
		var returnErr error
		for i, token := range ts {
			if token.kind == "ident" && token.text == "return" {
				p.i = i + 1
				value, parseErr := p.parseValue()
				if parseErr == nil {
					return value, nil
				}
				returnErr = parseErr
			}
		}
		if returnErr != nil {
			return nil, fmt.Errorf("no pg.base assignments in %s: return parse: %w", path, returnErr)
		}
		return nil, fmt.Errorf("no pg.base assignments in %s", path)
	}
	if len(root) == 1 {
		for _, v := range root {
			if m, ok := v.(OrderedObject); ok && m.Values["__name"] != nil {
				return []any{}, nil
			}
			return v, nil
		}
	}
	return root, nil
}

func loadConstants(constantsRoot string) (map[string]any, error) {
	if strings.TrimSpace(constantsRoot) == "" {
		return defaultConstants(), nil
	}
	if cached, ok := constantsCache.Load(constantsRoot); ok {
		return cached.(map[string]any), nil
	}
	constants := defaultConstants()
	globalPath := filepath.Join(constantsRoot, "const.lua")
	if err := loadGlobalConstants(constants, globalPath); err != nil {
		return nil, err
	}
	constPath := filepath.Join(constantsRoot, "model", "const", "shiptype.lua")
	if err := loadShipTypeConstants(constants, constPath); err != nil {
		return nil, err
	}
	constantsCache.Store(constantsRoot, constants)
	return constants, nil
}

func defaultConstants() map[string]any {
	return map[string]any{
		"ShipType.MainShipType":  []any{json.Number("4"), json.Number("5"), json.Number("6"), json.Number("7"), json.Number("10"), json.Number("12"), json.Number("13"), json.Number("21"), json.Number("24")},
		"STORY_EVENT.TEST":       "story event test",
		"STORY_EVENT.TEST_DONE":  "story event test done",
		"AspectMode.FitInParent": "FitInParent",
		"TRUE":                   true,
		"FALSE":                  false,
		"ship_unlock":            nil,
		"lv_max":                 nil,
		// These source-level labels are intentionally runtime-only nil values.
		"healthy":                        nil,
		"sub_move":                       nil,
		"defaultID":                      nil,
		"itemID":                         nil,
		"special_goods_list":             nil,
		"special_discount_list":          nil,
		"equipskin_discount":             nil,
		"equipskin_discount_2":           nil,
		"undefined":                      nil,
		"times":                          nil,
		"exchange":                       nil,
		"hour":                           nil,
		"random_buff":                    nil,
		"wash":                           nil,
		"a":                              nil,
		"map_call":                       nil,
		"sleep":                          nil,
		"playername":                     nil,
		"dance":                          nil,
		"area_scout":                     nil,
		"missile":                        nil,
		"support_missile":                nil,
		"expel":                          nil,
		"ActivityBossMediatorTemplate":   nil,
		"ActivityBossAisaikesiScene":     nil,
		"WEIXIANFAMINGPOJINZHONGWEITUO1": nil,
	}
}

func loadGlobalConstants(constants map[string]any, constPath string) error {
	b, err := os.ReadFile(constPath)
	if err != nil {
		return fmt.Errorf("read constants %s: %w", constPath, err)
	}
	ts, err := lex(b)
	if err != nil {
		return fmt.Errorf("parse constants %s: %w", constPath, err)
	}
	p := &parser{tokens: ts, constants: constants, source: []rune(string(b))}
	depth := 0
	for i := 0; i < len(ts); i++ {
		if depth == 0 && ts[i].kind == "ident" && i+1 < len(ts) && ts[i+1].kind == "=" {
			p.i = i + 2
			value, parseErr := p.parseValue()
			if parseErr != nil {
				return fmt.Errorf("parse constant %s in %s: %v", ts[i].text, constPath, parseErr)
			}
			constants[ts[i].text] = value
			i = p.i - 1
			continue
		}
		switch ts[i].kind {
		case "{", "(", "[":
			depth++
		case "}", ")", "]":
			if depth > 0 {
				depth--
			}
		}
	}
	return nil
}

func loadShipTypeConstants(constants map[string]any, constPath string) error {
	b, err := os.ReadFile(constPath)
	if err != nil {
		return fmt.Errorf("read constants %s: %w", constPath, err)
	}
	ts, err := lex(b)
	if err != nil {
		return fmt.Errorf("parse constants %s: %w", constPath, err)
	}
	p := &parser{tokens: ts, constants: constants, source: []rune(string(b))}
	for i := 0; i+3 < len(ts); i++ {
		if ts[i].kind != "ident" || ts[i].text != "slot0" || ts[i+1].kind != "." || ts[i+2].kind != "ident" || ts[i+3].kind != "=" {
			continue
		}
		p.i = i + 4
		value, parseErr := p.parseValue()
		if parseErr != nil {
			return fmt.Errorf("parse constant %s in %s: %v", ts[i+2].text, constPath, parseErr)
		}
		name := ts[i+2].text
		constants["slot0."+name] = value
		constants["ShipType."+name] = value
		i = p.i - 1
	}
	return nil
}

func formatFunctionComment(source []rune, start, end int) string {
	if start < 0 || end <= start || end > len(source) {
		return "-- lua function:"
	}
	lines := strings.Split(string(source[start:end]), "\n")
	if len(lines) > 0 {
		lines = lines[1:]
	}
	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "end" {
		lines = lines[:len(lines)-1]
	}
	body := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			body = append(body, line)
		}
	}
	if len(body) == 0 {
		return "-- lua function:"
	}
	return "-- lua function:\n" + strings.Join(body, "\n")
}
