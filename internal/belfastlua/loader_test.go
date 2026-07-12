package belfastlua

import (
	"os"
	"path/filepath"
	"testing"
)

func testLuaRoot(t *testing.T) string {
	t.Helper()
	if root := os.Getenv("AMAGI_DATA_TEST_LUASCRIPTS_ROOT"); root != "" {
		return root
	}
	fallback := `C:\Users\yutai\amagi-data\_external\AzurLaneLuaScripts`
	if info, err := os.Stat(fallback); err == nil && info.IsDir() {
		return fallback
	}
	return ""
}

func TestLoadReferenceSamples(t *testing.T) {
	root := testLuaRoot(t)
	if root == "" {
		t.Skip("Lua source root unavailable")
	}
	loaded := 0
	if paths, _ := filepath.Glob(filepath.Join(root, "JP", "gamecfg", "buff", "*.lua")); len(paths) > 0 {
		for _, path := range paths {
			if _, err := LoadFile(path); err != nil {
				t.Logf("buff %s: %v", filepath.Base(path), err)
				break
			}
			loaded++
		}
	}
	if paths, _ := filepath.Glob(filepath.Join(root, "JP", "gamecfg", "storyjp", "*.lua")); len(paths) > 0 {
		for _, path := range paths {
			if _, err := LoadFile(path); err != nil {
				t.Logf("storyjp %s: %v", filepath.Base(path), err)
				break
			}
			loaded++
		}
	}
	if v, err := LoadFile(filepath.Join(root, "JP", "sharecfgdata", "ship_data_template.lua")); err == nil {
		loaded++
		if m, ok := v.(map[string]any); ok {
			missing, nonmap := 0, 0
			for _, raw := range m {
				rec, ok := raw.(map[string]any)
				if !ok {
					nonmap++
					continue
				}
				if _, ok := rec["id"]; !ok {
					missing++
				}
			}
			if len(m) != 3789 || nonmap != 0 || missing != 0 || m["11500064"] == nil {
				t.Fatalf("unexpected ship dataset shape: records=%d nonmap=%d missing_id=%d", len(m), nonmap, missing)
			}
		}
	}
	for _, rel := range []string{
		"JP/sharecfgdata/item_data_statistics.lua", "JP/sharecfg/ship_skin_template.lua",
		"JP/sharecfg/enemy_data_statistics.lua", "JP/sharecfg/voice_actor_cn.lua",
		"JP/sharecfg/word_legal_template.lua", "JP/sharecfg/word_template.lua",
		"JP/sharecfgdata/aircraft_template.lua", "JP/sharecfgdata/enemy_data_statistics.lua",
		"JP/sharecfgdata/equip_data_statistics.lua", "JP/sharecfgdata/equip_data_template.lua",
		"JP/sharecfgdata/weapon_property.lua", "JP/gamecfg/buff/buff_1.lua",
		"JP/gamecfg/skill/skill_1.lua",
	} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if _, err := os.Stat(path); err != nil {
			continue
		}
		if _, err := LoadFile(path); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		loaded++
	}
	if loaded == 0 {
		t.Skip("no compatible Lua fixture files available")
	}
}

func TestDebugArg(t *testing.T) {
	root := testLuaRoot(t)
	if root == "" {
		t.Skip("Lua source root unavailable")
	}
	path := filepath.Join(root, "JP", "sharecfg", "strategy_data_template.lua")
	if _, err := os.Stat(path); err != nil {
		t.Skip("strategy_data_template.lua unavailable")
	}
	v, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	m := ToPlain(v).(map[string]any)
	t.Logf("arg=%#v", m["4"].(map[string]any)["arg"])
}
