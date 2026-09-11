package azurlanelua

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeLoaderFixture(t *testing.T, path, content string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadFileRejectsUnsupportedExpressionInsteadOfDroppingRecord(t *testing.T) {
	path := writeLoaderFixture(t, filepath.Join(t.TempDir(), "JP", "sharecfg", "sample.lua"), `return {
		{ id = 1, value = "kept" },
		{ id = 2, value = 1 + 2 },
	}
`)

	_, err := LoadFile(path)
	if err == nil || !strings.Contains(err.Error(), "unsupported Lua expression operator") {
		t.Fatalf("expected unsupported expression error, got %v", err)
	}
}

func TestLoadFileRejectsUnsupportedFunctionCall(t *testing.T) {
	path := writeLoaderFixture(t, filepath.Join(t.TempDir(), "JP", "sharecfg", "sample.lua"), `return {
		{ id = 1, value = max(1, 2) },
	}
`)

	_, err := LoadFile(path)
	if err == nil || !strings.Contains(err.Error(), "unsupported Lua function call") {
		t.Fatalf("expected unsupported function error, got %v", err)
	}
}

func TestLoadFileRejectsUnresolvedIdentifier(t *testing.T) {
	path := writeLoaderFixture(t, filepath.Join(t.TempDir(), "JP", "sharecfg", "sample.lua"), `return {
		{ id = 1, value = UNKNOWN_CONSTANT },
	}
`)

	_, err := LoadFile(path)
	if err == nil || !strings.Contains(err.Error(), "unresolved Lua identifier") {
		t.Fatalf("expected unresolved identifier error, got %v", err)
	}
}

func TestLoadFileTreatsUnknownArrayIdentifierAsNil(t *testing.T) {
	path := writeLoaderFixture(t, filepath.Join(t.TempDir(), "JP", "gamecfg", "storyjp", "niukasier6.lua"), `return {
		{
			blackBgtrue,
			actor = 202190,
			keep = "record",
		},
	}
`)

	value, err := LoadFile(path)
	if err != nil {
		t.Fatalf("unknown array identifier rejected: %v", err)
	}
	rows, ok := ToPlain(value).([]any)
	if !ok || len(rows) != 1 {
		t.Fatalf("unexpected rows: %#v", value)
	}
	record, ok := rows[0].(map[string]any)
	if !ok || record["actor"] != json.Number("202190") || record["keep"] != "record" {
		t.Fatalf("unexpected record: %#v", rows[0])
	}
	if _, ok := record["1"]; ok {
		t.Fatalf("unknown array identifier should not be emitted: %#v", record)
	}
}

func TestLoadFileAllowsKnownLegacyNilIdentifiers(t *testing.T) {
	path := writeLoaderFixture(t, filepath.Join(t.TempDir(), "JP", "sharecfg", "barrage_template.lua"), `return {
		{
			stopbgm = trur,
			random_angle = ture,
			page = BuildShipScene.PAGE_PRAY,
			spine = { [4] = walk },
			keep = true,
		},
	}
`)

	value, err := LoadFile(path)
	if err != nil {
		t.Fatalf("known legacy nil identifiers rejected: %v", err)
	}
	rows, ok := ToPlain(value).([]any)
	if !ok || len(rows) != 1 {
		t.Fatalf("unexpected rows: %#v", value)
	}
	record, ok := rows[0].(map[string]any)
	if !ok || record["keep"] != true {
		t.Fatalf("unexpected record: %#v", rows[0])
	}
	for _, key := range []string{"stopbgm", "random_angle", "page"} {
		if _, ok := record[key]; ok {
			t.Fatalf("legacy nil field %q should be omitted: %#v", key, record)
		}
	}
	spine, ok := record["spine"].(map[string]any)
	if !ok || len(spine) != 0 {
		t.Fatalf("legacy nil nested value should be omitted: %#v", record["spine"])
	}
}

func TestLoadFileAcceptsLegacyExtraLongStringTerminator(t *testing.T) {
	path := writeLoaderFixture(t, filepath.Join(t.TempDir(), "EN", "sharecfg", "skill_world_display.lua"), `return {
		{
			desc = [[legacy text]]],
			keep = true,
		},
	}
`)

	value, err := LoadFile(path)
	if err != nil {
		t.Fatalf("legacy long string terminator rejected: %v", err)
	}
	rows, ok := ToPlain(value).([]any)
	if !ok || len(rows) != 1 {
		t.Fatalf("unexpected rows: %#v", value)
	}
	record, ok := rows[0].(map[string]any)
	if !ok || record["desc"] != "legacy text" || record["keep"] != true {
		t.Fatalf("unexpected record: %#v", rows[0])
	}
}

func TestLoadFileReturnsErrorForTruncatedTable(t *testing.T) {
	path := writeLoaderFixture(t, filepath.Join(t.TempDir(), "JP", "sharecfg", "sample.lua"), `return {
		{ id = 1
`)

	_, err := LoadFile(path)
	if err == nil || !strings.Contains(err.Error(), "missing }") {
		t.Fatalf("expected truncated table error, got %v", err)
	}
}

func TestLoadFileReturnsErrorForTruncatedFunction(t *testing.T) {
	path := writeLoaderFixture(t, filepath.Join(t.TempDir(), "JP", "sharecfg", "sample.lua"), `return {
		callback = function()
`)

	_, err := LoadFile(path)
	if err == nil || !strings.Contains(err.Error(), "unterminated Lua function expression") {
		t.Fatalf("expected truncated function error, got %v", err)
	}
}

func TestLoadFileSkipsCompleteNestedFunctionBlocks(t *testing.T) {
	path := writeLoaderFixture(t, filepath.Join(t.TempDir(), "JP", "sharecfg", "sample.lua"), `return {
		callback = function()
			if true then
				for i = 1, 1 do
				end
			end
		end,
		value = "kept",
	}
`)

	value, err := LoadFile(path)
	if err != nil {
		t.Fatalf("nested function rejected: %v", err)
	}
	plain, ok := ToPlain(value).(map[string]any)
	if !ok || plain["value"] != "kept" {
		t.Fatalf("unexpected nested function result: %#v", value)
	}
}

func TestLoadFileAllowsExplicitBootstrapCalls(t *testing.T) {
	path := writeLoaderFixture(t, filepath.Join(t.TempDir(), "JP", "sharecfg", "sample.lua"), `pg = pg or {}
pg.sample = rawget(pg, "sample") or setmetatable({ __name = "sample" }, confNEO)
pg.sample.__stream__ = true
`)

	value, err := LoadFile(path)
	if err != nil {
		t.Fatalf("bootstrap calls rejected: %v", err)
	}
	plain, ok := ToPlain(value).(map[string]any)
	if !ok || plain["__stream__"] != true {
		t.Fatalf("unexpected bootstrap result: %#v", value)
	}
}

func TestLoadFileUsesExplicitConstantsRootForShallowSourcePath(t *testing.T) {
	root := t.TempDir()
	constantsRoot := filepath.Join(root, "CN")
	writeLoaderFixture(t, filepath.Join(constantsRoot, "const.lua"), "SYSTEM_DUEL = 3\n")
	writeLoaderFixture(t, filepath.Join(constantsRoot, "model", "const", "shiptype.lua"), `slot0 = class("ShipType")
slot0.QuZhu = 7
`)
	source := writeLoaderFixture(t, filepath.Join(root, "JP", "sharecfg", "sample.lua"), `pg = pg or {}
pg.sample = { [1] = { ship_type = slot0.QuZhu } }
`)

	value, err := LoadFileWithConstantsRoot(source, constantsRoot)
	if err != nil {
		t.Fatalf("explicit constants root rejected: %v", err)
	}
	plain, ok := ToPlain(value).(map[string]any)
	if !ok {
		t.Fatalf("unexpected result: %#v", value)
	}
	rows, ok := plain["1"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected sample row: %#v", plain["1"])
	}
	if got, ok := rows["ship_type"].(json.Number); !ok || got != json.Number("7") {
		t.Fatalf("ship_type = %#v, want 7", rows["ship_type"])
	}
}

func TestLoadFileResolvesGlobalAndRuntimeConstants(t *testing.T) {
	root := t.TempDir()
	constantsRoot := filepath.Join(root, "CN")
	writeLoaderFixture(t, filepath.Join(constantsRoot, "const.lua"), "SYSTEM_DUEL = 3\n")
	writeLoaderFixture(t, filepath.Join(constantsRoot, "model", "const", "shiptype.lua"), "slot0 = class(\"ShipType\")\n")
	source := writeLoaderFixture(t, filepath.Join(root, "JP", "sharecfg", "sample.lua"), `return {
		{
			system = SYSTEM_DUEL,
			flag = TRUE,
			limit = { ship_unlock, 20 },
		},
	}
`)

	value, err := LoadFileWithConstantsRoot(source, constantsRoot)
	if err != nil {
		t.Fatalf("runtime constants rejected: %v", err)
	}
	rows, ok := ToPlain(value).([]any)
	if !ok || len(rows) != 1 {
		t.Fatalf("unexpected rows: %#v", value)
	}
	record, ok := rows[0].(map[string]any)
	if !ok || record["system"] != json.Number("3") || record["flag"] != true {
		t.Fatalf("unexpected constants: %#v", record)
	}
	limit, ok := record["limit"].(map[string]any)
	if !ok || limit["2"] != json.Number("20") {
		t.Fatalf("unexpected sparse runtime value: %#v", record["limit"])
	}
}

func TestLoadFileWithConstantsRootRejectsMissingDependency(t *testing.T) {
	root := t.TempDir()
	writeLoaderFixture(t, filepath.Join(root, "CN", "const.lua"), "SYSTEM_DUEL = 3\n")
	source := writeLoaderFixture(t, filepath.Join(root, "JP", "sharecfg", "sample.lua"), `return { { id = 1 } }
`)

	_, err := LoadFileWithConstantsRoot(source, filepath.Join(root, "CN"))
	if err == nil || !strings.Contains(err.Error(), "shiptype.lua") {
		t.Fatalf("expected missing constants dependency error, got %v", err)
	}
}

func TestLoadReferenceFunctionWithExplicitConstantsRoot(t *testing.T) {
	root := testLuaRoot(t)
	if root == "" {
		t.Skip("Lua source root unavailable")
	}
	path := filepath.Join(root, "CN", "gamecfg", "buff", "buff_1010721.lua")
	if _, err := os.Stat(path); err != nil {
		t.Skip("buff_1010721.lua unavailable")
	}
	if _, err := LoadFileWithConstantsRoot(path, filepath.Join(root, "CN")); err != nil {
		t.Fatalf("reference function with explicit constants root: %v", err)
	}
}

func TestLoadReferenceConstantsWithExplicitConstantsRoot(t *testing.T) {
	root := testLuaRoot(t)
	if root == "" {
		t.Skip("Lua source root unavailable")
	}
	loaded := 0
	for _, rel := range []string{
		"CN/gamecfg/buff/buff_1012430.lua",
		"JP/gamecfg/storyjp/fuxingdezanmeishi25.lua",
		"JP/sharecfgdata/item_data_statistics.lua",
	} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if _, err := os.Stat(path); err != nil {
			continue
		}
		if _, err := LoadFileWithConstantsRoot(path, filepath.Join(root, "CN")); err != nil {
			t.Fatalf("%s with explicit constants root: %v", rel, err)
		}
		loaded++
	}
	if loaded == 0 {
		t.Skip("reference constant fixtures unavailable")
	}
}
