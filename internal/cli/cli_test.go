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

	"github.com/turio/AmnesiaGraph/internal/graph"
	"github.com/turio/AmnesiaGraph/internal/store"
)

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

	stdout, stderr, code := run(t, root, "init", input)
	if code != 0 || !strings.Contains(stdout, "INITIALIZED") || stderr != "" {
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
	if _, stderr, code := run(t, root, "init", validInput); code != 0 || stderr != "" {
		t.Fatalf("valid init: code=%d stderr=%q", code, stderr)
	}
	before, err := os.ReadFile(store.GraphPath(root))
	if err != nil {
		t.Fatal(err)
	}
	invalid := graph.Graph{Version: graph.Version, Tasks: []graph.Task{{ID: "T002", Order: 1, Title: "Invalid", Source: "plan.md#invalid", DependsOn: []string{"missing"}, Verify: []string{}}}}
	invalidInput := writeInput(t, root, invalid)
	_, stderr, code := run(t, root, "init", invalidInput)
	if code == 0 || !strings.Contains(stderr, "unknown task") {
		t.Fatalf("invalid init: code=%d stderr=%q", code, stderr)
	}
	after, err := os.ReadFile(store.GraphPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("failed init replaced existing graph")
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
			_, stderr, code := run(t, root, "init", input)
			if test.wantResult {
				if code != 0 || stderr != "" {
					t.Fatalf("init failed: code=%d stderr=%q", code, stderr)
				}
				loaded, err := store.Load(root)
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
			if _, err := os.Stat(store.GraphPath(root)); !os.IsNotExist(err) {
				t.Fatalf("rejected init installed state, stat error: %v", err)
			}
		})
	}
}

func TestInitRejectsEmptyGraph(t *testing.T) {
	root := newGitRepo(t)
	input := writeInput(t, root, graph.Graph{Version: graph.Version, Tasks: []graph.Task{}})
	_, stderr, code := run(t, root, "init", input)
	if code == 0 || !strings.Contains(stderr, "graph must contain at least one task") {
		t.Fatalf("empty init accepted: code=%d stderr=%q", code, stderr)
	}
	if _, err := os.Stat(store.GraphPath(root)); !os.IsNotExist(err) {
		t.Fatalf("empty init installed state, stat error: %v", err)
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
	if _, stderr, code := run(t, root, "init", input); code != 0 || stderr != "" {
		t.Fatalf("valid init: code=%d stderr=%q", code, stderr)
	}
	installed, err := store.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	installed.Tasks[1].Status = graph.Done
	corrupted, err := json.MarshalIndent(installed, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.GraphPath(root), corrupted, 0o644); err != nil {
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
	if _, stderr, code := run(t, root, "init", validInput); code != 0 || stderr != "" {
		t.Fatalf("valid init: code=%d stderr=%q", code, stderr)
	}
	before, err := os.ReadFile(store.GraphPath(root))
	if err != nil {
		t.Fatal(err)
	}
	progressed := graph.Graph{Version: graph.Version, Tasks: []graph.Task{{
		ID: "T001", Order: 1, Title: "Invalid replacement", Source: "plan.md#invalid", DependsOn: []string{}, Verify: []string{}, Status: graph.Done,
	}}}
	progressedInput := writeInput(t, root, progressed)
	_, stderr, code := run(t, root, "init", progressedInput)
	if code == 0 || !strings.Contains(stderr, "init requires pending status") {
		t.Fatalf("invalid replacement: code=%d stderr=%q", code, stderr)
	}
	after, err := os.ReadFile(store.GraphPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
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
	if _, stderr, code := run(t, root, "init", input); code != 0 || stderr != "" {
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
	if _, stderr, code := run(t, root, "init", input); code != 0 || stderr != "" {
		t.Fatalf("init: code=%d stderr=%q", code, stderr)
	}
	if _, stderr, code := run(t, root, "start", "T001"); code != 0 || stderr != "" {
		t.Fatalf("start: code=%d stderr=%q", code, stderr)
	}
	_, stderr, code := run(t, root, "done", "T001")
	if code == 0 || !strings.Contains(stderr, "VERIFY T001 FAIL") || !strings.Contains(stderr, "STATE_UNCHANGED active") {
		t.Fatalf("failed verification: code=%d stderr=%q", code, stderr)
	}
	loaded, err := store.Load(root)
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
	loaded, err = store.Load(root)
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
	if code != 0 || stderr != "" || !strings.Contains(stdout, "amnesia 0.1.0") {
		t.Fatalf("version: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestUninitializedRepositoryIsRejected(t *testing.T) {
	root := newGitRepo(t)
	_, stderr, code := run(t, root, "ready")
	if code == 0 || strings.TrimSpace(stderr) != "NOT_INITIALIZED" {
		t.Fatalf("got code=%d stderr=%q", code, stderr)
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
	if _, stderr, code := run(t, root, "init", input); code != 0 || stderr != "" {
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
