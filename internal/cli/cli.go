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

const version = "0.1.0"

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
		if len(args) != 2 {
			return usageError(errOut, "amnesia init <normalized-graph.json>")
		}
		return initGraph(root, cwd, args[1], out, errOut)
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
		current, err := loadValidated(root)
		if err != nil {
			return printError(errOut, err)
		}
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

func initGraph(root, cwd, inputPath string, out, errOut io.Writer) int {
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
	if err := graph.Validate(current, root); err != nil {
		return printError(errOut, err)
	}
	if err := store.Save(root, current); err != nil {
		return printError(errOut, err)
	}
	fmt.Fprintln(out, "INITIALIZED .amnesiagraph/graph.json")
	return 0
}

func loadValidated(root string) (graph.Graph, error) {
	current, err := store.Load(root)
	if err != nil {
		return graph.Graph{}, err
	}
	if err := graph.Validate(current, root); err != nil {
		return graph.Graph{}, err
	}
	return current, nil
}

func startTask(root, id string, out, errOut io.Writer) int {
	current, err := loadValidated(root)
	if err != nil {
		return printError(errOut, err)
	}
	if err := graph.Start(&current, id); err != nil {
		return printError(errOut, err)
	}
	if err := store.Save(root, current); err != nil {
		return printError(errOut, err)
	}
	fmt.Fprintf(out, "START %s\n", id)
	return 0
}

func blockTask(root, id, reason string, out, errOut io.Writer) int {
	current, err := loadValidated(root)
	if err != nil {
		return printError(errOut, err)
	}
	if err := graph.Block(&current, id, reason); err != nil {
		return printError(errOut, err)
	}
	if err := store.Save(root, current); err != nil {
		return printError(errOut, err)
	}
	fmt.Fprintf(out, "BLOCK %s\n", id)
	return 0
}

func doneTask(root, id string, out, errOut io.Writer) int {
	current, err := loadValidated(root)
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
	if err := store.Save(root, current); err != nil {
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
	fmt.Fprintln(out, "  init <normalized-graph.json>  install graph state")
	fmt.Fprintln(out, "  validate                     validate installed state")
	fmt.Fprintln(out, "  resume                       show the hot execution neighborhood")
	fmt.Fprintln(out, "  ready                        list dependency-ready tasks")
	fmt.Fprintln(out, "  start <id>                   begin a ready task")
	fmt.Fprintln(out, "  block <id> <reason>          block an active task")
	fmt.Fprintln(out, "  done <id>                    verify and complete an active task")
	fmt.Fprintln(out, "  version                      print the CLI version")
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
