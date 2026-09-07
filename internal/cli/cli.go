package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/turio/AmnesiaGraph/internal/graph"
	"github.com/turio/AmnesiaGraph/internal/repo"
	"github.com/turio/AmnesiaGraph/internal/store"
	"github.com/turio/AmnesiaGraph/internal/verify"
)

const version = "0.2.0"

// Run dispatches the CLI using the process's current working directory.
func Run(args []string, out, errOut io.Writer) int {
	cwd, err := os.Getwd()
	if err != nil {
		return printError(errOut, repo.ErrNotInRepo)
	}
	return RunInDir(args, cwd, out, errOut)
}

// RunInDir is the same command surface with an explicit working directory for
// tests and embedding inside a shell-capable agent.
func RunInDir(args []string, cwd string, out, errOut io.Writer) int {
	if out == nil {
		out = io.Discard
	}
	if errOut == nil {
		errOut = io.Discard
	}
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printHelp(out)
		return 0
	}
	if args[0] == "version" || args[0] == "--version" {
		fmt.Fprintf(out, "amnesia %s\n", version)
		return 0
	}

	root, err := repo.ResolveRootFrom(cwd)
	if err != nil {
		return printError(errOut, err)
	}

	switch args[0] {
	case "init":
		if len(args) != 3 {
			return usageError(errOut, "amnesia init <name> <normalized-graph.json>")
		}
		return initGraph(root, cwd, args[1], args[2], out, errOut)
	case "list":
		if len(args) != 1 {
			return usageError(errOut, "amnesia list")
		}
		return listGraphs(root, out, errOut)
	case "use":
		if len(args) != 2 {
			return usageError(errOut, "amnesia use <name>")
		}
		return useGraph(root, args[1], out, errOut)
	case "validate":
		if len(args) != 1 {
			return usageError(errOut, "amnesia validate")
		}
		current, err := loadValidated(root)
		if err != nil {
			return printError(errOut, err)
		}
		_ = current
		fmt.Fprintln(out, "VALID")
		return 0
	case "ready":
		if len(args) != 1 {
			return usageError(errOut, "amnesia ready")
		}
		current, err := loadValidated(root)
		if err != nil {
			return printError(errOut, err)
		}
		for _, task := range graph.ReadyTasks(current) {
			fmt.Fprintf(out, "%s  %s\n", task.ID, task.Title)
		}
		return 0
	case "resume":
		if len(args) != 1 {
			return usageError(errOut, "amnesia resume")
		}
		name, current, err := resolveResume(root)
		if err != nil {
			return printError(errOut, err)
		}
		fmt.Fprintf(out, "GRAPH %s\n", name)
		printResume(out, graph.HotSubgraph(current))
		return 0
	case "start":
		if len(args) != 2 {
			return usageError(errOut, "amnesia start <id>")
		}
		return startTask(root, args[1], out, errOut)
	case "block":
		if len(args) < 3 {
			return usageError(errOut, "amnesia block <id> <reason>")
		}
		return blockTask(root, args[1], strings.Join(args[2:], " "), out, errOut)
	case "done":
		if len(args) != 2 {
			return usageError(errOut, "amnesia done <id>")
		}
		return doneTask(root, args[1], out, errOut)
	default:
		return usageError(errOut, "unknown command: "+args[0])
	}
}

func initGraph(root, cwd, name, inputPath string, out, errOut io.Writer) int {
	if err := store.ValidateGraphName(name); err != nil {
		return printError(errOut, err)
	}
	if err := store.MigrateLegacyIfNeeded(root); err != nil {
		return printError(errOut, err)
	}
	if !filepath.IsAbs(inputPath) {
		inputPath = filepath.Join(cwd, inputPath)
	}
	inputPath = filepath.Clean(inputPath)
	file, err := os.Open(inputPath)
	if err != nil {
		return printError(errOut, fmt.Errorf("INPUT_FAILED %w", err))
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var current graph.Graph
	if err := decoder.Decode(&current); err != nil {
		return printError(errOut, fmt.Errorf("INVALID_GRAPH malformed input: %w", err))
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return printError(errOut, errors.New("INVALID_GRAPH multiple JSON values"))
		}
		return printError(errOut, fmt.Errorf("INVALID_GRAPH malformed input: %w", err))
	}

	current = graph.NormalizeInput(current)
	if err := graph.ValidateForInit(current, root); err != nil {
		return printError(errOut, err)
	}
	if store.Exists(root, name) {
		return printError(errOut, fmt.Errorf("GRAPH_EXISTS %s", name))
	}
	if err := store.Save(root, name, current); err != nil {
		return printError(errOut, err)
	}
	if err := store.SetCurrent(root, name); err != nil {
		return printError(errOut, err)
	}
	fmt.Fprintf(out, "INITIALIZED %s\n", name)
	return 0
}

// loadCurrentValidated runs migration, then loads only the selected graph.
func loadCurrentValidated(root string) (string, graph.Graph, error) {
	if err := store.MigrateLegacyIfNeeded(root); err != nil {
		return "", graph.Graph{}, err
	}
	name, err := store.Current(root)
	if err != nil {
		if errors.Is(err, store.ErrNoCurrent) {
			infos, listErr := store.List(root)
			if listErr != nil {
				return "", graph.Graph{}, listErr
			}
			if len(infos) == 0 {
				return "", graph.Graph{}, store.ErrNotInitialized
			}
			return "", graph.Graph{}, fmt.Errorf("NO_CURRENT run 'amnesia resume' or 'amnesia use <name>'")
		}
		return "", graph.Graph{}, err
	}
	if err := store.ValidateGraphName(name); err != nil {
		return "", graph.Graph{}, err
	}
	current, err := store.Load(root, name)
	if err != nil {
		return "", graph.Graph{}, err
	}
	if err := graph.Validate(current, root); err != nil {
		return "", graph.Graph{}, err
	}
	return name, current, nil
}

func loadValidated(root string) (graph.Graph, error) {
	_, current, err := loadCurrentValidated(root)
	return current, err
}

// resolveResume implements plain-resume selection: current unfinished first,
// otherwise newest unfinished by mtime with name tie-breaker, persisted.
func resolveResume(root string) (string, graph.Graph, error) {
	if err := store.MigrateLegacyIfNeeded(root); err != nil {
		return "", graph.Graph{}, err
	}
	infos, err := store.List(root)
	if err != nil {
		return "", graph.Graph{}, err
	}
	if len(infos) == 0 {
		return "", graph.Graph{}, store.ErrNotInitialized
	}
	byName := make(map[string]store.GraphInfo, len(infos))
	for _, info := range infos {
		byName[info.Name] = info
	}
	// Selection is explicit: only a missing pointer, a missing target, or a
	// completed selection may fall through to the fallback scan. Any other
	// failure in the selected context is a hard error and leaves `current`
	// unchanged, so a corrupted plan can never silently switch context.
	currentName, err := store.Current(root)
	if err != nil {
		if !errors.Is(err, store.ErrNoCurrent) {
			return "", graph.Graph{}, err
		}
	} else {
		if err := store.ValidateGraphName(currentName); err != nil {
			return "", graph.Graph{}, err
		}
		current, loadErr := store.Load(root, currentName)
		if loadErr != nil {
			if !errors.Is(loadErr, store.ErrGraphNotFound) {
				return "", graph.Graph{}, loadErr
			}
		} else if validateErr := graph.Validate(current, root); validateErr != nil {
			return "", graph.Graph{}, validateErr
		} else if !graph.IsComplete(current) {
			return currentName, current, nil
		}
	}
	type candidate struct {
		name    string
		current graph.Graph
		modTime int64
		nano    int
	}
	candidates := make([]candidate, 0, len(infos))
	var firstValidComplete bool
	var firstErr error
	for _, info := range infos {
		loaded, loadErr := store.Load(root, info.Name)
		if loadErr != nil {
			if firstErr == nil {
				firstErr = loadErr
			}
			continue
		}
		if validateErr := graph.Validate(loaded, root); validateErr != nil {
			if firstErr == nil {
				firstErr = validateErr
			}
			continue
		}
		if graph.IsComplete(loaded) {
			firstValidComplete = true
			continue
		}
		candidates = append(candidates, candidate{
			name:    info.Name,
			current: loaded,
			modTime: info.ModTime.Unix(),
			nano:    info.ModTime.Nanosecond(),
		})
	}
	if len(candidates) == 0 {
		if firstValidComplete {
			return "", graph.Graph{}, fmt.Errorf("NO_ACTIVE_GRAPH")
		}
		if firstErr != nil {
			return "", graph.Graph{}, firstErr
		}
		return "", graph.Graph{}, fmt.Errorf("NO_ACTIVE_GRAPH")
	}
	best := candidates[0]
	bestMod := byName[best.name].ModTime
	for _, c := range candidates[1:] {
		mod := byName[c.name].ModTime
		if mod.After(bestMod) || (mod.Equal(bestMod) && c.name < best.name) {
			best = c
			bestMod = mod
		}
	}
	// Tie-breaker when mtimes equal is lexical ascending (deterministic).
	if err := store.SetCurrent(root, best.name); err != nil {
		return "", graph.Graph{}, err
	}
	return best.name, best.current, nil
}

func listGraphs(root string, out, errOut io.Writer) int {
	if err := store.MigrateLegacyIfNeeded(root); err != nil {
		return printError(errOut, err)
	}
	infos, err := store.List(root)
	if err != nil {
		return printError(errOut, err)
	}
	if len(infos) == 0 {
		return printError(errOut, store.ErrNotInitialized)
	}
	currentName, _ := store.Current(root)
	byName := make(map[string]store.GraphInfo, len(infos))
	for _, info := range infos {
		byName[info.Name] = info
	}
	ordered := append([]store.GraphInfo(nil), infos...)
	hasCurrent := false
	for _, info := range infos {
		if info.Name == currentName {
			hasCurrent = true
			break
		}
	}
	if hasCurrent {
		rest := make([]store.GraphInfo, 0, len(ordered))
		var first store.GraphInfo
		for _, info := range ordered {
			if info.Name == currentName {
				first = info
			} else {
				rest = append(rest, info)
			}
		}
		sortGraphInfos(rest)
		ordered = append([]store.GraphInfo{first}, rest...)
	} else {
		sortGraphInfos(ordered)
	}
	for _, info := range ordered {
		status := "unfinished"
		if loaded, loadErr := store.Load(root, info.Name); loadErr == nil {
			if validateErr := graph.Validate(loaded, root); validateErr == nil {
				if graph.IsComplete(loaded) {
					status = "complete"
				}
			} else {
				status = "invalid"
			}
		} else {
			status = "invalid"
		}
		marker := " "
		if info.Name == currentName && hasCurrent {
			marker = "*"
		}
		fmt.Fprintf(out, "%s %s  %s\n", marker, info.Name, status)
	}
	return 0
}

func sortGraphInfos(infos []store.GraphInfo) {
	for i := 1; i < len(infos); i++ {
		for j := i; j > 0; j-- {
			a, b := infos[j-1], infos[j]
			swap := false
			if b.ModTime.After(a.ModTime) {
				swap = true
			} else if b.ModTime.Equal(a.ModTime) && b.Name < a.Name {
				swap = true
			}
			if !swap {
				break
			}
			infos[j-1], infos[j] = infos[j], infos[j-1]
		}
	}
}

func useGraph(root, name string, out, errOut io.Writer) int {
	if err := store.ValidateGraphName(name); err != nil {
		return printError(errOut, err)
	}
	if err := store.MigrateLegacyIfNeeded(root); err != nil {
		return printError(errOut, err)
	}
	current, err := store.Load(root, name)
	if err != nil {
		return printError(errOut, err)
	}
	if err := graph.Validate(current, root); err != nil {
		return printError(errOut, err)
	}
	if graph.IsComplete(current) {
		return printError(errOut, fmt.Errorf("GRAPH_COMPLETE %s", name))
	}
	if err := store.SetCurrent(root, name); err != nil {
		return printError(errOut, err)
	}
	fmt.Fprintf(out, "CURRENT %s\n", name)
	return 0
}

func startTask(root, id string, out, errOut io.Writer) int {
	name, current, err := loadCurrentValidated(root)
	if err != nil {
		return printError(errOut, err)
	}
	if err := graph.Start(&current, id); err != nil {
		return printError(errOut, err)
	}
	if err := store.Save(root, name, current); err != nil {
		return printError(errOut, err)
	}
	fmt.Fprintf(out, "START %s\n", id)
	return 0
}

func blockTask(root, id, reason string, out, errOut io.Writer) int {
	name, current, err := loadCurrentValidated(root)
	if err != nil {
		return printError(errOut, err)
	}
	if err := graph.Block(&current, id, reason); err != nil {
		return printError(errOut, err)
	}
	if err := store.Save(root, name, current); err != nil {
		return printError(errOut, err)
	}
	fmt.Fprintf(out, "BLOCK %s\n", id)
	return 0
}

func doneTask(root, id string, out, errOut io.Writer) int {
	name, current, err := loadCurrentValidated(root)
	if err != nil {
		return printError(errOut, err)
	}
	if err := graph.CanComplete(current, id); err != nil {
		return printError(errOut, err)
	}
	task, found := findTask(current, id)
	if !found {
		return printError(errOut, fmt.Errorf("TASK_NOT_FOUND %s", id))
	}
	if len(task.Verify) > 0 {
		result := verify.Run(root, task.Verify, out, errOut)
		if !result.Success() {
			fmt.Fprintf(errOut, "VERIFY %s FAIL: %s\nSTATE_UNCHANGED active\n", id, result.FailedCommand)
			return 1
		}
		fmt.Fprintf(out, "VERIFY %s %d/%d PASS\n", id, result.Passed, result.Total)
	}
	if err := graph.MarkDone(&current, id); err != nil {
		return printError(errOut, err)
	}
	if err := store.Save(root, name, current); err != nil {
		return printError(errOut, err)
	}
	fmt.Fprintf(out, "DONE %s\n", id)
	return 0
}

func findTask(current graph.Graph, id string) (graph.Task, bool) {
	for _, task := range current.Tasks {
		if task.ID == id {
			return task, true
		}
	}
	return graph.Task{}, false
}

func printResume(out io.Writer, projection graph.Projection) {
	if len(projection.Active) > 0 {
		fmt.Fprintln(out, "ACTIVE")
		for index, task := range projection.Active {
			fmt.Fprintf(out, "%s  %s\n", task.ID, task.Title)
			fmt.Fprintf(out, "READ   %s  (full original plan instructions)\n", task.Source)
			dependencies := projection.Dependencies[task.ID]
			if len(dependencies) == 0 {
				fmt.Fprintln(out, "DEPS   none")
			} else {
				parts := make([]string, 0, len(dependencies))
				for _, dependency := range dependencies {
					parts = append(parts, fmt.Sprintf("%s %s", dependency.ID, dependency.Status))
				}
				fmt.Fprintf(out, "DEPS   %s\n", strings.Join(parts, ", "))
			}
			for _, command := range task.Verify {
				fmt.Fprintf(out, "VERIFY %s\n", command)
			}
			next := projection.Downstream[task.ID]
			if len(next) == 0 {
				fmt.Fprintln(out, "NEXT   none")
			} else {
				parts := make([]string, 0, len(next))
				for _, dependent := range next {
					parts = append(parts, dependent.ID+"  "+dependent.Title)
				}
				fmt.Fprintf(out, "NEXT   %s\n", strings.Join(parts, "; "))
			}
			if index < len(projection.Active)-1 {
				fmt.Fprintln(out)
			}
		}
		fmt.Fprintln(out)
	}

	fmt.Fprintln(out, "READY")
	if len(projection.Ready) == 0 {
		fmt.Fprintln(out, "none")
	} else {
		for _, task := range projection.Ready {
			fmt.Fprintf(out, "%s  %s\n", task.ID, task.Title)
		}
	}

	if len(projection.Blocked) > 0 {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "BLOCKED")
		for _, task := range projection.Blocked {
			reason := ""
			if task.Blocker != nil {
				reason = *task.Blocker
			}
			if reason == "" {
				fmt.Fprintf(out, "%s  %s\n", task.ID, task.Title)
			} else {
				fmt.Fprintf(out, "%s  %s — %s\n", task.ID, task.Title, reason)
			}
		}
	}
}

func printHelp(out io.Writer) {
	fmt.Fprintln(out, "amnesia — repo-local execution graph")
	fmt.Fprintln(out, "usage: amnesia <command>")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "commands:")
	fmt.Fprintln(out, "  init <name> <normalized-graph.json>  install a named graph and select it")
	fmt.Fprintln(out, "  list                                 list named graphs")
	fmt.Fprintln(out, "  use <name>                           select an unfinished named graph")
	fmt.Fprintln(out, "  validate                             validate the selected graph")
	fmt.Fprintln(out, "  resume                               show the hot execution neighborhood")
	fmt.Fprintln(out, "  ready                                list dependency-ready tasks")
	fmt.Fprintln(out, "  start <id>                           begin a ready task")
	fmt.Fprintln(out, "  block <id> <reason>                  block an active task")
	fmt.Fprintln(out, "  done <id>                            verify and complete an active task")
	fmt.Fprintln(out, "  version                              print the CLI version")
}

func usageError(errOut io.Writer, message string) int {
	fmt.Fprintf(errOut, "USAGE %s\n", message)
	return 2
}

func printError(errOut io.Writer, err error) int {
	message := strings.TrimSpace(strings.ReplaceAll(err.Error(), "\n", " "))
	fmt.Fprintln(errOut, message)
	return 1
}
