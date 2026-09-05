package store

import (
	"errors"
	"os"
	"testing"

	"github.com/turio/AmnesiaGraph/internal/graph"
)

func TestSaveLoadAndNotInitialized(t *testing.T) {
	root := t.TempDir()
	if _, err := Load(root); !errors.Is(err, ErrNotInitialized) {
		t.Fatalf("got %v, want ErrNotInitialized", err)
	}
	current := graph.Graph{Version: graph.Version, Tasks: []graph.Task{{
		ID: "T001", Order: 1, Title: "Task", Source: "plan.md#task", DependsOn: []string{}, Verify: []string{}, Status: graph.Pending,
	}}}
	if err := os.WriteFile(root+string(os.PathSeparator)+"plan.md", []byte("plan"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Save(root, current); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Tasks[0].ID != "T001" || loaded.Tasks[0].Status != graph.Pending {
		t.Fatalf("unexpected graph: %+v", loaded)
	}
	if _, err := os.Stat(GraphPath(root)); err != nil {
		t.Fatal(err)
	}
}

func TestSaveReplacesExistingGraph(t *testing.T) {
	root := t.TempDir()
	first := graph.Graph{Version: graph.Version, Tasks: []graph.Task{{ID: "A", Order: 1, Title: "First", Source: "a.md", Status: graph.Pending}}}
	second := graph.Graph{Version: graph.Version, Tasks: []graph.Task{{ID: "B", Order: 1, Title: "Second", Source: "b.md", Status: graph.Pending}}}
	if err := Save(root, first); err != nil {
		t.Fatal(err)
	}
	if err := Save(root, second); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Tasks[0].ID != "B" {
		t.Fatalf("got %q, want replacement graph", loaded.Tasks[0].ID)
	}
}
