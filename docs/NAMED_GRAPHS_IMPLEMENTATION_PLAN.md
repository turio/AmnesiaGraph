# AmnesiaGraph — Named Graphs / Multiple Plans Per Repo Implementation Plan

> Repository: `turio/AmnesiaGraph`
>
> Purpose: allow one Git repository to preserve several unrelated implementation-plan execution graphs at once, while keeping AmnesiaGraph's default recovery path as simple as `amnesia resume`.

---

## 0. Product goal

### 0.1 User scenario

Today AmnesiaGraph has one repo-local graph file. That works until a user interrupts one unfinished implementation plan with another unrelated plan in the same repository.

Required behavior after this change:

```text
Plan A
T1 done
T2 active
T3 pending
        |
        | user starts unrelated Plan B
        v
Plan B
B1 pending
B2 pending
```

Plan B must **not** overwrite Plan A.

The repository should instead retain both execution contexts:

```text
.amnesiagraph/
  graphs/
    plan-a.json
    plan-b.json
  current
```

Initializing Plan B makes Plan B current. A later switch back to Plan A must recover Plan A exactly where it was, including `active`, `blocked`, `done`, dependency, and verification state.

### 0.2 Default recovery behavior

The normal agent/human command remains:

```text
amnesia resume
```

Resolution rule:

1. If `current` points to an existing unfinished graph, resume it.
2. If `current` is missing, points to a missing graph, or points to a completed graph, select the most recently modified unfinished graph.
3. Persist that fallback choice into `current`.
4. If graphs exist but all are complete, return `NO_ACTIVE_GRAPH`.
5. If the repository has no AmnesiaGraph state at all, return `NOT_INITIALIZED`.

For this feature, **unfinished** means at least one task has a status other than `done`. A graph does not need to contain an `active` task to be resumable; a freshly initialized all-`pending` graph is unfinished.

“Most recent” should be derived from the graph file's modification time. Do not add timestamps, usage-history records, an index database, or session metadata merely to implement fallback. Break modification-time ties deterministically by graph name.

### 0.3 Explicit switching

Add a small graph-selection surface:

```text
amnesia list
amnesia use <name>
```

- `list` shows the named graphs in the current repository, marks the selected graph, and distinguishes `unfinished` from `complete`.
- `use <name>` selects an existing unfinished graph by writing the `current` pointer.
- `use` must not merge graphs or move tasks between graphs.

Initializing a new graph selects it automatically.

### 0.4 Keep the product boundary narrow

Do **not** turn named graphs into projects, sessions, workspaces, epics, or an orchestration layer.

Do not add:

- cross-graph dependencies;
- graph merging;
- nested graph hierarchies;
- graph stacks or suspend/resume workflows;
- graph history or journals;
- priorities, labels, owners, comments, or metadata catalogs;
- graph deletion/rename/clone commands in this change;
- global recent-project state;
- MCP or harness-specific APIs;
- multi-agent claiming/locking semantics;
- timestamps persisted inside graph JSON;
- a database or index file;
- source-plan hashes;
- a new graph schema version merely for the storage-layout change.

The existing task graph remains the unit of execution. This change only allows more than one such graph to coexist in one repository.

---

# 1. Current implementation baseline

The current code is intentionally small and should remain recognizable after this change.

## 1.1 What already works and should remain unchanged

Keep these behaviors intact:

- Git-root repo gating in `internal/repo/repo.go`;
- graph schema in `internal/graph/model.go`;
- structural and temporal validation in `internal/graph/validate.go`;
- deterministic ready calculation in `internal/graph/ready.go`;
- dependency-gated transitions in `internal/graph/transition.go`;
- hot-subgraph projection in `internal/graph/projection.go`;
- verification execution in `internal/verify/verify.go`;
- source-plan pointers rather than copied plan content;
- support for multiple active tasks **inside one graph**;
- no cross-repository memory.

A tiny helper such as `graph.IsComplete(g Graph) bool` is allowed because graph selection needs a deterministic completion predicate. Do not otherwise enlarge the graph model.

## 1.2 Current storage limitation

Current persistence is hard-coded to:

```text
<repo>/.amnesiagraph/graph.json
```

`store.Save` replaces that one file, so a valid second `init` replaces the previous execution context.

This feature should replace that single-slot storage assumption while preserving the existing graph JSON content.

---

# 2. Fixed storage contract

## 2.1 New layout

Use:

```text
<repo>/.amnesiagraph/
  graphs/
    <name>.json
  current
```

Examples:

```text
.amnesiagraph/
  graphs/
    auth-refresh.json
    renderer-v2.json
    database-migration.json
  current
```

`current` is a tiny UTF-8 text file containing only the selected graph name plus a newline, for example:

```text
renderer-v2
```

Do not put graph selection into a global/home-directory file.

## 2.2 Graph names

Graph names are storage keys, not task metadata.

Use a deliberately simple portable name rule:

```text
^[a-z0-9][a-z0-9._-]*$
```

Examples:

```text
auth
renderer-v2
spec-2026.09
```

Reject names containing spaces, path separators, uppercase letters, empty names, `..` path traversal, or anything that could escape `graphs/`.

The graph name is **not** added to `graph.Graph` or every task. The filename and `current` pointer provide the selection context.

## 2.3 Named storage primitives

Refactor `internal/store` so callers work with a graph name.

Conceptually support operations equivalent to:

```text
GraphPath(root, name)
Load(root, name)
Save(root, name, graph)
List(root)
Current(root)
SetCurrent(root, name)
EnsureLayout(root)
```

Exact Go function names may differ, but keep all filesystem/path concerns in `internal/store`; do not create a new persistence framework or package tree.

`List` only needs to expose what selection requires, such as:

```text
name
modification time
```

Completion remains a graph-level derived property and should not be persisted as separate metadata.

## 2.4 Atomic writes

Continue using temporary-file + atomic replacement semantics for graph files.

Write `current` atomically as well so an interrupted selection cannot leave a partially written graph name.

Reuse the existing platform-specific `atomicRename` support. Do not introduce a new dependency.

---

# 3. One-case legacy migration

Existing repositories may already contain:

```text
.amnesiagraph/graph.json
```

Do not make existing execution state disappear after upgrading.

## 3.1 Migration rule

On the first stateful command after the new version is installed:

```text
legacy graph.json exists
AND no named graph layout has been established
        |
        v
create graphs/
move graph.json -> graphs/default.json
write current -> default
```

Preserve the existing graph bytes/state. Do not reset statuses or re-run `init` validation during migration; validate the migrated graph through the normal installed-state validator before using it.

## 3.2 Migration conflict

If both the legacy single file and named-graph state already exist, do not guess, merge, or overwrite.

Return a concise error such as:

```text
MIGRATION_CONFLICT
```

and leave all files unchanged.

Do not build a general migration subsystem. This is one compatibility bridge from the current released storage layout to the named-graph layout.

## 3.3 Dogfood the migration

The implementation agent should use the current version of AmnesiaGraph to track this plan **before modifying storage code**:

1. Normalize the numbered tasks in this document into a graph input.
2. Run the current V1 `amnesia init <normalized-graph.json>` so this feature work occupies the existing legacy `.amnesiagraph/graph.json` slot.
3. Start the first task and use `resume` normally.
4. When the migration implementation lands, the next invocation of the updated CLI must migrate that live unfinished graph to `graphs/default.json` and continue from the same execution state.

This makes the real AmnesiaGraph repository itself a migration fixture rather than relying only on synthetic tests.

---

# 4. Selection and fallback rules

## 4.1 Current graph

`current` is the explicit selection pointer.

The following actions update it:

- successful `amnesia init <name> <normalized-graph.json>`;
- successful `amnesia use <name>`;
- automatic fallback performed by `amnesia resume`.

Normal task commands operate only on the selected graph.

They must never scan all graphs for a matching task ID. Different plans may legitimately reuse IDs such as `T001`.

## 4.2 Plain `resume`

Implement one resolver specifically for recovery:

```text
read current
   |
   +-- current graph exists and unfinished --> use it
   |
   +-- missing / missing target / complete --> scan named graphs
                                             filter unfinished
                                             newest mtime first
                                             name tie-breaker
                                             select first
```

Then print the existing hot-subgraph projection for that graph.

Add one small line at the top of resume output so an agent knows which execution context it recovered:

```text
GRAPH renderer-v2
```

The remainder of `resume` should stay as compact as today.

## 4.3 No unfinished graph

If named graphs exist but every task in every graph is `done`, plain `resume` returns non-zero with:

```text
NO_ACTIVE_GRAPH
```

Do not auto-reopen a completed graph.

## 4.4 Explicit `use`

Command:

```text
amnesia use <name>
```

Rules:

1. validate graph name;
2. require the named graph to exist;
3. load and validate it;
4. require it to be unfinished;
5. atomically write `current`;
6. print:

```text
CURRENT <name>
```

If complete, return:

```text
GRAPH_COMPLETE <name>
```

No task state changes.

## 4.5 `list`

Command:

```text
amnesia list
```

Keep output compact, for example:

```text
* renderer-v2  unfinished
  auth-refresh unfinished
  old-migration complete
```

`*` marks `current` when it points to an existing graph.

Ordering:

1. current graph first when present;
2. remaining graphs by modification time newest first;
3. graph-name lexical tie-breaker for deterministic output.

Do not print task bodies, plan excerpts, timestamps, owners, or other metadata.

---

# 5. CLI contract after the change

Target command surface:

```text
amnesia init <name> <normalized-graph.json>
amnesia list
amnesia use <name>
amnesia validate
amnesia resume
amnesia ready
amnesia start <id>
amnesia block <id> <reason>
amnesia done <id>
amnesia help
amnesia version
```

## 5.1 `init`

Change initialization to require an explicit graph name:

```text
amnesia init renderer-v2 normalized-graph.json
```

Behavior:

1. repo-gate as today;
2. run the one-case legacy-layout migration if needed;
3. validate the graph name;
4. decode and normalize the input graph exactly as today;
5. run `ValidateForInit` exactly as today;
6. reject an existing graph name rather than overwriting it:

```text
GRAPH_EXISTS renderer-v2
```

7. atomically save `graphs/renderer-v2.json`;
8. atomically set `current` to `renderer-v2`;
9. print:

```text
INITIALIZED renderer-v2
```

Do not add `--force`, `--replace`, or reset behavior in this change.

The CLI is still pre-1.0; do not retain the old one-argument `init` form merely by inventing a hidden default-name policy. Persisted legacy state receives the migration path above; new initialization should use the explicit named contract.

## 5.2 Current-scoped commands

These commands operate only on the selected graph:

```text
validate
ready
start
block
done
```

Refactor the current `loadValidated(root)` path into a current-name-aware loader.

Do not let `start`, `done`, or similar commands search other graphs when a task ID is missing in the selected graph.

`resume` is the only command that automatically repairs selection by falling back to another unfinished graph.

## 5.3 Version

Bump the CLI version from `0.1.0` to `0.2.0` because the storage layout and `init` syntax change while graph JSON schema version remains `1`.

---

# 6. Graph package changes

Keep changes to `internal/graph` minimal.

Add only a completion predicate if useful:

```text
IsComplete(Graph) bool
```

Definition:

```text
true iff every task is done
```

The existing validator already rejects empty graphs, so no special empty-graph semantics are needed.

Do not add:

- graph names;
- plan IDs;
- last-used timestamps;
- parent graph IDs;
- cross-graph edges;
- storage concerns.

Readiness, projection, transitions, and verification semantics remain graph-local and unchanged.

---

# 7. Tests required before documentation cleanup

Tests should prove the user scenario, not only helper functions.

## 7.1 Store tests

Add or adapt tests covering:

- named graph save/load isolation;
- safe graph-name validation and path traversal rejection;
- atomic `current` read/write;
- named graph listing with modification times;
- legacy `graph.json` -> `graphs/default.json` migration;
- migration preserves graph task state byte-for-byte or semantically exactly;
- migration writes `current=default`;
- legacy + named-layout conflict returns `MIGRATION_CONFLICT` without mutation;
- missing state still returns `NOT_INITIALIZED`;
- existing graph name is not silently replaced.

Replace the current `TestSaveReplacesExistingGraph` expectation: replacement of one global graph is precisely the behavior this feature is removing.

## 7.2 CLI end-to-end interruption test

Create one temporary Git repository with two unrelated source-plan files.

Exercise this exact workflow:

```text
init plan-a
start A-T001
done A-T001
start A-T002

init plan-b
resume
```

Assert:

- Plan B is current;
- Plan A's `A-T002` remains active in its own graph;
- Plan B did not merge or copy Plan A tasks.

Then:

```text
use plan-a
resume
```

Assert that `A-T002` is still active with its original source/dependency/verification context.

This is the primary acceptance test for the feature.

## 7.3 Same task IDs across graphs

Initialize two graphs that both contain `T001`.

Switch between them and prove `start`, `block`, and `done` mutate only the current graph.

This protects against accidental repo-wide task lookup.

## 7.4 Default resume selection test

Cover all branches:

1. current exists + unfinished -> resume current regardless of another graph's newer mtime;
2. current missing -> choose newest unfinished graph and persist it as current;
3. current points to missing graph -> same fallback;
4. current graph complete -> choose newest unfinished graph and persist it;
5. newest graph complete -> skip it and choose the newest unfinished graph;
6. mtime tie -> deterministic graph-name tie-breaker;
7. all graphs complete -> `NO_ACTIVE_GRAPH`;
8. no graphs -> `NOT_INITIALIZED`.

Use controlled file mtimes in tests; do not add persisted recency metadata just to simplify testing.

## 7.5 Existing behavior regression

Keep the current tests for:

- repo isolation;
- outside-repo rejection;
- dependency gates;
- impossible temporal-state rejection;
- verification-gated `done`;
- blocker transitions;
- deterministic ready ordering;
- hot resume omitting unrelated plan content;
- multiple independent active tasks within one graph.

Adapt helpers to initialize/select named graphs rather than weakening those assertions.

---

# 8. Documentation and skill updates

After behavior and tests are stable, update only documentation that is now inaccurate.

## 8.1 README

Update:

- storage layout;
- command table;
- minimal workflow to use `amnesia init <name> <file>`;
- one short example showing Plan A interrupted by Plan B and later resumed;
- explanation that `amnesia resume` uses current unfinished graph first and otherwise falls back to the most recently modified unfinished graph;
- `amnesia list` / `amnesia use`.

Do not turn the README into a project-management manual.

## 8.2 `skills/amnesiagraph/SKILL.md`

Teach the agent:

1. normalize each independent implementation plan into its own named graph;
2. do **not** merge unrelated plans merely because they live in one repo;
3. choose a short deterministic lowercase graph name;
4. initialize with `amnesia init <name> <normalized-graph.json>`;
5. run plain `amnesia resume` when recovering context;
6. use `amnesia list` only when graph identity is unclear;
7. use `amnesia use <name>` to return to another unfinished plan;
8. after selection, task commands apply only to that graph;
9. repo isolation remains absolute.

Keep the canonical skill harness-neutral.

## 8.3 `AGENTS.md`

Remove the direct assumption that state exists only at `.amnesiagraph/graph.json`.

The recovery instruction should become conceptually:

```text
run amnesia resume
follow GRAPH + READ output
```

Do not make agents manually inspect the storage directory to choose a graph.

## 8.4 Examples

Update normalization example instructions so `amnesia init` includes a graph name.

Do not create dedicated Spec Kit/OpenSpec adapters.

---

# 9. Numbered execution plan

Checkbox meanings:

```text
[ ] not started
[~] in progress
[x] implemented and verified
[!] blocked
```

The implementation agent should normalize these task IDs/dependencies into AmnesiaGraph before editing code and use AmnesiaGraph itself as the execution ledger.

## MG00 — Bootstrap and baseline

### MG00-T01 — Bootstrap this plan under the current single-graph implementation

- [ ] Run `go test ./...`, `go vet ./...`, and `go build ./cmd/amnesia` before changes.
- [ ] Normalize the tasks in this document into a small graph JSON input with these IDs and source pointers back to this document.
- [ ] Using the **current** CLI syntax, initialize that graph into the legacy `.amnesiagraph/graph.json` slot.
- [ ] Start `MG00-T01` and confirm `resume` points back to this section.
- [ ] Record the exact status of the legacy graph before changing storage code so later migration can prove it was preserved.

**Depends on:** none.

**Verify:**

```text
go test ./...
go vet ./...
go build ./cmd/amnesia
```

---

## MG01 — Named storage foundation

### MG01-T01 — Implement named graph paths, names, and current pointer

- [ ] Replace the one-file `GraphPath(root)` assumption with named graph paths under `.amnesiagraph/graphs/`.
- [ ] Add portable lowercase graph-name validation.
- [ ] Add named `Load`/`Save` storage behavior without changing graph JSON schema.
- [ ] Add atomic `current` pointer read/write.
- [ ] Add graph listing with file modification time.
- [ ] Reuse existing atomic rename support and standard library only.

**Depends on:** MG00-T01.

**Verify:**

```text
go test ./internal/store/...
```

### MG01-T02 — Add one-case legacy storage migration

- [ ] Detect the current `.amnesiagraph/graph.json` layout.
- [ ] Migrate it to `.amnesiagraph/graphs/default.json` when no named layout exists.
- [ ] Set `current` to `default`.
- [ ] Reject legacy/named coexistence with `MIGRATION_CONFLICT` and no mutation.
- [ ] Confirm the live dogfood graph created in MG00 survives with identical execution state.
- [ ] Run the updated `resume` path only after the CLI layer is ready; until then validate migration through store tests and direct inspection.

**Depends on:** MG01-T01.

**Verify:**

```text
go test ./internal/store/...
```

---

## MG02 — Selection semantics

### MG02-T01 — Add graph completion predicate and current-scoped loading

- [ ] Add the minimal graph completion predicate needed for selection.
- [ ] Refactor installed-state loading so task commands load the graph named by `current` rather than a global file.
- [ ] Preserve all existing graph validation before commands use a selected graph.
- [ ] Ensure task-ID lookup never crosses graph boundaries.

**Depends on:** MG01-T02.

**Verify:**

```text
go test ./internal/graph/... ./internal/cli/...
```

### MG02-T02 — Implement `list` and `use`

- [ ] Add `amnesia list` with compact current/unfinished/complete output.
- [ ] Add `amnesia use <name>`.
- [ ] Require selected graphs to exist, validate, and be unfinished.
- [ ] Make `use` write only the current pointer; do not alter task state or graph mtime intentionally.

**Depends on:** MG02-T01.

**Verify:**

```text
go test ./internal/store/... ./internal/cli/...
```

### MG02-T03 — Implement default `resume` fallback

- [ ] Keep current unfinished graph as first choice.
- [ ] If current is unavailable or complete, select newest unfinished graph by file mtime with deterministic name tie-breaker.
- [ ] Persist automatic fallback into `current`.
- [ ] Return `NO_ACTIVE_GRAPH` when graphs exist but all are complete.
- [ ] Keep `NOT_INITIALIZED` for repositories with no AmnesiaGraph state.
- [ ] Prefix resume output with `GRAPH <name>` and otherwise preserve the current compact hot-subgraph format.

**Depends on:** MG02-T02.

**Verify:**

```text
go test ./internal/cli/...
```

---

## MG03 — Initialization and task-command integration

### MG03-T01 — Make `init` create a named graph and select it

- [ ] Change syntax to `amnesia init <name> <normalized-graph.json>`.
- [ ] Preserve current JSON decoding, normalization, source validation, and `ValidateForInit` gates.
- [ ] Reject duplicate graph names with `GRAPH_EXISTS <name>`.
- [ ] Save the new graph before writing `current`.
- [ ] Select the new graph automatically after successful initialization.
- [ ] Do not add force/replace/reset flags.
- [ ] Bump CLI version to `0.2.0`.

**Depends on:** MG02-T01.

**Verify:**

```text
go test ./internal/cli/...
go build ./cmd/amnesia
```

### MG03-T02 — Route validate/ready/start/block/done through current graph only

- [ ] Update all current installed-state commands to read/write only the selected graph file.
- [ ] Preserve existing transition and verification behavior exactly.
- [ ] Confirm a task ID present in another graph is never used as a fallback.
- [ ] Keep repo gating unchanged.

**Depends on:** MG03-T01.

**Verify:**

```text
go test ./internal/cli/... ./internal/graph/... ./internal/verify/...
```

---

## MG04 — End-to-end regression suite

### MG04-T01 — Add storage and fallback regression coverage

- [ ] Cover graph-name safety.
- [ ] Cover current-pointer atomic behavior.
- [ ] Cover named save/load isolation.
- [ ] Cover legacy migration and migration conflict.
- [ ] Cover all plain-`resume` fallback branches and mtime tie-break behavior.
- [ ] Remove/update tests whose expected behavior was global graph replacement.

**Depends on:** MG02-T03, MG03-T02.

**Verify:**

```text
go test ./internal/store/... ./internal/cli/...
```

### MG04-T02 — Add the interrupted Plan A / Plan B acceptance test

- [ ] In one temporary repo, initialize Plan A and progress it to an active middle task.
- [ ] Initialize unrelated Plan B and confirm it becomes current without modifying Plan A.
- [ ] Switch back with `use plan-a` and verify Plan A resumes at the exact prior task/state.
- [ ] Use overlapping task IDs in an additional case to prove graph isolation.
- [ ] Complete the current graph and prove plain `resume` falls back to the most recently modified unfinished graph.
- [ ] Preserve existing repo-isolation and hot-context assertions.

**Depends on:** MG04-T01.

**Verify:**

```text
go test ./...
```

---

## MG05 — Agent-facing contract

### MG05-T01 — Update README, skill, examples, and AGENTS.md

- [ ] Update README storage and command documentation.
- [ ] Add one concise interruption/switch-back example.
- [ ] Update `SKILL.md` to create separate named graphs for unrelated plans in the same repo.
- [ ] Teach plain `resume` as the default recovery operation and `list`/`use` only for explicit selection.
- [ ] Update example init commands to include graph names.
- [ ] Update `AGENTS.md` so agents no longer inspect a single hard-coded `graph.json` path.
- [ ] Keep all documentation harness-neutral and anti-bloat.

**Depends on:** MG04-T02.

**Verify:**

```text
go test ./...
go vet ./...
go build ./cmd/amnesia
```

---

## MG06 — Self-hosted final gate

### MG06-T01 — Prove migration and multi-plan recovery on the finished implementation

- [ ] Run `amnesia resume` in this repository after the legacy dogfood state has migrated and confirm it identifies the expected named/current graph.
- [ ] Confirm this implementation plan's execution state survived the storage migration rather than restarting from pending.
- [ ] In automated tests, rerun the exact Plan A interrupted by Plan B scenario.
- [ ] Confirm plain `resume` recovers current when unfinished and falls back correctly after current completes.
- [ ] Confirm `list` and `use` do not leak or merge task context.
- [ ] Confirm no global state, database, session system, cross-graph dependency model, or new task metadata was introduced.
- [ ] Run the complete quality gate.

**Depends on:** MG05-T01.

**Verify:**

```text
gofmt -w cmd internal
go test ./...
go vet ./...
go build ./cmd/amnesia
```

---

# 10. Final acceptance criteria

The change is complete only when all of the following are true:

1. Two unrelated implementation plans can coexist in one Git repo without task/state merging.
2. Initializing the second plan cannot destroy the first plan's execution state.
3. Each graph keeps the existing V1 task schema and deterministic execution rules.
4. `amnesia init <name> <file>` creates a new named graph and selects it.
5. Duplicate graph names are rejected instead of replaced.
6. `amnesia use <name>` switches to another unfinished graph without changing its task state.
7. Plain `amnesia resume` resumes current when current is unfinished.
8. Plain `resume` falls back to the most recently modified unfinished graph when current is missing or complete, then persists that selection.
9. `resume` clearly identifies the graph with one compact `GRAPH <name>` line.
10. All task commands operate only on the selected graph, even when another graph has the same task IDs.
11. Legacy `.amnesiagraph/graph.json` state migrates once to `graphs/default.json` without loss.
12. Repo isolation remains unchanged: no graph or current pointer is read from another repo or home-directory state.
13. The implementation remains standard-library Go and does not add sessions, project management, graph merging, global recency storage, or orchestration features.
14. `go test ./...`, `go vet ./...`, and `go build ./cmd/amnesia` all pass.

The intended mental model after this change is still small:

```text
Git repository
   |
   +-- graph: plan-a ---- task execution context
   +-- graph: plan-b ---- task execution context
   +-- graph: plan-c ---- task execution context
   |
   +-- current ---------- which unfinished context resume should prefer
```

AmnesiaGraph remains a durable execution context graph for coding agents; this change merely prevents a new unrelated plan in the same repository from erasing the execution context of an unfinished one.
