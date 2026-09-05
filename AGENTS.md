# AmnesiaGraph agent guide

AmnesiaGraph is a small standard-library Go CLI for repo-local execution state. Keep V1 deliberately narrow: do not add an issue tracker, graph database, orchestrator, harness integration, global state, or discovered-work system.

## Recover work

- State belongs only in `.amnesiagraph/graph.json` under this Git repository.
- If that file exists, begin or recover with `go run ./cmd/amnesia resume`.
- Use the ready/dependency state printed by `resume`; start work with `go run ./cmd/amnesia start <id>` and complete it with `go run ./cmd/amnesia done <id>`.
- For an active task, read the full instructions only from the `READ` source pointer printed by `resume`. Do not reread an entire plan to reconstruct execution state.
- If state is not initialized, use the approved implementation or remediation plan and its normalized graph input; do not invent a parallel backlog.

## Invariants

- Every stateful command is repo-gated; there is no fallback or cross-repository memory.
- The original plan remains authoritative. Graph state stores short task data and source pointers, not copied plan prose.
- Readiness and transitions are deterministic; independent tasks may be active simultaneously.
- `done` must pass all task verification commands before changing state.

## Validate changes

Run focused tests for the changed package, then:

```text
go test ./...
go vet ./...
go build ./cmd/amnesia
```

Use `README.md`, `docs/IMPLEMENTATION_PLAN.md`, and the source files for deeper architecture and product details. Do not add a separate `CLAUDE.md` or other harness-specific instruction file for this project.
