package graph

import (
	"fmt"
	"os"
	pathpkg "path"
	"path/filepath"
	"sort"
	"strings"
)

// InvalidError identifies structural graph input that cannot be installed or
// used as execution state.
type InvalidError struct {
	Message string
}

func (e *InvalidError) Error() string {
	return "INVALID_GRAPH " + e.Message
}

func invalidf(format string, args ...any) error {
	return &InvalidError{Message: fmt.Sprintf(format, args...)}
}

// Validate checks all deterministic V1 graph invariants against repoRoot.
func Validate(g Graph, repoRoot string) error {
	if g.Version != Version {
		return invalidf("unsupported version: %d", g.Version)
	}
	if len(g.Tasks) == 0 {
		return invalidf("graph must contain at least one task")
	}

	root, err := filepath.Abs(filepath.Clean(repoRoot))
	if err != nil {
		return invalidf("invalid repository root")
	}
	if resolved, resolveErr := filepath.EvalSymlinks(root); resolveErr == nil {
		root = filepath.Clean(resolved)
	}

	byID := make(map[string]Task, len(g.Tasks))
	orders := make(map[int]string, len(g.Tasks))
	for _, task := range g.Tasks {
		if strings.TrimSpace(task.ID) == "" {
			return invalidf("task id is empty")
		}
		if _, exists := byID[task.ID]; exists {
			return invalidf("duplicate task id: %s", task.ID)
		}
		byID[task.ID] = task

		if task.Order <= 0 {
			return invalidf("task %s has missing or invalid order", task.ID)
		}
		if previous, exists := orders[task.Order]; exists {
			return invalidf("duplicate order %d for %s and %s", task.Order, previous, task.ID)
		}
		orders[task.Order] = task.ID

		if strings.TrimSpace(task.Title) == "" {
			return invalidf("task %s has empty title", task.ID)
		}
		if strings.TrimSpace(task.Source) == "" {
			return invalidf("task %s has empty source", task.ID)
		}
		if err := validateSource(task.Source, root); err != nil {
			return invalidf("task %s: %s", task.ID, err)
		}
		if !isAllowedStatus(task.Status) {
			return invalidf("task %s has invalid status: %q", task.ID, task.Status)
		}
		if task.Status == Blocked {
			if task.Blocker == nil || strings.TrimSpace(*task.Blocker) == "" {
				return invalidf("blocked task %s has no blocker reason", task.ID)
			}
		} else if task.Blocker != nil {
			return invalidf("non-blocked task %s has a blocker reason", task.ID)
		}
		for index, command := range task.Verify {
			if strings.TrimSpace(command) == "" {
				return invalidf("task %s has empty verify command at index %d", task.ID, index)
			}
		}
	}

	for _, task := range g.Tasks {
		for _, dependencyID := range task.DependsOn {
			if _, exists := byID[dependencyID]; !exists {
				return invalidf("task %s depends on unknown task %s", task.ID, dependencyID)
			}
			if dependencyID == task.ID {
				return invalidf("task %s depends on itself", task.ID)
			}
		}
	}
	for _, task := range g.Tasks {
		if task.Status == Pending {
			continue
		}
		for _, dependencyID := range task.DependsOn {
			dependency := byID[dependencyID]
			if dependency.Status != Done {
				return invalidf("task %s has incomplete dependency %s (%s)", task.ID, dependency.ID, dependency.Status)
			}
		}
	}

	if err := detectCycle(g.Tasks, byID); err != nil {
		return err
	}
	return nil
}

// ValidateForInit validates a normalized graph and enforces the rule that a
// new execution run cannot bypass runtime transitions or verification.
func ValidateForInit(g Graph, repoRoot string) error {
	if err := Validate(g, repoRoot); err != nil {
		return err
	}
	for _, task := range g.Tasks {
		if task.Status != Pending {
			return invalidf("task %s: init requires pending status, got %s", task.ID, task.Status)
		}
		if task.Blocker != nil {
			return invalidf("task %s: init cannot include a blocker reason", task.ID)
		}
	}
	return nil
}

func isAllowedStatus(status Status) bool {
	switch status {
	case Pending, Active, Blocked, Done:
		return true
	default:
		return false
	}
}

func validateSource(source, root string) error {
	pathPart := strings.TrimSpace(source)
	if fragmentIndex := strings.IndexByte(pathPart, '#'); fragmentIndex >= 0 {
		pathPart = pathPart[:fragmentIndex]
	}
	pathPart = strings.TrimSpace(pathPart)
	if pathPart == "" {
		return fmt.Errorf("source path is empty")
	}
	if isAbsoluteReference(pathPart) {
		return fmt.Errorf("source path must be relative")
	}

	relative := filepath.FromSlash(pathPart)
	candidate, err := filepath.Abs(filepath.Join(root, relative))
	if err != nil {
		return fmt.Errorf("source path is invalid")
	}
	candidate = filepath.Clean(candidate)
	if !within(root, candidate) {
		return fmt.Errorf("source path escapes repository: %s", pathPart)
	}
	info, err := os.Stat(candidate)
	if err != nil {
		return fmt.Errorf("source file does not exist: %s", pathPart)
	}
	if info.IsDir() {
		return fmt.Errorf("source is not a file: %s", pathPart)
	}
	if resolved, resolveErr := filepath.EvalSymlinks(candidate); resolveErr == nil && !within(root, resolved) {
		return fmt.Errorf("source path escapes repository: %s", pathPart)
	}
	return nil
}

func isAbsoluteReference(pathPart string) bool {
	converted := filepath.FromSlash(pathPart)
	if pathpkg.IsAbs(pathPart) || filepath.IsAbs(converted) || filepath.VolumeName(converted) != "" {
		return true
	}
	if strings.HasPrefix(pathPart, "/") || strings.HasPrefix(pathPart, `\`) {
		return true
	}
	return len(pathPart) >= 2 && pathPart[1] == ':' &&
		((pathPart[0] >= 'a' && pathPart[0] <= 'z') || (pathPart[0] >= 'A' && pathPart[0] <= 'Z'))
}

func within(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil || filepath.IsAbs(relative) {
		return false
	}
	if relative == "." {
		return true
	}
	separator := string(filepath.Separator)
	return relative != ".." && !strings.HasPrefix(relative, ".."+separator)
}

func detectCycle(tasks []Task, byID map[string]Task) error {
	ordered := append([]Task(nil), tasks...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Order < ordered[j].Order })
	state := make(map[string]uint8, len(tasks))
	stack := make([]string, 0, len(tasks))

	var visit func(string) error
	visit = func(id string) error {
		switch state[id] {
		case 2:
			return nil
		case 1:
			start := 0
			for index, stackID := range stack {
				if stackID == id {
					start = index
					break
				}
			}
			cycle := append([]string{}, stack[start:]...)
			cycle = append(cycle, id)
			return invalidf("cycle: %s", strings.Join(cycle, " -> "))
		}

		state[id] = 1
		stack = append(stack, id)
		for _, dependencyID := range byID[id].DependsOn {
			if err := visit(dependencyID); err != nil {
				return err
			}
		}
		stack = stack[:len(stack)-1]
		state[id] = 2
		return nil
	}

	for _, task := range ordered {
		if err := visit(task.ID); err != nil {
			return err
		}
	}
	return nil
}
