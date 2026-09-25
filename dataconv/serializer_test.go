package dataconv

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"

	"github.com/Fokiiiiiii/amagi-data/azurlanelua"
)

// legacyMarshalGeneratedJSON is the buffer-per-value serializer that
// marshalGeneratedJSON replaced. Published data must stay byte-identical, so it is
// kept as the reference the append-based implementation is checked against.
func legacyMarshalGeneratedJSON(v any) ([]byte, error) {
	switch value := v.(type) {
	case azurlanelua.OrderedObject:
		var b bytes.Buffer
		b.WriteByte('{')
		for i, key := range value.Keys {
			if i > 0 {
				b.WriteByte(',')
			}
			kb, _ := json.Marshal(key)
			b.Write(kb)
			b.WriteByte(':')
			child, err := legacyMarshalGeneratedJSON(value.Values[key])
			if err != nil {
				return nil, err
			}
			b.Write(child)
		}
		b.WriteByte('}')
		return b.Bytes(), nil
	case map[string]any:
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		numeric := true
		nums := make(map[string]int, len(keys))
		for _, key := range keys {
			// Most keys are field names, and strconv.Atoi allocates a *NumError for
			// every one of them. Reject the obvious non-numbers by their first byte
			// first: anything Atoi accepts starts with a sign or a digit.
			if !mayBeNumericKey(key) {
				numeric = false
				break
			}
			n, err := strconv.Atoi(key)
			if err != nil {
				numeric = false
				break
			}
			nums[key] = n
		}
		generatedIDAtEnd := false
		if _, ok := value["id"].(generatedID); ok {
			generatedIDAtEnd = true
		}
		if generatedIDAtEnd {
			sort.Slice(keys, func(i, j int) bool {
				if keys[i] == "id" {
					return false
				}
				if keys[j] == "id" {
					return true
				}
				return keys[i] < keys[j]
			})
		} else if numeric {
			sort.Slice(keys, func(i, j int) bool {
				if nums[keys[i]] != nums[keys[j]] {
					return nums[keys[i]] < nums[keys[j]]
				}
				return keys[i] < keys[j]
			})
		} else {
			// Classify once instead of inside the comparator: sort.Slice runs
			// O(n log n) comparisons and each failed Atoi allocates a *NumError.
			parsed := make(map[string]int, len(keys))
			for _, key := range keys {
				if !mayBeNumericKey(key) {
					continue
				}
				if n, err := strconv.Atoi(key); err == nil {
					parsed[key] = n
				}
			}
			sort.Slice(keys, func(i, j int) bool {
				numI, okI := parsed[keys[i]]
				numJ, okJ := parsed[keys[j]]
				if okI && okJ {
					return numI < numJ
				}
				if okI != okJ {
					return okI
				}
				return keys[i] < keys[j]
			})
		}
		var b bytes.Buffer
		b.WriteByte('{')
		for i, key := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			kb, _ := json.Marshal(key)
			b.Write(kb)
			b.WriteByte(':')
			child, err := legacyMarshalGeneratedJSON(value[key])
			if err != nil {
				return nil, err
			}
			b.Write(child)
		}
		b.WriteByte('}')
		return b.Bytes(), nil
	case []any:
		var b bytes.Buffer
		b.WriteByte('[')
		for i, childValue := range value {
			if i > 0 {
				b.WriteByte(',')
			}
			child, err := legacyMarshalGeneratedJSON(childValue)
			if err != nil {
				return nil, err
			}
			b.Write(child)
		}
		b.WriteByte(']')
		return b.Bytes(), nil
	case json.Number:
		if value == "-0" || value == "-0.0" {
			return []byte("0"), nil
		}
		parsed, err := strconv.ParseFloat(string(value), 64)
		if err == nil && math.Trunc(parsed) == parsed {
			return []byte(strconv.FormatFloat(parsed, 'f', -1, 64)), nil
		}
		return []byte(value), nil
	case generatedID:
		return []byte(strconv.Itoa(int(value))), nil
	default:
		var b bytes.Buffer
		enc := json.NewEncoder(&b)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(v); err != nil {
			return nil, err
		}
		return bytes.TrimSuffix(b.Bytes(), []byte{'\n'}), nil
	}
}

func TestMarshalGeneratedJSONMatchesLegacySerializer(t *testing.T) {
	values := []any{
		nil, true, false, "", "plain", "quote\" back\\slash", "<html> & 'amp'", "tab\tnew\nline",
		"日本語テキスト", " sep ", "del\x7f", "bad\xffutf8", int64(-3), 1.5, float64(1e21),
		json.Number("-0"), json.Number("-0.0"), json.Number("2.0"), json.Number("1.25"), json.Number("1e3"),
		generatedID(7), []any{}, map[string]any{},
		map[string]any{"10": 1, "9": 2, "-1": 3, "+2": 4, "007": 5},
		map[string]any{"b": 1, "a": 2, "10": 3, "2": 4, "<k>": 5, "é": 6},
		map[string]any{"name": "x", "id": generatedID(3), "alpha": []any{json.Number("1.0"), nil}},
		azurlanelua.OrderedObject{Keys: []string{"z", "a&b", "1"}, Values: map[string]any{
			"z": map[string]any{"2": "two", "1": "one"}, "a&b": []any{"x", map[string]any{}}, "1": nil,
		}},
	}
	for i, value := range values {
		want, wantErr := legacyMarshalGeneratedJSON(value)
		got, gotErr := marshalGeneratedJSON(value)
		if (wantErr != nil) != (gotErr != nil) || !bytes.Equal(got, want) {
			t.Fatalf("value %d (%#v):\n got %q (%v)\nwant %q (%v)", i, value, got, gotErr, want, wantErr)
		}
	}
	// Every committed ShareCfg table of one region, as read back for an incremental refresh.
	paths, _ := filepath.Glob(filepath.Join("..", "global", "*.json"))
	more, _ := filepath.Glob(filepath.Join("..", "JP", "ShareCfg", "a*.json"))
	for _, path := range append(paths, more...) {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := decodeOrderedJSON(data)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		want, _ := legacyMarshalGeneratedJSON(decoded)
		got, _ := marshalGeneratedJSON(decoded)
		if !bytes.Equal(got, want) {
			t.Fatalf("%s: serializers differ", path)
		}
	}
}
