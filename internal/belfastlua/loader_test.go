package belfastlua

import (
	"path/filepath"
	"testing"
)

func TestLoadReferenceSamples(t *testing.T) {
	if paths, _ := filepath.Glob(`C:\Users\yutai\amagi-data\_external\AzurLaneLuaScripts\JP\gamecfg\buff\*.lua`); len(paths) > 0 {
		for _, path := range paths {
			if _, err := LoadFile(path); err != nil {
				t.Logf("buff %s: %v", filepath.Base(path), err)
				break
			}
		}
	}
	if paths, _ := filepath.Glob(`C:\Users\yutai\amagi-data\_external\AzurLaneLuaScripts\JP\gamecfg\storyjp\*.lua`); len(paths) > 0 {
		for _, path := range paths {
			if _, err := LoadFile(path); err != nil {
				t.Logf("storyjp %s: %v", filepath.Base(path), err)
				break
			}
		}
	}
	if v, err := LoadFile(`C:\Users\yutai\amagi-data\_external\AzurLaneLuaScripts\JP\sharecfgdata\ship_data_template.lua`); err == nil {
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
	for _, path := range []string{
		`C:\Users\yutai\amagi-data\_external\AzurLaneLuaScripts\JP\sharecfgdata\item_data_statistics.lua`,
		`C:\Users\yutai\amagi-data\_external\AzurLaneLuaScripts\JP\sharecfg\ship_skin_template.lua`,
		`C:\Users\yutai\amagi-data\_external\AzurLaneLuaScripts\JP\sharecfg\enemy_data_statistics.lua`,
		`C:\Users\yutai\amagi-data\_external\AzurLaneLuaScripts\JP\sharecfg\voice_actor_cn.lua`,
		`C:\Users\yutai\amagi-data\_external\AzurLaneLuaScripts\JP\sharecfg\word_legal_template.lua`,
		`C:\Users\yutai\amagi-data\_external\AzurLaneLuaScripts\JP\sharecfg\word_template.lua`,
		`C:\Users\yutai\amagi-data\_external\AzurLaneLuaScripts\JP\sharecfgdata\aircraft_template.lua`,
		`C:\Users\yutai\amagi-data\_external\AzurLaneLuaScripts\JP\sharecfgdata\enemy_data_statistics.lua`,
		`C:\Users\yutai\amagi-data\_external\AzurLaneLuaScripts\JP\sharecfgdata\equip_data_statistics.lua`,
		`C:\Users\yutai\amagi-data\_external\AzurLaneLuaScripts\JP\sharecfgdata\equip_data_template.lua`,
		`C:\Users\yutai\amagi-data\_external\AzurLaneLuaScripts\JP\sharecfgdata\weapon_property.lua`,
		`C:\Users\yutai\amagi-data\_external\AzurLaneLuaScripts\JP\gamecfg\buff\buff_1.lua`,
		`C:\Users\yutai\amagi-data\_external\AzurLaneLuaScripts\JP\gamecfg\skill\skill_1.lua`,
	} {
		if _, err := LoadFile(path); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
}

func TestDebugArg(t *testing.T) {
	v, err := LoadFile(`C:\Users\yutai\amagi-data\_external\AzurLaneLuaScripts\JP\sharecfg\strategy_data_template.lua`)
	if err != nil { t.Fatal(err) }
	m := ToPlain(v).(map[string]any)
	t.Logf("arg=%#v", m["4"].(map[string]any)["arg"])
}
