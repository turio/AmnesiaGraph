package graph

import "sort"

// Projection contains only the execution neighborhood needed by resume. It
// is derived output, not part of the persisted graph schema.
type Projection struct {
	Active       []Task
	Dependencies map[string][]Task
	Downstream   map[string][]Task
	Ready        []Task
	Blocked      []Task
}

// HotSubgraph derives the compact recovery neighborhood from a validated
// graph. Unrelated completed and pending tasks are intentionally omitted.
func HotSubgraph(g Graph) Projection {
	projection := Projection{
		Dependencies: make(map[string][]Task),
		Downstream:   make(map[string][]Task),
		Ready:        ReadyTasks(g),
		Blocked:      make([]Task, 0),
	}
	for _, task := range g.Tasks {
		switch task.Status {
		case Active:
			projection.Active = append(projection.Active, task)
			projection.Dependencies[task.ID] = DependenciesFor(g, task.ID)
			projection.Downstream[task.ID] = DependentsFor(g, task.ID)
		case Blocked:
			projection.Blocked = append(projection.Blocked, task)
		}
	}
	sort.SliceStable(projection.Active, func(i, j int) bool { return projection.Active[i].Order < projection.Active[j].Order })
	sort.SliceStable(projection.Blocked, func(i, j int) bool { return projection.Blocked[i].Order < projection.Blocked[j].Order })
	return projection
}
