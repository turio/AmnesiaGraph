package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/turio/AmnesiaGraph/internal/graph"
	"github.com/turio/AmnesiaGraph/internal/store"
)

const testGraph = "test"

func TestEndToEndWorkflowAndHotResume(t *testing.T) {
	root := newGitRepo(t)
	writeFile(t, root, "docs/IMPLEMENTATION_PLAN.md", "FULL PLAN BODY DISTINCTIVE_SENTENCE_SHOULD_NOT_BE_PRINTED")
	current := graph.Graph{Version: graph.Version, Tasks: []graph.Task{
		{ID: "T001", Order: 1, Title: "Foundation", Source: "docs/IMPLEMENTATION_PLAN.md#t001", DependsOn: []string{}, Verify: []string{}},
		{ID: "T002", Order: 2, Title: "Active middle", Source: "docs/IMPLEMENTATION_PLAN.md#t002", DependsOn: []string{"T001"}, Verify: []string{"echo verification"}},
		{ID: "T003", Order: 3, Title: "Next step", Source: "docs/IMPLEMENTATION_PLAN.md#t003", DependsOn: []string{"T002"}, Verify: []string{}},
		{ID: "T004", Order: 4, Title: "Unrelated pending", Source: "docs/IMPLEMENTATION_PLAN.md#t004", DependsOn: []string{"T005"}, Verify: []string{}},
		{ID: "T005", Order: 5, Title: "Blocked fixture", Source: "docs/IMPLEMENTATION_PLAN.md#t005", DependsOn: []string{}, Verify: []string{}},
	}}
	input := writeInput(t, root, current)

	stdout, stderr, code := run(t, root, "init", testGraph, input)
	if code != 0 || !strings.Contains(stdout, "INITIALIZED "+testGraph) || stderr != "" {
		t.Fatalf("init: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if _, stderr, code = run(t, root, "start", "T005"); code != 0 || stderr != "" {
		t.Fatalf("start blockable fixture: code=%d stderr=%q", code, stderr)
	}
	if _, stderr, code = run(t, root, "block", "T005", "waiting", "for", "fixture"); code != 0 || stderr != "" {
		t.Fatalf("block fixture: code=%d stderr=%q", code, stderr)
	}
	stdout, stderr, code = run(t, root, "ready")
	if code != 0 || stdout != "T001  Foundation\n" || stderr != "" {
		t.Fatalf("ready before work: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	_, stderr, code = run(t, root, "start", "T002")
	if code == 0 || !strings.Contains(stderr, "DEPENDENCIES_INCOMPLETE T002: T001(pending)") {
		t.Fatalf("dependency gate: code=%d stderr=%q", code, stderr)
	}

	if _, stderr, code = run(t, root, "start", "T001"); code != 0 || stderr != "" {
		t.Fatalf("start root: code=%d stderr=%q", code, stderr)
	}
	if stdout, stderr, code = run(t, root, "done", "T001"); code != 0 || stdout != "DONE T001\n" || stderr != "" {
		t.Fatalf("done root: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	stdout, _, code = run(t, root, "ready")
	if code != 0 || stdout != "T002  Active middle\n" {
		t.Fatalf("ready after root: code=%d stdout=%q", code, stdout)
	}
	if _, stderr, code = run(t, root, "start", "T002"); code != 0 || stderr != "" {
		t.Fatalf("start middle: code=%d stderr=%q", code, stderr)
	}

	stdout, stderr, code = run(t, root, "resume")
	if code != 0 || stderr != "" {
		t.Fatalf("resume: code=%d stderr=%q", code, stderr)
	}
	for _, expected := range []string{
		"GRAPH " + testGraph,
		"ACTIVE",
		"T002  Active middle",
		"READ   docs/IMPLEMENTATION_PLAN.md#t002  (full original plan instructions)",
		"DEPS   T001 done",
		"VERIFY echo verification",
		"NEXT   T003  Next step",
		"BLOCKED",
		"T005  Blocked fixture — waiting for fixture",
	} {
		if !strings.Contains(stdout, expected) {
			t.Errorf("resume missing %q in:\n%s", expected, stdout)
		}
	}
	for _, absent := range []string{"FULL PLAN BODY DISTINCTIVE_SENTENCE_SHOULD_NOT_BE_PRINTED", "Unrelated pending"} {
		if strings.Contains(stdout, absent) {
			t.Errorf("resume leaked unrelated content %q:\n%s", absent, stdout)
		}
	}

	stdout, stderr, code = run(t, root, "done", "T002")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "VERIFY T002 1/1 PASS") || !strings.Contains(stdout, "DONE T002") {
		t.Fatalf("verified done: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	stdout, _, code = run(t, root, "ready")
	if code != 0 || stdout != "T003  Next step\n" {
		t.Fatalf("ready after verified done: code=%d stdout=%q", code, stdout)
	}
}

func TestRepoIsolationAndOutsideRepoGate(t *testing.T) {
	first := newGitRepo(t)
	second := newGitRepo(t)
	initSingleTask(t, first, "A-7")
	initSingleTask(t, second, "B-3")
	if _, stderr, code := run(t, first, "start", "A-7"); code != 0 || stderr != "" {
		t.Fatalf("start first: code=%d stderr=%q", code, stderr)
	}
	if _, stderr, code := run(t, second, "start", "B-3"); code != 0 || stderr != "" {
		t.Fatalf("start second: code=%d stderr=%q", code, stderr)
	}
	if _, stderr, code := run(t, first, "done", "A-7"); code != 0 || stderr != "" {
		t.Fatalf("complete first: code=%d stderr=%q", code, stderr)
	}
	if _, stderr, code := run(t, second, "done", "B-3"); code != 0 || stderr != "" {
		t.Fatalf("complete second: code=%d stderr=%q", code, stderr)
	}
	if _, stderr, code := run(t, first, "start", "A-7-next"); code != 0 || stderr != "" {
		t.Fatalf("start first successor: code=%d stderr=%q", code, stderr)
	}
	if _, stderr, code := run(t, second, "start", "B-3-next"); code != 0 || stderr != "" {
		t.Fatalf("start second successor: code=%d stderr=%q", code, stderr)
	}

	firstOutput, _, code := run(t, first, "resume")
	if code != 0 || !strings.Contains(firstOutput, "A-7") || strings.Contains(firstOutput, "B-3") {
		t.Fatalf("first repo leaked state: code=%d output=%q", code, firstOutput)
	}
	secondOutput, _, code := run(t, second, "resume")
	if code != 0 || !strings.Contains(secondOutput, "B-3") || strings.Contains(secondOutput, "A-7") {
		t.Fatalf("second repo leaked state: code=%d output=%q", code, secondOutput)
	}

	outside := t.TempDir()
	_, stderr, code := run(t, outside, "resume")
	if code == 0 || !strings.Contains(stderr, "NOT_IN_REPO") {
		t.Fatalf("outside repo was not rejected: code=%d stderr=%q", code, stderr)
	}
}

func TestFailedInitLeavesExistingGraphUnchanged(t *testing.T) {
	root := newGitRepo(t)
	writeFile(t, root, "plan.md", "plan")
	valid := graph.Graph{Version: graph.Version, Tasks: []graph.Task{{ID: "T001", Order: 1, Title: "Valid", Source: "plan.md#valid", DependsOn: []string{}, Verify: []string{}}}}
	validInput := writeInput(t, root, valid)
	if _, stderr, code := run(t, root, "init", testGraph, validInput); code != 0 || stderr != "" {
		t.Fatalf("valid init: code=%d stderr=%q", code, stderr)
	}
	before, err := os.ReadFile(store.GraphPath(root, testGraph))
	if err != nil {
		t.Fatal(err)
	}
	invalid := graph.Graph{Version: graph.Version, Tasks: []graph.Task{{ID: "T002", Order: 1, Title: "Invalid", Source: "plan.md#invalid", DependsOn: []string{"missing"}, Verify: []string{}}}}
	invalidInput := writeInput(t, root, invalid)
	_, stderr, code := run(t, root, "init", "other", invalidInput)
	if code == 0 || !strings.Contains(stderr, "unknown task") {
		t.Fatalf("invalid init: code=%d stderr=%q", code, stderr)
	}
	after, err := os.ReadFile(store.GraphPath(root, testGraph))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("failed init replaced existing graph")
	}
	if _, err := os.Stat(store.GraphPath(root, "other")); !os.IsNotExist(err) {
		t.Fatalf("failed init created partial graph, stat: %v", err)
	}
}

func TestInitRejectsDuplicateGraphName(t *testing.T) {
	root := newGitRepo(t)
	writeFile(t, root, "plan.md", "plan")
	first := graph.Graph{Version: graph.Version, Tasks: []graph.Task{{ID: "T001", Order: 1, Title: "First", Source: "plan.md#one"}}}
	second := graph.Graph{Version: graph.Version, Tasks: []graph.Task{{ID: "T001", Order: 1, Title: "Second", Source: "plan.md#two"}}}
	if _, stderr, code := run(t, root, "init", "dup", writeInput(t, root, first)); code != 0 || stderr != "" {
		t.Fatalf("first init: code=%d stderr=%q", code, stderr)
	}
	before, _ := os.ReadFile(store.GraphPath(root, "dup"))
	_, stderr, code := run(t, root, "init", "dup", writeInput(t, root, second))
	if code == 0 || !strings.Contains(stderr, "GRAPH_EXISTS dup") {
		t.Fatalf("duplicate init: code=%d stderr=%q", code, stderr)
	}
	after, _ := os.ReadFile(store.GraphPath(root, "dup"))
	if string(before) != string(after) {
		t.Fatal("duplicate init replaced existing graph")
	}
}

func TestInitRejectsProgressedSeedStatesAndAllowsPendingDefaults(t *testing.T) {
	for _, test := range []struct {
		name       string
		status     graph.Status
		blocker    *string
		verify     []string
		wantResult bool
	}{
		{name: "omitted status", wantResult: true},
		{name: "explicit pending", status: graph.Pending, wantResult: true},
		{name: "active", status: graph.Active},
		{name: "blocked with reason", status: graph.Blocked, blocker: stringPtr("waiting")},
		{name: "done without verification", status: graph.Done},
		{name: "done with verification", status: graph.Done, verify: []string{"echo should-not-run"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := newGitRepo(t)
			writeFile(t, root, "plan.md", "plan")
			current := graph.Graph{Version: graph.Version, Tasks: []graph.Task{{
				ID: "T001", Order: 1, Title: "Task", Source: "plan.md#task", DependsOn: []string{}, Verify: test.verify, Status: test.status, Blocker: test.blocker,
			}}}
			input := writeInput(t, root, current)
			_, stderr, code := run(t, root, "init", testGraph, input)
			if test.wantResult {
				if code != 0 || stderr != "" {
					t.Fatalf("init failed: code=%d stderr=%q", code, stderr)
				}
				loaded, err := store.Load(root, testGraph)
				if err != nil {
					t.Fatal(err)
				}
				if loaded.Tasks[0].Status != graph.Pending || loaded.Tasks[0].Blocker != nil {
					t.Fatalf("unexpected initialized state: %+v", loaded.Tasks[0])
				}
				return
			}
			if code == 0 || !strings.Contains(stderr, "init requires pending status") {
				t.Fatalf("progressed seed accepted: code=%d stderr=%q", code, stderr)
			}
			if _, err := os.Stat(store.GraphPath(root, testGraph)); !os.IsNotExist(err) {
				t.Fatalf("rejected init installed state, stat error: %v", err)
			}
		})
	}
}

func TestInitRejectsEmptyGraph(t *testing.T) {
	root := newGitRepo(t)
	input := writeInput(t, root, graph.Graph{Version: graph.Version, Tasks: []graph.Task{}})
	_, stderr, code := run(t, root, "init", testGraph, input)
	if code == 0 || !strings.Contains(stderr, "graph must contain at least one task") {
		t.Fatalf("empty init accepted: code=%d stderr=%q", code, stderr)
	}
	if _, err := os.Stat(store.GraphPath(root, testGraph)); !os.IsNotExist(err) {
		t.Fatalf("empty init installed state, stat error: %v", err)
	}
}

func TestInitRejectsInvalidGraphName(t *testing.T) {
	root := newGitRepo(t)
	writeFile(t, root, "plan.md", "plan")
	current := graph.Graph{Version: graph.Version, Tasks: []graph.Task{{ID: "T001", Order: 1, Title: "T", Source: "plan.md#t"}}}
	input := writeInput(t, root, current)
	for _, bad := range []string{"Bad", "has space", "a/b", "../x"} {
		_, stderr, code := run(t, root, "init", bad, input)
		if code == 0 || !strings.Contains(stderr, "INVALID_GRAPH_NAME") {
			t.Fatalf("bad name %q accepted: code=%d stderr=%q", bad, code, stderr)
		}
	}
}

func TestValidateRejectsCorruptedProgressedDependencyState(t *testing.T) {
	root := newGitRepo(t)
	writeFile(t, root, "plan.md", "plan")
	current := graph.Graph{Version: graph.Version, Tasks: []graph.Task{
		{ID: "P", Order: 1, Title: "Parent", Source: "plan.md#parent", DependsOn: []string{}, Verify: []string{}},
		{ID: "C", Order: 2, Title: "Child", Source: "plan.md#child", DependsOn: []string{"P"}, Verify: []string{}},
	}}
	input := writeInput(t, root, current)
	if _, stderr, code := run(t, root, "init", testGraph, input); code != 0 || stderr != "" {
		t.Fatalf("valid init: code=%d stderr=%q", code, stderr)
	}
	installed, err := store.Load(root, testGraph)
	if err != nil {
		t.Fatal(err)
	}
	installed.Tasks[1].Status = graph.Done
	corrupted, err := json.MarshalIndent(installed, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.GraphPath(root, testGraph), corrupted, 0o644); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := run(t, root, "validate")
	if code == 0 || !strings.Contains(stderr, "task C has incomplete dependency P (pending)") {
		t.Fatalf("corrupted state accepted: code=%d stderr=%q", code, stderr)
	}
}

func TestRejectedProgressedReinitPreservesInstalledGraph(t *testing.T) {
	root := newGitRepo(t)
	writeFile(t, root, "plan.md", "plan")
	valid := graph.Graph{Version: graph.Version, Tasks: []graph.Task{{
		ID: "T001", Order: 1, Title: "Valid", Source: "plan.md#valid", DependsOn: []string{}, Verify: []string{},
	}}}
	validInput := writeInput(t, root, valid)
	if _, stderr, code := run(t, root, "init", testGraph, validInput); code != 0 || stderr != "" {
		t.Fatalf("valid init: code=%d stderr=%q", code, stderr)
	}
	before, err := os.ReadFile(store.GraphPath(root, testGraph))
	if err != nil {
		t.Fatal(err)
	}
	progressed := graph.Graph{Version: graph.Version, Tasks: []graph.Task{{
		ID: "T001", Order: 1, Title: "Invalid replacement", Source: "plan.md#invalid", DependsOn: []string{}, Verify: []string{}, Status: graph.Done,
	}}}
	progressedInput := writeInput(t, root, progressed)
	_, stderr, code := run(t, root, "init", "other", progressedInput)
	if code == 0 || !strings.Contains(stderr, "init requires pending status") {
		t.Fatalf("invalid replacement: code=%d stderr=%q", code, stderr)
	}
	after, err := os.ReadFile(store.GraphPath(root, testGraph))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("rejected progressed init changed installed graph")
	}
}

func TestMultipleReadyAndActiveTasksAreAllowed(t *testing.T) {
	root := newGitRepo(t)
	writeFile(t, root, "plan.md", "plan")
	current := graph.Graph{Version: graph.Version, Tasks: []graph.Task{
		{ID: "T001", Order: 1, Title: "Root", Source: "plan.md#one"},
		{ID: "T002", Order: 2, Title: "Branch A", Source: "plan.md#two", DependsOn: []string{"T001"}},
		{ID: "T003", Order: 3, Title: "Branch B", Source: "plan.md#three", DependsOn: []string{"T001"}},
	}}
	input := writeInput(t, root, current)
	if _, stderr, code := run(t, root, "init", testGraph, input); code != 0 || stderr != "" {
		t.Fatalf("init: code=%d stderr=%q", code, stderr)
	}
	if _, stderr, code := run(t, root, "start", "T001"); code != 0 || stderr != "" {
		t.Fatalf("start root: code=%d stderr=%q", code, stderr)
	}
	if _, stderr, code := run(t, root, "done", "T001"); code != 0 || stderr != "" {
		t.Fatalf("done root: code=%d stderr=%q", code, stderr)
	}
	stdout, _, code := run(t, root, "ready")
	if code != 0 || stdout != "T002  Branch A\nT003  Branch B\n" {
		t.Fatalf("ready branches: code=%d stdout=%q", code, stdout)
	}
	for _, id := range []string{"T002", "T003"} {
		if _, stderr, code := run(t, root, "start", id); code != 0 || stderr != "" {
			t.Fatalf("start %s: code=%d stderr=%q", id, code, stderr)
		}
	}
	stdout, _, code = run(t, root, "resume")
	if code != 0 || !strings.Contains(stdout, "T002  Branch A") || !strings.Contains(stdout, "T003  Branch B") {
		t.Fatalf("multiple active tasks rejected or hidden: code=%d output=%q", code, stdout)
	}
}

func TestBlockLifecycleAndVerificationFailure(t *testing.T) {
	root := newGitRepo(t)
	writeFile(t, root, "plan.md", "plan")
	failing := "false"
	if runtime.GOOS == "windows" {
		failing = "exit /b 1"
	}
	current := graph.Graph{Version: graph.Version, Tasks: []graph.Task{
		{ID: "T001", Order: 1, Title: "Needs verification", Source: "plan.md#one", Verify: []string{failing}},
	}}
	input := writeInput(t, root, current)
	if _, stderr, code := run(t, root, "init", testGraph, input); code != 0 || stderr != "" {
		t.Fatalf("init: code=%d stderr=%q", code, stderr)
	}
	if _, stderr, code := run(t, root, "start", "T001"); code != 0 || stderr != "" {
		t.Fatalf("start: code=%d stderr=%q", code, stderr)
	}
	_, stderr, code := run(t, root, "done", "T001")
	if code == 0 || !strings.Contains(stderr, "VERIFY T001 FAIL") || !strings.Contains(stderr, "STATE_UNCHANGED active") {
		t.Fatalf("failed verification: code=%d stderr=%q", code, stderr)
	}
	loaded, err := store.Load(root, testGraph)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Tasks[0].Status != graph.Active {
		t.Fatalf("failed verification changed state: %+v", loaded.Tasks[0])
	}
	if _, stderr, code = run(t, root, "block", "T001", "waiting", "for", "fixture"); code != 0 || stderr != "" {
		t.Fatalf("block: code=%d stderr=%q", code, stderr)
	}
	resume, _, code := run(t, root, "resume")
	if code != 0 || !strings.Contains(resume, "T001  Needs verification — waiting for fixture") {
		t.Fatalf("blocked resume: code=%d output=%q", code, resume)
	}
	if _, stderr, code = run(t, root, "start", "T001"); code != 0 || stderr != "" {
		t.Fatalf("reactivate: code=%d stderr=%q", code, stderr)
	}
	loaded, err = store.Load(root, testGraph)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Tasks[0].Status != graph.Active || loaded.Tasks[0].Blocker != nil {
		t.Fatalf("reactivation did not clear blocker: %+v", loaded.Tasks[0])
	}
}

func TestHelpAndVersionDoNotRequireRepository(t *testing.T) {
	outside := t.TempDir()
	stdout, stderr, code := run(t, outside, "help")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "amnesia") {
		t.Fatalf("help: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	stdout, stderr, code = run(t, outside, "version")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "amnesia 0.2.0") {
		t.Fatalf("version: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestUninitializedRepositoryIsRejected(t *testing.T) {
	root := newGitRepo(t)
	_, stderr, code := run(t, root, "ready")
	if code == 0 || strings.TrimSpace(stderr) != "NOT_INITIALIZED" {
		t.Fatalf("got code=%d stderr=%q", code, stderr)
	}
	_, stderr, code = run(t, root, "resume")
	if code == 0 || strings.TrimSpace(stderr) != "NOT_INITIALIZED" {
		t.Fatalf("resume uninit: code=%d stderr=%q", code, stderr)
	}
	_, stderr, code = run(t, root, "list")
	if code == 0 || strings.TrimSpace(stderr) != "NOT_INITIALIZED" {
		t.Fatalf("list uninit: code=%d stderr=%q", code, stderr)
	}
}

func TestInterruptedPlanAPlanBAcceptance(t *testing.T) {
	root := newGitRepo(t)
	writeFile(t, root, "plan-a.md", "Plan A body")
	writeFile(t, root, "plan-b.md", "Plan B body")
	planA := graph.Graph{Version: graph.Version, Tasks: []graph.Task{
		{ID: "A-T001", Order: 1, Title: "A first", Source: "plan-a.md#a1"},
		{ID: "A-T002", Order: 2, Title: "A middle", Source: "plan-a.md#a2", DependsOn: []string{"A-T001"}, Verify: []string{"echo a2"}},
		{ID: "A-T003", Order: 3, Title: "A last", Source: "plan-a.md#a3", DependsOn: []string{"A-T002"}},
	}}
	planB := graph.Graph{Version: graph.Version, Tasks: []graph.Task{
		{ID: "B-T001", Order: 1, Title: "B first", Source: "plan-b.md#b1"},
		{ID: "B-T002", Order: 2, Title: "B last", Source: "plan-b.md#b2", DependsOn: []string{"B-T001"}},
	}}
	if _, stderr, code := run(t, root, "init", "plan-a", writeInput(t, root, planA)); code != 0 || stderr != "" {
		t.Fatalf("init plan-a: code=%d stderr=%q", code, stderr)
	}
	if _, stderr, code := run(t, root, "start", "A-T001"); code != 0 || stderr != "" {
		t.Fatalf("start A-T001: %v", stderr)
	}
	if _, stderr, code := run(t, root, "done", "A-T001"); code != 0 || stderr != "" {
		t.Fatalf("done A-T001: %v", stderr)
	}
	if _, stderr, code := run(t, root, "start", "A-T002"); code != 0 || stderr != "" {
		t.Fatalf("start A-T002: %v", stderr)
	}
	if _, stderr, code := run(t, root, "init", "plan-b", writeInput(t, root, planB)); code != 0 || stderr != "" {
		t.Fatalf("init plan-b: code=%d stderr=%q", code, stderr)
	}
	stdout, _, code := run(t, root, "resume")
	if code != 0 || !strings.Contains(stdout, "GRAPH plan-b") {
		t.Fatalf("resume should prefer plan-b: code=%d out=%q", code, stdout)
	}
	if strings.Contains(stdout, "A-T002") {
		t.Fatalf("plan-b resume leaked plan-a: %q", stdout)
	}
	loadedA, err := store.Load(root, "plan-a")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, task := range loadedA.Tasks {
		if task.ID == "A-T002" && task.Status == graph.Active {
			found = true
		}
	}
	if !found {
		t.Fatalf("plan-a A-T002 not preserved active: %+v", loadedA)
	}
	if _, stderr, code := run(t, root, "use", "plan-a"); code != 0 || stderr != "" {
		t.Fatalf("use plan-a: code=%d stderr=%q", code, stderr)
	}
	stdout, _, code = run(t, root, "resume")
	if code != 0 || !strings.Contains(stdout, "GRAPH plan-a") || !strings.Contains(stdout, "A-T002  A middle") {
		t.Fatalf("resume plan-a: code=%d out=%q", code, stdout)
	}
	if !strings.Contains(stdout, "READ   plan-a.md#a2") || !strings.Contains(stdout, "VERIFY echo a2") {
		t.Fatalf("plan-a context lost: %q", stdout)
	}
}

func TestSameTaskIDsAcrossGraphsAreIsolated(t *testing.T) {
	root := newGitRepo(t)
	writeFile(t, root, "a.md", "a")
	writeFile(t, root, "b.md", "b")
	graphA := graph.Graph{Version: graph.Version, Tasks: []graph.Task{{ID: "T001", Order: 1, Title: "A task", Source: "a.md#t"}}}
	graphB := graph.Graph{Version: graph.Version, Tasks: []graph.Task{{ID: "T001", Order: 1, Title: "B task", Source: "b.md#t"}}}
	if _, stderr, code := run(t, root, "init", "graph-a", writeInput(t, root, graphA)); code != 0 || stderr != "" {
		t.Fatalf("init a: %v", stderr)
	}
	if _, stderr, code := run(t, root, "init", "graph-b", writeInput(t, root, graphB)); code != 0 || stderr != "" {
		t.Fatalf("init b: %v", stderr)
	}
	if _, stderr, code := run(t, root, "use", "graph-a"); code != 0 || stderr != "" {
		t.Fatalf("use a: %v", stderr)
	}
	if _, stderr, code := run(t, root, "start", "T001"); code != 0 || stderr != "" {
		t.Fatalf("start a T001: %v", stderr)
	}
	loadedB, _ := store.Load(root, "graph-b")
	if loadedB.Tasks[0].Status != graph.Pending {
		t.Fatalf("start leaked to graph-b: %+v", loadedB)
	}
	if _, stderr, code := run(t, root, "use", "graph-b"); code != 0 || stderr != "" {
		t.Fatalf("use b: %v", stderr)
	}
	stdout, _, _ := run(t, root, "resume")
	if !strings.Contains(stdout, "B task") || strings.Contains(stdout, "A task") {
		t.Fatalf("resume isolation broken: %q", stdout)
	}
	if _, stderr, code := run(t, root, "block", "T001", "need"); code != 0 {
		// T001 in graph-b is pending, block should fail (only active->blocked).
		if !strings.Contains(stderr, "INVALID_TRANSITION") {
			t.Fatalf("expected INVALID_TRANSITION, got %q", stderr)
		}
	} else {
		t.Fatal("block pending should fail")
	}
	// Start in graph-b, then done in graph-b must not affect graph-a.
	if _, stderr, code := run(t, root, "start", "T001"); code != 0 || stderr != "" {
		t.Fatalf("start b: %v", stderr)
	}
	if _, stderr, code := run(t, root, "done", "T001"); code != 0 || stderr != "" {
		t.Fatalf("done b: %v", stderr)
	}
	loadedA, _ := store.Load(root, "graph-a")
	if loadedA.Tasks[0].Status != graph.Active {
		t.Fatalf("graph-a mutated by graph-b ops: %+v", loadedA)
	}
}

func TestTaskCommandsDoNotSearchOtherGraphs(t *testing.T) {
	root := newGitRepo(t)
	writeFile(t, root, "a.md", "a")
	writeFile(t, root, "b.md", "b")
	graphA := graph.Graph{Version: graph.Version, Tasks: []graph.Task{{ID: "ONLY-A", Order: 1, Title: "Only A", Source: "a.md#t"}}}
	graphB := graph.Graph{Version: graph.Version, Tasks: []graph.Task{{ID: "ONLY-B", Order: 1, Title: "Only B", Source: "b.md#t"}}}
	if _, _, code := run(t, root, "init", "ga", writeInput(t, root, graphA)); code != 0 {
		t.Fatal("init ga")
	}
	if _, _, code := run(t, root, "init", "gb", writeInput(t, root, graphB)); code != 0 {
		t.Fatal("init gb")
	}
	// gb is current; ONLY-A exists only in ga.
	_, stderr, code := run(t, root, "start", "ONLY-A")
	if code == 0 || !strings.Contains(stderr, "TASK_NOT_FOUND ONLY-A") {
		t.Fatalf("cross-graph lookup: code=%d stderr=%q", code, stderr)
	}
}

func TestListAndUseBehavior(t *testing.T) {
	root := newGitRepo(t)
	writeFile(t, root, "plan.md", "plan")
	mk := func(id string) graph.Graph {
		return graph.Graph{Version: graph.Version, Tasks: []graph.Task{{ID: id, Order: 1, Title: "Task " + id, Source: "plan.md#t"}}}
	}
	if _, _, code := run(t, root, "init", "alpha", writeInput(t, root, mk("A"))); code != 0 {
		t.Fatal("init alpha")
	}
	time.Sleep(20 * time.Millisecond)
	if _, _, code := run(t, root, "init", "beta", writeInput(t, root, mk("B"))); code != 0 {
		t.Fatal("init beta")
	}
	stdout, _, code := run(t, root, "list")
	if code != 0 {
		t.Fatalf("list: %d", code)
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "* beta") || !strings.HasPrefix(lines[1], "  alpha") {
		t.Fatalf("list order should be current first: %q", stdout)
	}
	if !strings.Contains(stdout, "unfinished") {
		t.Fatalf("list should mark unfinished: %q", stdout)
	}
	stdout, stderr, code := run(t, root, "use", "alpha")
	if code != 0 || !strings.Contains(stdout, "CURRENT alpha") || stderr != "" {
		t.Fatalf("use alpha: code=%d out=%q err=%q", code, stdout, stderr)
	}
	stdout, _, _ = run(t, root, "list")
	if !strings.HasPrefix(strings.Split(strings.TrimSpace(stdout), "\n")[0], "* alpha") {
		t.Fatalf("list after use: %q", stdout)
	}
	// use missing
	_, stderr, code = run(t, root, "use", "missing")
	if code == 0 || !strings.Contains(stderr, "GRAPH_NOT_FOUND") {
		t.Fatalf("use missing: code=%d stderr=%q", code, stderr)
	}
	// use invalid name
	_, stderr, code = run(t, root, "use", "BadName")
	if code == 0 || !strings.Contains(stderr, "INVALID_GRAPH_NAME") {
		t.Fatalf("use bad name: code=%d stderr=%q", code, stderr)
	}
}

func TestUseRejectsCompleteGraph(t *testing.T) {
	root := newGitRepo(t)
	writeFile(t, root, "plan.md", "plan")
	single := graph.Graph{Version: graph.Version, Tasks: []graph.Task{{ID: "T001", Order: 1, Title: "Solo", Source: "plan.md#t"}}}
	if _, _, code := run(t, root, "init", "solo", writeInput(t, root, single)); code != 0 {
		t.Fatal("init solo")
	}
	if _, _, code := run(t, root, "start", "T001"); code != 0 {
		t.Fatal("start")
	}
	if _, _, code := run(t, root, "done", "T001"); code != 0 {
		t.Fatal("done")
	}
	other := graph.Graph{Version: graph.Version, Tasks: []graph.Task{{ID: "T001", Order: 1, Title: "Other", Source: "plan.md#t"}}}
	if _, _, code := run(t, root, "init", "other", writeInput(t, root, other)); code != 0 {
		t.Fatal("init other")
	}
	_, stderr, code := run(t, root, "use", "solo")
	if code == 0 || !strings.Contains(stderr, "GRAPH_COMPLETE solo") {
		t.Fatalf("use complete: code=%d stderr=%q", code, stderr)
	}
}

func TestResumeFallbackBranches(t *testing.T) {
	newGraph := func(title string) graph.Graph {
		return graph.Graph{Version: graph.Version, Tasks: []graph.Task{{ID: "T001", Order: 1, Title: title, Source: "plan.md#t"}}}
	}
	t.Run("current unfinished wins over newer mtime", func(t *testing.T) {
		root := newGitRepo(t)
		writeFile(t, root, "plan.md", "plan")
		if _, _, code := run(t, root, "init", "old", writeInput(t, root, newGraph("Old"))); code != 0 {
			t.Fatal("init old")
		}
		time.Sleep(20 * time.Millisecond)
		if _, _, code := run(t, root, "init", "new", writeInput(t, root, newGraph("New"))); code != 0 {
			t.Fatal("init new")
		}
		if _, _, code := run(t, root, "use", "old"); code != 0 {
			t.Fatal("use old")
		}
		stdout, _, code := run(t, root, "resume")
		if code != 0 || !strings.Contains(stdout, "GRAPH old") {
			t.Fatalf("expected current old: %q", stdout)
		}
	})
	t.Run("missing current falls back to newest unfinished and persists", func(t *testing.T) {
		root := newGitRepo(t)
		writeFile(t, root, "plan.md", "plan")
		if _, _, code := run(t, root, "init", "ga", writeInput(t, root, newGraph("A"))); code != 0 {
			t.Fatal("init ga")
		}
		oldTime := time.Now().Add(-2 * time.Hour)
		_ = os.Chtimes(store.GraphPath(root, "ga"), oldTime, oldTime)
		if _, _, code := run(t, root, "init", "gb", writeInput(t, root, newGraph("B"))); code != 0 {
			t.Fatal("init gb")
		}
		newTime := time.Now().Add(-1 * time.Hour)
		_ = os.Chtimes(store.GraphPath(root, "gb"), newTime, newTime)
		if err := os.Remove(store.CurrentPath(root)); err != nil {
			t.Fatal(err)
		}
		stdout, _, code := run(t, root, "resume")
		if code != 0 || !strings.Contains(stdout, "GRAPH gb") {
			t.Fatalf("fallback: %q", stdout)
		}
		current, err := store.Current(root)
		if err != nil || current != "gb" {
			t.Fatalf("fallback not persisted: %q %v", current, err)
		}
	})
	t.Run("current pointing to missing falls back", func(t *testing.T) {
		root := newGitRepo(t)
		writeFile(t, root, "plan.md", "plan")
		if _, _, code := run(t, root, "init", "ga", writeInput(t, root, newGraph("A"))); code != 0 {
			t.Fatal("init ga")
		}
		if err := store.SetCurrent(root, "ghost"); err != nil {
			t.Fatal(err)
		}
		stdout, _, code := run(t, root, "resume")
		if code != 0 || !strings.Contains(stdout, "GRAPH ga") {
			t.Fatalf("ghost fallback: %q", stdout)
		}
	})
	t.Run("complete current falls back to newest unfinished", func(t *testing.T) {
		root := newGitRepo(t)
		writeFile(t, root, "plan.md", "plan")
		if _, _, code := run(t, root, "init", "finish", writeInput(t, root, newGraph("Finish"))); code != 0 {
			t.Fatal("init finish")
		}
		if _, _, code := run(t, root, "start", "T001"); code != 0 {
			t.Fatal("start")
		}
		if _, _, code := run(t, root, "done", "T001"); code != 0 {
			t.Fatal("done")
		}
		if _, _, code := run(t, root, "init", "next", writeInput(t, root, newGraph("Next"))); code != 0 {
			t.Fatal("init next")
		}
		// Force current back to the complete graph.
		if err := store.SetCurrent(root, "finish"); err != nil {
			t.Fatal(err)
		}
		stdout, _, code := run(t, root, "resume")
		if code != 0 || !strings.Contains(stdout, "GRAPH next") {
			t.Fatalf("complete fallback: %q", stdout)
		}
	})
	t.Run("newest complete skipped", func(t *testing.T) {
		root := newGitRepo(t)
		writeFile(t, root, "plan.md", "plan")
		if _, _, code := run(t, root, "init", "work", writeInput(t, root, newGraph("Work"))); code != 0 {
			t.Fatal("init work")
		}
		oldTime := time.Now().Add(-2 * time.Hour)
		_ = os.Chtimes(store.GraphPath(root, "work"), oldTime, oldTime)
		if _, _, code := run(t, root, "init", "complete-me", writeInput(t, root, newGraph("Done"))); code != 0 {
			t.Fatal("init complete-me")
		}
		if _, _, code := run(t, root, "start", "T001"); code != 0 {
			t.Fatal("start")
		}
		if _, _, code := run(t, root, "done", "T001"); code != 0 {
			t.Fatal("done")
		}
		if err := os.Remove(store.CurrentPath(root)); err != nil {
			t.Fatal(err)
		}
		stdout, _, code := run(t, root, "resume")
		if code != 0 || !strings.Contains(stdout, "GRAPH work") {
			t.Fatalf("should skip complete newest: %q", stdout)
		}
	})
	t.Run("mtime tie uses name tie-breaker", func(t *testing.T) {
		root := newGitRepo(t)
		writeFile(t, root, "plan.md", "plan")
		if _, _, code := run(t, root, "init", "beta", writeInput(t, root, newGraph("Beta"))); code != 0 {
			t.Fatal("init beta")
		}
		if _, _, code := run(t, root, "init", "alpha", writeInput(t, root, newGraph("Alpha"))); code != 0 {
			t.Fatal("init alpha")
		}
		fixed := time.Now().Add(-1 * time.Hour).Truncate(time.Second)
		_ = os.Chtimes(store.GraphPath(root, "beta"), fixed, fixed)
		_ = os.Chtimes(store.GraphPath(root, "alpha"), fixed, fixed)
		if err := os.Remove(store.CurrentPath(root)); err != nil {
			t.Fatal(err)
		}
		stdout, _, code := run(t, root, "resume")
		if code != 0 || !strings.Contains(stdout, "GRAPH alpha") {
			t.Fatalf("tie-breaker: %q", stdout)
		}
	})
	t.Run("all complete returns NO_ACTIVE_GRAPH", func(t *testing.T) {
		root := newGitRepo(t)
		writeFile(t, root, "plan.md", "plan")
		if _, _, code := run(t, root, "init", "solo", writeInput(t, root, newGraph("Solo"))); code != 0 {
			t.Fatal("init solo")
		}
		if _, _, code := run(t, root, "start", "T001"); code != 0 {
			t.Fatal("start")
		}
		if _, _, code := run(t, root, "done", "T001"); code != 0 {
			t.Fatal("done")
		}
		_, stderr, code := run(t, root, "resume")
		if code == 0 || !strings.Contains(stderr, "NO_ACTIVE_GRAPH") {
			t.Fatalf("expected NO_ACTIVE_GRAPH: code=%d stderr=%q", code, stderr)
		}
	})
}

func TestLegacyMigrationThroughCLI(t *testing.T) {
	root := newGitRepo(t)
	writeFile(t, root, "plan.md", "plan body")
	legacy := `{"version":1,"tasks":[{"id":"T001","order":1,"title":"Legacy","source":"plan.md#t","depends_on":[],"verify":[],"status":"active","blocker":null}]}` + "\n"
	dir := filepath.Join(root, ".amnesiagraph")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "graph.json"), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _, code := run(t, root, "resume")
	if code != 0 || !strings.Contains(stdout, "GRAPH default") || !strings.Contains(stdout, "T001  Legacy") {
		t.Fatalf("migrated resume: code=%d out=%q", code, stdout)
	}
	if _, err := os.Stat(filepath.Join(dir, "graph.json")); !os.IsNotExist(err) {
		t.Fatalf("legacy should be moved: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "graphs", "default.json"))
	if string(data) != legacy {
		t.Fatalf("bytes changed: %q", data)
	}
}

func TestMigrationConflictThroughCLI(t *testing.T) {
	root := newGitRepo(t)
	writeFile(t, root, "plan.md", "plan")
	mk := graph.Graph{Version: graph.Version, Tasks: []graph.Task{{ID: "T001", Order: 1, Title: "T", Source: "plan.md#t"}}}
	if _, _, code := run(t, root, "init", "named", writeInput(t, root, mk)); code != 0 {
		t.Fatal("init named")
	}
	// Re-create legacy file to force conflict.
	if err := os.WriteFile(filepath.Join(root, ".amnesiagraph", "graph.json"), []byte(`{"version":1,"tasks":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := run(t, root, "resume")
	if code == 0 || !strings.Contains(stderr, "MIGRATION_CONFLICT") {
		t.Fatalf("expected conflict: code=%d stderr=%q", code, stderr)
	}
}

func run(t *testing.T, directory string, args ...string) (string, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := RunInDir(args, directory, &stdout, &stderr)
	return stdout.String(), stderr.String(), code
}

func newGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	command := exec.Command("git", "init", "--quiet")
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, output)
	}
	return root
}

func initSingleTask(t *testing.T, root, id string) {
	t.Helper()
	writeFile(t, root, "plan.md", "plan for "+id)
	current := graph.Graph{Version: graph.Version, Tasks: []graph.Task{{
		ID: id, Order: 1, Title: "Task " + id, Source: "plan.md#task", DependsOn: []string{}, Verify: []string{},
	}, {
		ID: id + "-next", Order: 2, Title: "Successor " + id, Source: "plan.md#successor", DependsOn: []string{id}, Verify: []string{},
	}}}
	input := writeInput(t, root, current)
	if _, stderr, code := run(t, root, "init", testGraph, input); code != 0 || stderr != "" {
		t.Fatalf("init %s: code=%d stderr=%q", id, code, stderr)
	}
}

func writeInput(t *testing.T, root string, current graph.Graph) string {
	t.Helper()
	data, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "normalized.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeFile(t *testing.T, root, relative, contents string) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func stringPtr(value string) *string {
	return &value
}
