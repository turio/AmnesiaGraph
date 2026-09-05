package graph

// Version is the only graph schema supported by V1.
const Version = 1

// Status is the execution state of a planned task.
type Status string

const (
	Pending Status = "pending"
	Active  Status = "active"
	Blocked Status = "blocked"
	Done    Status = "done"
)

// Task is deliberately limited to the V1 persisted execution model.
type Task struct {
	ID        string   `json:"id"`
	Order     int      `json:"order"`
	Title     string   `json:"title"`
	Source    string   `json:"source"`
	DependsOn []string `json:"depends_on"`
	Verify    []string `json:"verify"`
	Status    Status   `json:"status"`
	Blocker   *string  `json:"blocker"`
}

// Graph is the complete repo-local persisted state.
type Graph struct {
	Version int    `json:"version"`
	Tasks   []Task `json:"tasks"`
}

// NormalizeInput applies the defaults accepted by amnesia init. It does not
// alter the source plan or infer semantic dependencies.
func NormalizeInput(g Graph) Graph {
	if g.Tasks == nil {
		g.Tasks = []Task{}
	}
	for i := range g.Tasks {
		if g.Tasks[i].DependsOn == nil {
			g.Tasks[i].DependsOn = []string{}
		}
		if g.Tasks[i].Verify == nil {
			g.Tasks[i].Verify = []string{}
		}
		if g.Tasks[i].Status == "" {
			g.Tasks[i].Status = Pending
		}
	}
	return g
}
