# Repository guide for coding agents

AmnesiaGraph is a small Go command-line tool for maintaining a repo-local execution graph for long-running coding agents. The original implementation plan remains authoritative for product intent; the binary stores only compact execution state and source pointers so work can resume after context loss. The runtime uses the Go standard library, Git for repository-root discovery, JSON for persistence, and the platform shell for declared verification commands.

There are no nested `AGENTS.md` files or other repository-specific instruction files. Read `docs/IMPLEMENTATION_PLAN.md` before changing product behavior; its numbered tasks and checkboxes are the durable implementation ledger.

## Repository map

| Path | Role | Important details |
|---|---|---|
| `cmd/amnesia/main.go` | Executable entry point | Passes process arguments and standard streams to `internal/cli.Run`. |
| `internal/repo/` | Git-root gate | `ResolveRoot`/`ResolveRootFrom` run `git rev-parse --show-toplevel` and return `NOT_IN_REPO` when it fails. |
| `internal/graph/` | Domain model and deterministic engine | Defines the fixed graph/task JSON model, structural validation, cycle detection, readiness, transitions, and hot-subgraph projection. |
| `internal/store/` | Repo-local persistence | Reads/writes only `<git-root>/.amnesiagraph/graph.json`; writes use a temporary file and replacement rename. |
| `internal/verify/` | Verification runner | Runs task commands from the Git root through `/bin/sh -c` or `cmd.exe /C`, stopping at the first failure. |
| `internal/cli/` | Command surface | Implements `init`, `validate`, `resume`, `ready`, `start`, `block`, `done`, `help`, and `version`, plus end-to-end CLI tests. |
| `skills/amnesiagraph/` | Harness-neutral agent procedure | Contains the portable skill and normalization examples; it is documentation/fixture content, not a parser or runtime dependency. |
| `docs/IMPLEMENTATION_PLAN.md` | Detailed architecture and execution plan | Also contains the V1 non-goals, behavioral contracts, and migration checklist. |
| `README.md` | Public usage documentation | Covers installation, normalization, commands, state location, and the minimal workflow. |
| `.github/workflows/test.yml` | CI | Runs `go test ./...` and `go vet ./...` on Ubuntu with Go 1.22.x. |
| `go.mod` | Module manifest | Module path is `github.com/turio/AmnesiaGraph`; runtime dependencies are standard-library only. |

Tests live beside the packages they exercise in `*_test.go` files. There are no generated sources, database migrations, vendored dependencies, or committed build outputs.

## Architecture and execution

The executable flow is:

```text
cmd/amnesia/main.go -> cli.Run -> repo.ResolveRootFrom
                    -> store.Load / normalized input
                    -> graph.Validate
                    -> graph transition or verify.Run
                    -> store.Save(.amnesiagraph/graph.json)
```

All stateful commands resolve the current Git root first. `init` validates a normalized JSON input against source files inside that root before installing it. `ready` derives pending tasks whose dependencies are all `done`. `start` and `block` apply the allowed state transitions. `done` checks dependencies, runs declared verification from the Git root, and writes `done` only after every command succeeds. `resume` derives a projection containing active tasks, their immediate dependency statuses, direct downstream tasks, ready tasks, and blocked tasks; it prints source pointers but never reads or prints the source-plan body.

The persisted schema is the `graph.Graph`/`graph.Task` model in `internal/graph/model.go`. V1 has only `version`, `tasks`, and the required task fields `id`, `order`, `title`, `source`, `depends_on`, `verify`, `status`, and `blocker`. JSON is indented for human inspection. The original plan is external authoritative data referenced by `source` fragments; it is not copied into the graph.

The external boundaries are the local `git` executable for root discovery and local shell commands declared by a task's `verify` list. There are no network calls, daemons, configuration files, databases, or global state paths.

## Working in the repository

### Prerequisites and command reference

Use Go 1.22 or a compatible newer Go toolchain and Git on `PATH`. Run commands from the repository unless noted otherwise.

| Task | Command | Notes |
|---|---|---|
| Format Go source | `gofmt -w cmd internal` | Rewrites Go files. |
| Unit and end-to-end tests | `go test ./...` | Tests create temporary Git repositories and temporary verification workspaces. |
| Static analysis | `go vet ./...` | Read-only validation. |
| Build the CLI | `go build ./cmd/amnesia` | Writes the local `amnesia` artifact, ignored by `.gitignore`. |
| Install the CLI | `go install github.com/turio/AmnesiaGraph/cmd/amnesia@latest` | Installs outside the repository using the Go toolchain. |

No environment variables, `.env` files, services, database setup, or code-generation step are required. The only persisted application state is created on demand by `amnesia init` in `.amnesiagraph/`; that directory is intentionally not ignored because a project may choose whether to commit execution state.

### Common development workflow

Read the relevant plan task, implement the smallest standard-library change, add or update colocated tests, run `gofmt`, then run the focused package tests followed by `go test ./...`, `go vet ./...`, and `go build ./cmd/amnesia`. Update the plan checkbox only after the acceptance criteria pass. Keep full implementation instructions in the plan and use graph source pointers rather than duplicating them.

## Testing and validation

The tests use Go's standard `testing` package. `internal/graph` covers validation, cycle paths, readiness, and transition rules. `internal/repo` covers root resolution inside, nested within, and outside Git. `internal/store` covers missing state and replacement persistence. `internal/verify` covers working directory, command order, and stop-on-first-failure behavior. `internal/cli` covers initialization, failed-init preservation, repo isolation, multiple active tasks, resume projection, blocker lifecycle, verification failure, and informational commands.

The CLI tests invoke a real local `git` executable in temporary directories. Verification tests invoke the platform shell, so commands are selected for both POSIX and Windows behavior. No network, credentials, or external service is required.

For a core behavior change, the minimum credible check is the focused package test plus `go vet ./...`; before completing a plan phase also run the full test suite and CLI build. CI adds the same full test and vet commands on Ubuntu.

## Conventions and critical invariants

- Keep the runtime standard-library-only. Do not add Cobra, a graph library, a database, or a persistence framework for V1.
- Keep the persisted model limited to the fields defined in `internal/graph/model.go`; do not add history, notes, ownership, priorities, timestamps, or copied plan text.
- Resolve state only from the current Git root. Never add home-directory fallback, recent-repository lookup, or cross-repository search.
- Validate source paths as repo-relative files before installing state. A `source` fragment identifies where the full original plan instructions live.
- Readiness is exactly `pending` plus all dependencies `done`, sorted by `order`; do not add ranking or scheduling policy.
- Do not enforce a single active task. Independent ready tasks may be active simultaneously.
- `done` must leave the task `active` when any declared verification command fails. Verification output is streamed, never persisted.
- Persist mutations through the store's temporary-file replacement path so failed validation or verification cannot replace installed state.
- Keep errors concise and non-zero for failure; preserve the command names and state markers used by the CLI tests and plan examples.

## Where to make common changes

| Goal | Start here | Also inspect/update |
|---|---|---|
| Change graph fields or validation | `internal/graph/model.go`, `internal/graph/validate.go` | `internal/store`, CLI tests, plan schema and non-goal sections |
| Change a state transition | `internal/graph/transition.go` | `internal/cli/cli.go` and transition/CLI tests |
| Change resume/ready content | `internal/graph/ready.go`, `internal/graph/projection.go`, `internal/cli/cli.go` | Projection tests, `README.md`, and plan Section 6 |
| Change persistence behavior | `internal/store/store.go` and platform atomic-rename files | Store/isolation tests and the repo-local storage rules in the plan |
| Change verification behavior | `internal/verify/verify.go` | Cross-platform verification tests and the plan's shell rules |
| Change agent procedure or examples | `skills/amnesiagraph/SKILL.md` and its `examples/` | `README.md` and P06 checklist items |
| Change public CLI usage | `internal/cli/cli.go` | CLI tests, `README.md`, and plan Section 7 |
