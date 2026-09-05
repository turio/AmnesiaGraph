package graph

import (
	"fmt"
	"sort"
	"strings"
)

// Start applies pending -> active or blocked -> active after checking every
// immediate dependency.
func Start(g *Graph, id string) error {
	index, task, err := lookup(g, id)
	if err != nil {
		return err
	}
	if task.Status != Pending && task.Status != Blocked {
		return fmt.Errorf("INVALID_TRANSITION %s: %s -> active", id, task.Status)
	}
	if blockers := incompleteDependencies(*g, task); len(blockers) > 0 {
		return dependencyError(id, blockers)
	}
	g.Tasks[index].Status = Active
	g.Tasks[index].Blocker = nil
	return nil
}

// Block applies active -> blocked and stores only the supplied short reason.
func Block(g *Graph, id, reason string) error {
	index, task, err := lookup(g, id)
	if err != nil {
		return err
	}
	if task.Status != Active {
		return fmt.Errorf("INVALID_TRANSITION %s: %s -> blocked", id, task.Status)
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return fmt.Errorf("BLOCK_REASON_REQUIRED %s", id)
	}
	g.Tasks[index].Status = Blocked
	g.Tasks[index].Blocker = &reason
	return nil
}

// CanComplete validates the non-verification part of active -> done.
func CanComplete(g Graph, id string) error {
	_, task, err := lookup(&g, id)
	if err != nil {
		return err
	}
	if task.Status != Active {
		return fmt.Errorf("INVALID_TRANSITION %s: %s -> done", id, task.Status)
	}
	if blockers := incompleteDependencies(g, task); len(blockers) > 0 {
		return dependencyError(id, blockers)
	}
	return nil
}

// MarkDone applies active -> done after CanComplete and verification have
// succeeded.
func MarkDone(g *Graph, id string) error {
	if err := CanComplete(*g, id); err != nil {
		return err
	}
	index, _, _ := lookup(g, id)
	g.Tasks[index].Status = Done
	g.Tasks[index].Blocker = nil
	return nil
}

func lookup(g *Graph, id string) (int, Task, error) {
	for index, task := range g.Tasks {
		if task.ID == id {
			return index, task, nil
		}
	}
	return -1, Task{}, fmt.Errorf("TASK_NOT_FOUND %s", id)
}

func incompleteDependencies(g Graph, task Task) []Task {
	blockers := make([]Task, 0)
	for _, dependencyID := range task.DependsOn {
		dependency, ok := taskByID(g, dependencyID)
		if !ok || dependency.Status != Done {
			if ok {
				blockers = append(blockers, dependency)
			}
		}
	}
	sort.SliceStable(blockers, func(i, j int) bool { return blockers[i].Order < blockers[j].Order })
	return blockers
}

func dependencyError(id string, blockers []Task) error {
	parts := make([]string, 0, len(blockers))
	for _, blocker := range blockers {
		parts = append(parts, fmt.Sprintf("%s(%s)", blocker.ID, blocker.Status))
	}
	return fmt.Errorf("DEPENDENCIES_INCOMPLETE %s: %s", id, strings.Join(parts, ", "))
}
