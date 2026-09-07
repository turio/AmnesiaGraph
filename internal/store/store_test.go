package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/turio/AmnesiaGraph/internal/graph"
)

func TestNamedSaveLoadIsolation(t *testing.T) {
	root := t.TempDir()
	first := graph.Graph{Version: graph.Version, Tasks: []graph.Task{{ID: "A", Order: 1, Title: "First", Source: "a.md", Status: graph.Pending}}}
	second := graph.Graph{Version: graph.Version, Tasks: []graph.Task{{ID: "B", Order: 1, Title: "Second", Source: "b.md", Status: graph.Pending}}}
	if err := Save(root, "plan-a", first); err != nil {
		t.Fatal(err)
	}
	if err := Save(root, "plan-b", second); err != nil {
		t.Fatal(err)
	}
	loadedA, err := Load(root, "plan-a")
	if err != nil {
		t.Fatal(err)
	}
	loadedB, err := Load(root, "plan-b")
	if err != nil {
		t.Fatal(err)
	}
	if loadedA.Tasks[0].ID != "A" || loadedB.Tasks[0].ID != "B" {
		t.Fatalf("isolation broken: %+v %+v", loadedA, loadedB)
	}
	// Overwriting plan-a must not touch plan-b.
	third := graph.Graph{Version: graph.Version, Tasks: []graph.Task{{ID: "A2", Order: 1, Title: "First v2", Source: "a.md", Status: graph.Pending}}}
	if err := Save(root, "plan-a", third); err != nil {
		t.Fatal(err)
	}
	loadedB, err = Load(root, "plan-b")
	if err != nil {
		t.Fatal(err)
	}
	if loadedB.Tasks[0].ID != "B" {
		t.Fatalf("named save leaked across graphs: %+v", loadedB)
	}
}

func TestGraphNameValidationRejectsUnsafe(t *testing.T) {
	for _, bad := range []string{"", "Plan-A", "has space", "a/b", "a\\b", "../escape", ".", "..", "-lead", "_lead", ".lead", "UPPER", "a/b/c"} {
		if ValidateGraphName(bad) == nil {
			t.Errorf("invalid name accepted: %q", bad)
		}
	}
	for _, good := range []string{"a", "0", "auth", "renderer-v2", "spec-2026.09", "a_b-c.d"} {
		if ValidateGraphName(good) != nil {
			t.Errorf("valid name rejected: %q", good)
		}
	}
	// Traversal attempt must not escape graphs dir even if constructed.
	root := t.TempDir()
	if err := Save(root, "../escape", graph.Graph{}); err == nil {
		t.Fatal("traversal save accepted")
	}
	if _, err := Load(root, "../escape"); err == nil {
		t.Fatal("traversal load accepted")
	}
}

func TestCurrentPointerAtomicBehavior(t *testing.T) {
	root := t.TempDir()
	if _, err := Current(root); err == nil {
		t.Fatal("expected no current")
	}
	if err := SetCurrent(root, "plan-a"); err != nil {
		t.Fatal(err)
	}
	name, err := Current(root)
	if err != nil || name != "plan-a" {
		t.Fatalf("got %q %v", name, err)
	}
	if err := SetCurrent(root, "plan-b"); err != nil {
		t.Fatal(err)
	}
	name, err = Current(root)
	if err != nil || name != "plan-b" {
		t.Fatalf("got %q %v", name, err)
	}
	data, err := os.ReadFile(CurrentPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != "plan-b" {
		t.Fatalf("current file body: %q", data)
	}
	if SetCurrent(root, "BAD NAME") == nil {
		t.Fatal("invalid current accepted")
	}
}

func TestListExposesNamesAndModTimes(t *testing.T) {
	root := t.TempDir()
	infos, err := List(root)
	if err != nil || len(infos) != 0 {
		t.Fatalf("empty list: %v %v", infos, err)
	}
	g := graph.Graph{Version: graph.Version, Tasks: []graph.Task{{ID: "T", Order: 1, Title: "T", Source: "a.md", Status: graph.Pending}}}
	if err := Save(root, "older", g); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	if err := Save(root, "newer", g); err != nil {
		t.Fatal(err)
	}
	// Force deterministic mtimes.
	oldTime := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
	newTime := time.Now().Add(-1 * time.Hour).Truncate(time.Second)
	if err := os.Chtimes(GraphPath(root, "older"), oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(GraphPath(root, "newer"), newTime, newTime); err != nil {
		t.Fatal(err)
	}
	infos, err = List(root)
	if err != nil || len(infos) != 2 {
		t.Fatalf("list: %v %v", infos, err)
	}
	byName := map[string]GraphInfo{}
	for _, info := range infos {
		byName[info.Name] = info
	}
	if _, ok := byName["older"]; !ok {
		t.Fatalf("missing older: %v", infos)
	}
	if !byName["newer"].ModTime.After(byName["older"].ModTime) {
		t.Fatalf("mtime order wrong: %+v", byName)
	}
}

func TestLegacyMigrationPreservesState(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, StateDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacyBody := `{"version":1,"tasks":[{"id":"MG00-T01","order":1,"title":"Bootstrap","source":"docs/PLAN.md#x","depends_on":[],"verify":[],"status":"active","blocker":null}]}` + "\n"
	if err := os.WriteFile(LegacyGraphPath(root), []byte(legacyBody), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := MigrateLegacyIfNeeded(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(LegacyGraphPath(root)); !os.IsNotExist(err) {
		t.Fatalf("legacy file should be moved, stat: %v", err)
	}
	data, err := os.ReadFile(GraphPath(root, DefaultGraphName))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != legacyBody {
		t.Fatalf("migration changed bytes:\n got %q\nwant %q", data, legacyBody)
	}
	current, err := Current(root)
	if err != nil || current != DefaultGraphName {
		t.Fatalf("current after migration: %q %v", current, err)
	}
	loaded, err := Load(root, DefaultGraphName)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Tasks[0].ID != "MG00-T01" || loaded.Tasks[0].Status != graph.Active {
		t.Fatalf("state not preserved: %+v", loaded)
	}
	// Second call is a no-op.
	if err := MigrateLegacyIfNeeded(root); err != nil {
		t.Fatal(err)
	}
}

func TestMigrationConflictLeavesFilesUnchanged(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, StateDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacyBody := []byte(`{"version":1,"tasks":[]}` + "\n")
	if err := os.WriteFile(LegacyGraphPath(root), legacyBody, 0o644); err != nil {
		t.Fatal(err)
	}
	g := graph.Graph{Version: graph.Version, Tasks: []graph.Task{{ID: "T", Order: 1, Title: "T", Source: "a.md", Status: graph.Pending}}}
	if err := Save(root, "plan-a", g); err != nil {
		t.Fatal(err)
	}
	if err := SetCurrent(root, "plan-a"); err != nil {
		t.Fatal(err)
	}
	namedBefore, err := os.ReadFile(GraphPath(root, "plan-a"))
	if err != nil {
		t.Fatal(err)
	}
	currentBefore, err := os.ReadFile(CurrentPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if err := MigrateLegacyIfNeeded(root); err == nil || !strings.Contains(err.Error(), "MIGRATION_CONFLICT") {
		t.Fatalf("expected MIGRATION_CONFLICT, got %v", err)
	}
	legacyAfter, err := os.ReadFile(LegacyGraphPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if string(legacyAfter) != string(legacyBody) {
		t.Fatal("legacy mutated on conflict")
	}
	namedAfter, _ := os.ReadFile(GraphPath(root, "plan-a"))
	if string(namedAfter) != string(namedBefore) {
		t.Fatal("named graph mutated on conflict")
	}
	currentAfter, _ := os.ReadFile(CurrentPath(root))
	if string(currentAfter) != string(currentBefore) {
		t.Fatal("current mutated on conflict")
	}
}

func TestMissingStateSignals(t *testing.T) {
	root := t.TempDir()
	infos, err := List(root)
	if err != nil || len(infos) != 0 {
		t.Fatalf("expected empty list, got %v %v", infos, err)
	}
	if _, err := Current(root); err == nil {
		t.Fatal("expected no current")
	}
	if _, err := Load(root, "missing"); err == nil || !strings.Contains(err.Error(), "GRAPH_NOT_FOUND") {
		t.Fatalf("expected GRAPH_NOT_FOUND, got %v", err)
	}
}
