package graph

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateStructuralRules(t *testing.T) {
	root := t.TempDir()
	writeSource(t, root, "plan.md")
	writeSource(t, root, "other.md")

	tests := []struct {
		name    string
		mutate  func(*Graph)
		wantErr string
	}{
		{name: "unsupported version", mutate: func(g *Graph) { g.Version = 2 }, wantErr: "unsupported version"},
		{name: "empty id", mutate: func(g *Graph) { g.Tasks[0].ID = " " }, wantErr: "task id is empty"},
		{name: "duplicate id", mutate: func(g *Graph) { g.Tasks[1].ID = g.Tasks[0].ID }, wantErr: "duplicate task id"},
		{name: "duplicate order", mutate: func(g *Graph) { g.Tasks[1].Order = g.Tasks[0].Order }, wantErr: "duplicate order"},
		{name: "empty title", mutate: func(g *Graph) { g.Tasks[0].Title = "" }, wantErr: "empty title"},
		{name: "empty source", mutate: func(g *Graph) { g.Tasks[0].Source = "#section" }, wantErr: "source path is empty"},
		{name: "unknown dependency", mutate: func(g *Graph) { g.Tasks[1].DependsOn = []string{"missing"} }, wantErr: "unknown task"},
		{name: "self dependency", mutate: func(g *Graph) { g.Tasks[0].DependsOn = []string{g.Tasks[0].ID} }, wantErr: "depends on itself"},
		{name: "invalid status", mutate: func(g *Graph) { g.Tasks[0].Status = Status("other") }, wantErr: "invalid status"},
		{name: "blocked without reason", mutate: func(g *Graph) { g.Tasks[0].Status = Blocked }, wantErr: "no blocker reason"},
		{name: "reason on non blocked", mutate: func(g *Graph) { reason := "bad"; g.Tasks[0].Blocker = &reason }, wantErr: "has a blocker reason"},
		{name: "empty verify", mutate: func(g *Graph) { g.Tasks[0].Verify = []string{"  "} }, wantErr: "empty verify command"},
		{name: "missing source file", mutate: func(g *Graph) { g.Tasks[0].Source = "missing.md#section" }, wantErr: "does not exist"},
		{name: "escaping source", mutate: func(g *Graph) { g.Tasks[0].Source = "../outside.md#section" }, wantErr: "escapes repository"},
		{name: "absolute source", mutate: func(g *Graph) { g.Tasks[0].Source = filepath.Join(root, "plan.md") }, wantErr: "must be relative"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			current := baseGraph()
			test.mutate(&current)
			err := Validate(current, root)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("got %v, want error containing %q", err, test.wantErr)
			}
		})
	}
}

func TestValidateSupportsMultipleSourceFilesAndBranchedDAG(t *testing.T) {
	root := t.TempDir()
	writeSource(t, root, "plan.md")
	writeSource(t, root, "specs/other.md")
	current := Graph{
		Version: Version,
		Tasks: []Task{
			{ID: "T001", Order: 1, Title: "Root", Source: "plan.md#root", DependsOn: []string{}, Verify: []string{}, Status: Pending},
			{ID: "T002", Order: 2, Title: "Branch A", Source: "specs/other.md#a", DependsOn: []string{"T001"}, Verify: []string{}, Status: Pending},
			{ID: "T003", Order: 3, Title: "Branch B", Source: "plan.md#b", DependsOn: []string{"T001"}, Verify: []string{}, Status: Pending},
			{ID: "T004", Order: 4, Title: "Join", Source: "plan.md#join", DependsOn: []string{"T002", "T003"}, Verify: []string{}, Status: Pending},
		},
	}
	if err := Validate(current, root); err != nil {
		t.Fatal(err)
	}
}

func TestCycleErrorIncludesActionablePath(t *testing.T) {
	root := t.TempDir()
	writeSource(t, root, "plan.md")
	current := Graph{
		Version: Version,
		Tasks: []Task{
			{ID: "T001", Order: 1, Title: "One", Source: "plan.md#one", DependsOn: []string{"T002"}, Verify: []string{}, Status: Pending},
			{ID: "T002", Order: 2, Title: "Two", Source: "plan.md#two", DependsOn: []string{"T003"}, Verify: []string{}, Status: Pending},
			{ID: "T003", Order: 3, Title: "Three", Source: "plan.md#three", DependsOn: []string{"T001"}, Verify: []string{}, Status: Pending},
		},
	}
	err := Validate(current, root)
	if err == nil || !strings.Contains(err.Error(), "cycle: T001 -> T002 -> T003 -> T001") {
		t.Fatalf("got %v, want actionable cycle", err)
	}
}

func TestReadyTasksAreDependencyBasedAndOrdered(t *testing.T) {
	current := Graph{Version: Version, Tasks: []Task{
		{ID: "T006", Order: 6, Title: "Independent", Status: Pending},
		{ID: "T003", Order: 3, Title: "Branch B", DependsOn: []string{"T001"}, Status: Pending},
		{ID: "T001", Order: 1, Title: "Root", Status: Done},
		{ID: "T005", Order: 5, Title: "Blocked", Status: Blocked, Blocker: stringPtr("waiting")},
		{ID: "T002", Order: 2, Title: "Branch A", DependsOn: []string{"T001"}, Status: Pending},
		{ID: "T004", Order: 4, Title: "Join", DependsOn: []string{"T002", "T003"}, Status: Pending},
	}}
	ready := ReadyTasks(current)
	ids := make([]string, 0, len(ready))
	for _, task := range ready {
		ids = append(ids, task.ID)
	}
	want := []string{"T002", "T003", "T006"}
	if strings.Join(ids, ",") != strings.Join(want, ",") {
		t.Fatalf("got ready %v, want %v", ids, want)
	}
}

func TestTransitionsAllowMultipleActiveTasksAndClearBlocker(t *testing.T) {
	current := Graph{Version: Version, Tasks: []Task{
		{ID: "T001", Order: 1, Title: "Root", Status: Pending},
		{ID: "T002", Order: 2, Title: "Branch A", DependsOn: []string{"T001"}, Status: Pending},
		{ID: "T003", Order: 3, Title: "Branch B", DependsOn: []string{"T001"}, Status: Pending},
	}}
	if err := Start(&current, "T002"); err == nil || !strings.Contains(err.Error(), "T001(pending)") {
		t.Fatalf("expected dependency gate, got %v", err)
	}
	if err := Start(&current, "T001"); err != nil {
		t.Fatal(err)
	}
	if err := MarkDone(&current, "T001"); err != nil {
		t.Fatal(err)
	}
	if err := Start(&current, "T002"); err != nil {
		t.Fatal(err)
	}
	if err := Start(&current, "T003"); err != nil {
		t.Fatal(err)
	}
	if current.Tasks[1].Status != Active || current.Tasks[2].Status != Active {
		t.Fatalf("independent tasks were not both active: %+v", current.Tasks)
	}
	if err := Block(&current, "T002", "external fixture missing"); err != nil {
		t.Fatal(err)
	}
	if err := Start(&current, "T002"); err != nil {
		t.Fatal(err)
	}
	if current.Tasks[1].Blocker != nil || current.Tasks[1].Status != Active {
		t.Fatalf("blocker was not cleared: %+v", current.Tasks[1])
	}
}

func TestInvalidTransitionMatrix(t *testing.T) {
	current := Graph{Version: Version, Tasks: []Task{
		{ID: "P", Order: 1, Title: "Pending", Status: Pending},
		{ID: "A", Order: 2, Title: "Active", Status: Active},
		{ID: "B", Order: 3, Title: "Blocked", Status: Blocked, Blocker: stringPtr("reason")},
		{ID: "D", Order: 4, Title: "Done", Status: Done},
	}}
	for _, test := range []struct {
		name string
		call func(*Graph) error
		want bool
	}{
		{name: "pending to active", call: func(g *Graph) error { return Start(g, "P") }, want: true},
		{name: "blocked to active", call: func(g *Graph) error { return Start(g, "B") }, want: true},
		{name: "active to blocked", call: func(g *Graph) error { return Block(g, "A", "reason") }, want: true},
		{name: "active to done", call: func(g *Graph) error { return MarkDone(g, "A") }, want: true},
		{name: "done to active", call: func(g *Graph) error { return Start(g, "D") }, want: false},
		{name: "pending to blocked", call: func(g *Graph) error { return Block(g, "P", "reason") }, want: false},
		{name: "pending to done", call: func(g *Graph) error { return MarkDone(g, "P") }, want: false},
		{name: "active to blocked without reason", call: func(g *Graph) error { return Block(g, "A", " ") }, want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			copy := cloneGraph(current)
			if err := test.call(&copy); (err == nil) != test.want {
				t.Fatalf("got error %v, want success=%v", err, test.want)
			}
		})
	}
}

func baseGraph() Graph {
	return Graph{Version: Version, Tasks: []Task{
		{ID: "T001", Order: 1, Title: "First", Source: "plan.md#first", DependsOn: []string{}, Verify: []string{}, Status: Pending},
		{ID: "T002", Order: 2, Title: "Second", Source: "other.md#second", DependsOn: []string{"T001"}, Verify: []string{}, Status: Pending},
	}}
}

func cloneGraph(current Graph) Graph {
	copy := current
	copy.Tasks = append([]Task(nil), current.Tasks...)
	return copy
}

func writeSource(t *testing.T, root, relative string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("plan source"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func stringPtr(value string) *string {
	return &value
}
