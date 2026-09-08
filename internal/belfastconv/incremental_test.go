package belfastconv

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestConvertMVPIncrementalMatchesFullBuildAfterUpstreamChanges(t *testing.T) {
	root := t.TempDir()
	writeVersionsFixture(t, root, "1")
	writeLuaFixture(t, filepath.Join(root, "CN", "sharecfg", "changed.lua"), `pg = pg or {}
pg.changed = rawget(pg, "changed") or setmetatable({}, confNEO)
pg.changed.__stream__ = true
pg.changed.all = { 1 }
`)
	writeLuaFixture(t, filepath.Join(root, "CN", "sharecfgdata", "changed.lua"), `pg = pg or {}
pg.changed = { [1] = { id = 1, value = "old" } }
`)
	writeLuaFixture(t, filepath.Join(root, "CN", "sharecfgdata", "removed.lua"), `pg = pg or {}
pg.removed = { [1] = { id = 1, value = "removed" } }
`)
	writeLuaFixture(t, filepath.Join(root, "CN", "sharecfg", "removed.lua"), `pg = pg or {}
pg.removed = { [1] = { id = 1, value = "removed" } }
`)
	writeLuaFixture(t, filepath.Join(root, "CN", "gamecfg", "buff", "100.lua"), `return { [1] = { id = 1, value = "old" } }
`)
	writeLuaFixture(t, filepath.Join(root, "JP", "gamecfg", "storyjp", "100.lua"), `return { [1] = { id = 1, value = "story" } }
`)

	fullBefore := t.TempDir()
	if _, err := ConvertMVP(Options{SourceRoot: root, LuaScriptsRoot: root, OutputRoot: fullBefore}); err != nil {
		t.Fatalf("full before conversion: %v", err)
	}
	incremental := t.TempDir()
	copyTree(t, fullBefore, incremental)

	writeLuaFixture(t, filepath.Join(root, "CN", "sharecfgdata", "changed.lua"), `pg = pg or {}
pg.changed = { [1] = { id = 1, value = "new" } }
`)
	writeLuaFixture(t, filepath.Join(root, "CN", "sharecfg", "new.lua"), `pg = pg or {}
pg.new = { [2] = { id = 2, value = "added" } }
`)
	writeLuaFixture(t, filepath.Join(root, "CN", "gamecfg", "buff", "200.lua"), `return { [2] = { id = 2, value = "added" } }
`)
	writeLuaFixture(t, filepath.Join(root, "JP", "gamecfg", "storyjp", "200.lua"), `return { [2] = { id = 2, value = "added" } }
`)
	if err := os.Remove(filepath.Join(root, "CN", "sharecfgdata", "removed.lua")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "CN", "sharecfg", "removed.lua")); err != nil {
		t.Fatal(err)
	}
	writeVersionsFixture(t, root, "2")

	fullAfter := t.TempDir()
	if _, err := ConvertMVP(Options{SourceRoot: root, LuaScriptsRoot: root, OutputRoot: fullAfter}); err != nil {
		t.Fatalf("full after conversion: %v", err)
	}
	plan := IncrementalPlan{
		Mode:        "incremental",
		PreviousSHA: "before",
		LatestSHA:   "after",
		SourcePaths: []string{
			"CN/sharecfg/changed.lua", "CN/sharecfgdata/changed.lua",
			"CN/sharecfg/new.lua", "CN/sharecfgdata/new.lua",
			"CN/sharecfg/removed.lua", "CN/sharecfgdata/removed.lua",
		},
		RequiredSourcePaths: []string{"CN/sharecfgdata/changed.lua", "CN/sharecfg/new.lua"},
		GameCfg:             []string{"CN/GameCfg/buff.json", "JP/GameCfg/story.json"},
		Versions:            true,
		DeleteOutputs:       []string{"CN/ShareCfg/removed.json", "CN/sharecfgdata/removed.json"},
	}
	if _, err := ConvertMVPIncremental(Options{SourceRoot: root, LuaScriptsRoot: root, OutputRoot: incremental}, plan); err != nil {
		t.Fatalf("incremental conversion: %v", err)
	}

	got := treeHashes(t, incremental)
	want := treeHashes(t, fullAfter)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("incremental output differs from full output:\n got: %v\nwant: %v", got, want)
	}
	if _, err := os.Stat(filepath.Join(incremental, "CN", "ShareCfg", "removed.json")); !os.IsNotExist(err) {
		t.Fatalf("deleted ShareCfg output still exists: %v", err)
	}
}

func writeVersionsFixture(t *testing.T, root, suffix string) {
	t.Helper()
	for _, region := range regionNames {
		writeLuaFixture(t, filepath.Join(root, "versions", region+".txt"), suffix+"-"+region)
	}
}

func writeLuaFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func copyTree(t *testing.T, source, destination string) {
	t.Helper()
	err := filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func treeHashes(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || entry.Name() == "belfast-json-mvp-report.json" {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		hash := sha256.New()
		_, copyErr := io.Copy(hash, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		result[filepath.ToSlash(rel)] = fmt.Sprintf("%x", hash.Sum(nil))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}
