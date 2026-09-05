package graph

import "sort"

// ReadyTasks returns pending tasks whose dependencies are all done, in plan
// order. It does not impose a one-active-task policy.
func ReadyTasks(g Graph) []Task {
	ready := make([]Task, 0)
	for _, task := range g.Tasks {
		if task.Status != Pending || !dependenciesDone(g, task) {
			continue
		}
		ready = append(ready, task)
	}
	sort.SliceStable(ready, func(i, j int) bool { return ready[i].Order < ready[j].Order })
	return ready
}

// DependenciesFor returns the immediate dependencies of taskID in graph
// order. The graph must already have passed structural validation.
func DependenciesFor(g Graph, taskID string) []Task {
	task, ok := taskByID(g, taskID)
	if !ok {
		return nil
	}
	result := make([]Task, 0, len(task.DependsOn))
	for _, dependencyID := range task.DependsOn {
		if dependency, exists := taskByID(g, dependencyID); exists {
			result = append(result, dependency)
		}
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].Order < result[j].Order })
	return result
}

// DependentsFor returns tasks that directly depend on taskID in plan order.
func DependentsFor(g Graph, taskID string) []Task {
	result := make([]Task, 0)
	for _, task := range g.Tasks {
		for _, dependencyID := range task.DependsOn {
			if dependencyID == taskID {
				result = append(result, task)
				break
			}
		}
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].Order < result[j].Order })
	return result
}

func dependenciesDone(g Graph, task Task) bool {
	for _, dependencyID := range task.DependsOn {
		dependency, ok := taskByID(g, dependencyID)
		if !ok || dependency.Status != Done {
			return false
		}
	}
	return true
}

func taskByID(g Graph, id string) (Task, bool) {
	for _, task := range g.Tasks {
		if task.ID == id {
			return task, true
		}
	}
	return Task{}, false
}
