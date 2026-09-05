# AmnesiaGraph — Agent-First Implementation Plan

> Repository: `turio/AmnesiaGraph`
>
> Purpose: build a deliberately tiny, repo-local temporal execution graph for coding agents. AmnesiaGraph does **not** replace implementation plans. It preserves the minimum durable execution state needed to continue a proper plan correctly after context compaction, handoff, or a fresh session.

---

## 0. Product contract and anti-bloat boundary

### 0.1 Core problem

Long coding tasks often already have a good implementation plan, but an agent can lose temporal execution state after context compaction:

- what has actually been completed;
- what is currently being worked on;
- what remains;
- what dependencies must be satisfied first;
- what is blocked;
- what verification is required before a task can be closed.

AmnesiaGraph externalizes only that state.

### 0.2 Core design

```text
existing authoritative plan(s)
Spec Kit / OpenSpec / Markdown / other
                |
                | agent-assisted normalization once
                v
      repo-local execution DAG
                |
                | tiny deterministic CLI
                v
         compact hot subgraph
                |
                v
            coding agent
```

The source plan remains authoritative for implementation intent and full instructions.

The execution graph is authoritative only for execution state, dependencies, readiness, blockers, source pointers, and verification gates.

### 0.3 V1 product laws

1. **A proper implementation plan is a hard prerequisite.** AmnesiaGraph is not a planning framework.
2. **Never copy the full source plan into the execution graph.** Store only a short title and a source pointer back to the original plan.
3. **Never inject the full graph by default.** `resume` exposes only the current execution neighborhood and immediately useful state.
4. **All state is repository-local.** AmnesiaGraph never searches, recalls, or merges execution state from another repository.
5. **Dependency and readiness rules are deterministic code, not prompt instructions.**
6. **If executable verification is defined for a task, the task cannot become `done` until verification succeeds.**
7. **Do not add product features merely because other task trackers have them.**

### 0.4 Explicit V1 non-goals

Do **not** implement:

- a graph database;
- Neo4j, Cypher, RDF, embeddings, semantic search, or vector memory;
- an issue tracker;
- a project-management system;
- priorities, labels, assignees, comments, story points, epics, or milestones;
- a general journal or conversation-memory store;
- verbose evidence/test-log storage;
- agent orchestration, scheduling, worktree management, or messaging;
- GitHub Issues synchronization;
- a web UI or editor extension;
- MCP;
- harness-specific core logic;
- automatic discovered-work nodes;
- retry counters or autonomous repair loops;
- global state or cross-repository memory;
- enforcement of a single active task;
- multi-agent claiming/locking/coordination semantics;
- token-budget measurement or context-window instrumentation;
- source-plan hash tracking or plan-change reconciliation.

If later demand justifies one of these, evaluate it as a separate extension. Do not pre-build extension points for hypothetical features.

---

# 1. Fixed V1 architecture

## 1.1 Technology

Use **Go** and the standard library unless a dependency is proven necessary.

Reasons:

- one small cross-platform binary;
- fast startup for frequent agent calls;
- simple `go install` distribution;
- no runtime environment to activate;
- standard library is sufficient for JSON, process execution, filesystem operations, and tests.

Do not add Cobra, a graph library, a database, or a persistence framework for V1.

Suggested module:

```text
github.com/turio/AmnesiaGraph
```

Binary:

```text
amnesia
```

## 1.2 Repository-local storage

Every command that reads or mutates execution state must first resolve the current Git repository root with:

```text
git rev-parse --show-toplevel
```

State lives only at:

```text
<git-root>/.amnesiagraph/graph.json
```

There is no home-directory state file, global database, daemon state, recent-repository list, or fallback search.

If the current working directory is not inside a Git repository, exit non-zero with a short error such as:

```text
NOT_IN_REPO
```

If the repository has not been initialized for AmnesiaGraph, exit non-zero with:

```text
NOT_INITIALIZED
```

A globally installed `amnesia` binary is allowed. Execution state itself is never global.

## 1.3 One intentionally small graph file

Use one repo-local JSON file rather than a database or separate topology/state stores.

Conceptual V1 shape:

```json
{
  "version": 1,
  "tasks": [
    {
      "id": "T001",
      "order": 1,
      "title": "Define public request contract",
      "source": "docs/IMPLEMENTATION_PLAN.md#p01-t01",
      "depends_on": [],
      "verify": ["go test ./..."],
      "status": "pending",
      "blocker": null
    }
  ]
}
```

Required task fields:

```text
id
order
title
source
depends_on[]
verify[]
status
blocker
```

Allowed status values:

```text
pending
active
blocked
done
```

`blocker` is `null` unless status is `blocked`, where it is a short non-empty string.

Do not add description, notes, history, owner, priority, labels, timestamps, token counts, or copied plan content.

## 1.4 Source pointers

Every task must point back to the original implementation-plan material using a repo-relative source reference such as:

```text
docs/IMPLEMENTATION_PLAN.md#compiler
specs/auth/tasks.md#2-1-refresh-token-support
openspec/changes/oauth/tasks.md#implementation
```

A project may have many source files. Do not assume one `IMPLEMENTATION_PLAN.md`.

Source paths must be repository-relative and must resolve inside the current Git root. Reject absolute paths and `..` escapes.

The graph stores **where the instructions are**, not the instructions themselves.

---

# 2. Plan prerequisite and normalization workflow

## Goal

Allow AmnesiaGraph to sit underneath plans from Spec Kit, OpenSpec, custom Markdown, or other planning systems without requiring those systems to adopt an AmnesiaGraph-specific format.

## 2.1 Hard prerequisite

The skill must not begin implementation if no proper implementation plan exists.

A usable plan must contain enough information for the calling agent to identify discrete executable tasks and their intended ordering/dependencies.

AmnesiaGraph does not generate product requirements, architecture, or implementation strategy from scratch.

## 2.2 Stable task IDs

If the source plan already contains usable unique IDs such as:

```text
T037
P04-T07
AUTH-03
2.4
```

preserve them.

If the source plan lacks usable stable IDs, the calling agent must normalize it into deterministic sequential IDs:

```text
T001
T002
T003
...
```

The source plan does not need to be rewritten merely to add these IDs.

## 2.3 Agent-assisted normalization

Semantic decomposition belongs in the skill/calling agent, not in the deterministic CLI.

The skill should:

1. Locate the implementation plan source file or files supplied by the user/project.
2. Confirm that a real implementation plan exists before implementation begins.
3. Read enough of the source plan to identify executable tasks.
4. Preserve existing stable task IDs when present.
5. Assign deterministic sequential IDs when IDs are absent.
6. Convert implementation ordering into explicit `depends_on` edges.
7. Preserve source order in the numeric `order` field.
8. Attach a short title only.
9. Attach a repo-relative `source` pointer to the relevant original plan section.
10. Capture executable verification commands from the plan when they are clearly specified or directly implied by the task's documented verification step.
11. Keep `verify` empty when there is no executable verification command rather than inventing a large verification framework.
12. Produce a normalized graph input file.
13. Call the CLI to validate and install it.

If the source plan is too coarse to provide meaningful execution checkpoints, the skill/calling agent must first decompose the existing plan into executable numbered steps for the graph. This is a normalization step, not a new product-planning framework.

## 2.4 No discovered-work subsystem

Do not add `discover`, dynamic backlog growth, or automatic child-task creation.

Work discovered while completing a task should normally be treated as work required to satisfy the original planned goal.

AmnesiaGraph tracks execution of the plan; it does not turn normal implementation discoveries into a second planning system.

---

# 3. Deterministic graph validation

## Goal

Make invalid execution state impossible to initialize without adding a graph framework.

## 3.1 Validation rules

`amnesia init <normalized-graph.json>` and `amnesia validate` must reject graphs where:

1. `version` is unsupported;
2. task IDs are empty or duplicated;
3. `order` values are missing or duplicated;
4. task titles are empty;
5. source references are empty;
6. source paths are absolute or escape the repository root;
7. the referenced source file does not exist;
8. a task depends on an unknown task ID;
9. a task depends on itself;
10. the dependency graph contains a cycle;
11. status is outside the allowed enum;
12. a blocked task has no blocker reason;
13. a non-blocked task has a blocker reason;
14. `verify` contains empty commands.

Do not perform semantic correctness checks that require an LLM.

## 3.2 Cycle detection

Implement cycle detection directly with a small standard graph algorithm such as Kahn's algorithm or DFS.

Do not add NetworkX or another graph dependency.

Example invalid graph:

```text
T1 -> T2
T2 -> T3
T3 -> T1
```

Expected result:

```text
INVALID_GRAPH cycle: T1 -> T2 -> T3 -> T1
```

Exact wording may differ, but errors should stay concise and actionable.

## 3.3 Atomic initialization

Validation must complete before the installed graph is replaced.

Write mutations through a temporary file in `.amnesiagraph/` and atomically rename it over `graph.json` only after serialization succeeds.

Do not add a database transaction layer.

---

# 4. Deterministic execution-state engine

## Goal

Move the smallest high-value execution decisions out of model memory and into deterministic code.

## 4.1 Ready calculation

A task is `ready` exactly when:

```text
status == pending
AND
all depends_on tasks have status == done
```

Return ready tasks sorted by ascending `order`.

No LLM decides readiness.

## 4.2 Start transition

Command:

```text
amnesia start <id>
```

Allowed transitions:

```text
pending -> active
blocked -> active
```

Before transition, all dependencies must be `done`.

If dependencies are incomplete, reject the transition and list only the blocking dependency IDs/statuses.

Starting a previously blocked task clears its blocker reason.

Do **not** enforce that only one task may be active. The harness/model/user may choose how many independent ready tasks to activate.

## 4.3 Block transition

Command:

```text
amnesia block <id> <reason>
```

Allowed transition:

```text
active -> blocked
```

Reason must be short and non-empty.

Do not implement retries, retry counts, automatic unblock logic, or failure policy.

## 4.4 Done transition

Command:

```text
amnesia done <id>
```

Allowed transition:

```text
active -> done
```

Dependencies must still be `done`.

If `verify` is empty, the transition may complete immediately.

If `verify` contains commands, run them before changing status. Only a complete successful verification sequence may transition the task to `done`.

No other state transition is valid in V1.

---

# 5. Verification-gated completion

## Goal

Prevent an agent from marking a task complete merely because it believes the implementation is finished when the plan already defines an executable check.

## 5.1 Command behavior

For a task such as:

```json
{
  "id": "T017",
  "verify": [
    "go test ./internal/graph/...",
    "go test ./internal/cli/..."
  ]
}
```

running:

```text
amnesia done T017
```

must:

1. verify that `T017` is currently active;
2. verify that all dependencies are done;
3. execute each verification command from the Git repository root in listed order;
4. stop on the first non-zero result;
5. leave task state unchanged on failure;
6. transition to `done` only if every command succeeds;
7. print a compact result.

Success example:

```text
VERIFY T017 2/2 PASS
DONE T017
```

Failure example:

```text
VERIFY T017 FAIL: go test ./internal/graph/...
STATE_UNCHANGED active
```

Do not copy stdout/stderr into `graph.json`.

Normal command output may stream to the terminal so the caller can diagnose a failure; persisted state remains compact.

## 5.2 Cross-platform shell execution

Implement one small shell adapter:

- POSIX: execute verification strings through `/bin/sh -c`;
- Windows: execute through `cmd.exe /C`.

Run commands with working directory set to the resolved Git root.

Do not build a workflow language, command parser, sandbox, or remote executor.

---

# 6. Compact context projection and `resume`

## Goal

Make one command sufficient to recover useful execution state after compaction or a fresh session without reloading the entire implementation plan or graph.

## 6.1 Primary recovery command

Command:

```text
amnesia resume
```

This is the primary agent-facing re-entry command.

The skill should instruct agents to run it:

- at the beginning of work in an initialized repository;
- after context compaction;
- after a fresh coding-agent session;
- after handoff;
- whenever execution state is uncertain.

No harness-specific hook is required.

## 6.2 Hot-subgraph projection

`resume` must not dump every task.

Default output should contain only execution information that is immediately relevant:

1. currently active task(s);
2. each active task's short title;
3. the original plan `source` pointer for each active task;
4. immediate dependency statuses for active task(s);
5. verification command(s) for active task(s), if any;
6. immediate downstream task IDs/titles that depend directly on active task(s);
7. currently ready tasks, in deterministic order;
8. currently blocked tasks and their short blocker reasons.

If no task is active, foreground the ready tasks.

The output must explicitly remind the agent that full implementation instructions remain in the original plan source, for example:

```text
ACTIVE
T017  Implement deterministic ready calculation
READ   docs/IMPLEMENTATION_PLAN.md#t017  (full instructions)
DEPS   T012 done, T015 done
VERIFY go test ./internal/graph/...
NEXT   T018 CLI ready command

READY
T021  Add source-reference validation

BLOCKED
T025  missing local test fixture
```

Do not include full task descriptions or source-plan excerpts.

## 6.3 Supporting inspection command

Command:

```text
amnesia ready
```

Print only tasks satisfying the deterministic ready rule, ordered by `order`.

Keep output compact:

```text
T021  Add source-reference validation
T024  Add repo-isolation tests
```

No ranking, priority, scheduling, or recommendation layer.

---

# 7. CLI surface

## Goal

Keep the complete V1 interface small enough for an agent or human to learn in one glance.

Required commands:

```text
amnesia init <normalized-graph.json>
amnesia validate
amnesia resume
amnesia ready
amnesia start <id>
amnesia block <id> <reason>
amnesia done <id>
```

Global behavior:

- every command except help/version resolves the current Git root first;
- every installed-state command reads only `<git-root>/.amnesiagraph/graph.json`;
- success output is concise;
- errors are non-zero and concise;
- do not add interactive menus;
- do not add a daemon;
- do not add network calls;
- do not add configuration files unless implementation proves one is necessary.

Optional standard informational flags are allowed:

```text
amnesia help
amnesia version
```

Do not add commands for features outside the V1 product contract.

---

# 8. Portable agent skill

## Goal

Provide one small, harness-neutral procedural skill that makes the CLI usable by Codex, Claude Code, and other shell-capable coding agents without moving harness-specific behavior into the product core.

## 8.1 Skill location

Add:

```text
skills/amnesiagraph/SKILL.md
```

Keep the skill short. It should teach workflow, not restate the full product documentation.

## 8.2 Required skill behavior

The skill must tell the calling agent:

1. AmnesiaGraph requires an existing proper implementation plan.
2. The source plan may come from Spec Kit, OpenSpec, custom Markdown, or another planning system.
3. If the plan already has stable task IDs, preserve them.
4. If task IDs are absent, create deterministic numbered IDs during normalization.
5. If the plan is not decomposed enough for execution tracking, decompose its existing planned work into numbered executable steps before initialization.
6. Keep the original plan file(s) authoritative for full implementation details.
7. Store only short task titles, dependencies, source pointers, verification commands, and execution state in AmnesiaGraph.
8. Run `amnesia resume` before continuing work when context may be stale or missing.
9. Before implementing an active task, read the full instructions at its `source` pointer in the original plan.
10. Use `amnesia ready` rather than reasoning from memory about which dependency-gated task is available.
11. Use `amnesia start <id>` when beginning a task.
12. Use `amnesia block <id> <reason>` when an active planned task cannot currently progress.
13. Use `amnesia done <id>` when implementation is complete; let the CLI enforce defined verification gates.
14. Do not turn ordinary implementation discoveries into a new AmnesiaGraph backlog. They remain part of completing the original planned goal.
15. Never use state from another repository.

The skill must not require Codex `update_plan`, Claude `TodoWrite`, hooks, MCP, or any specific harness API.

Harnesses remain free to use their own transient planning/todo features independently.

## 8.3 Normalized input shape in the skill

Include one compact example the agent can follow when constructing the input passed to `amnesia init`:

```json
{
  "version": 1,
  "tasks": [
    {
      "id": "T001",
      "order": 1,
      "title": "Create graph validator",
      "source": "docs/IMPLEMENTATION_PLAN.md#t001",
      "depends_on": [],
      "verify": ["go test ./internal/graph/..."]
    },
    {
      "id": "T002",
      "order": 2,
      "title": "Implement ready calculation",
      "source": "docs/IMPLEMENTATION_PLAN.md#t002",
      "depends_on": ["T001"],
      "verify": ["go test ./internal/graph/..."]
    }
  ]
}
```

`init` adds initial execution-state fields; the normalization input should not need to duplicate default `pending` state.

---

# 9. Internal package structure

Keep the codebase small and obvious.

Target structure:

```text
AmnesiaGraph/
├── cmd/
│   └── amnesia/
│       └── main.go
├── internal/
│   ├── repo/
│   │   └── repo.go
│   ├── graph/
│   │   ├── model.go
│   │   ├── validate.go
│   │   ├── ready.go
│   │   └── transition.go
│   ├── store/
│   │   └── store.go
│   ├── verify/
│   │   └── verify.go
│   └── cli/
│       └── cli.go
├── skills/
│   └── amnesiagraph/
│       └── SKILL.md
├── docs/
│   └── IMPLEMENTATION_PLAN.md
├── README.md
├── go.mod
└── .github/
    └── workflows/
        └── test.yml
```

Do not create empty abstraction layers merely to match this tree. If two packages remain trivial during implementation, merge them rather than preserving architecture for its own sake.

---

# 10. Numbered execution plan

The following tasks are the implementation order for AmnesiaGraph itself.

Checkbox meanings:

```text
[ ] not started
[~] in progress
[x] implemented and verified
[!] blocked
```

Agents implementing this repository should update these checkboxes as work completes.

## P00 — Foundation

### P00-T01 — Initialize minimal Go project

- [ ] Create `go.mod` for `github.com/turio/AmnesiaGraph`.
- [ ] Create `cmd/amnesia/main.go`.
- [ ] Implement only enough command dispatch to support `help` and `version` placeholders while later tasks fill behavior.
- [ ] Use the Go standard library only.
- [ ] Add `.gitignore` entries for local build artifacts only; do not automatically ignore `.amnesiagraph/` because projects may choose whether to commit execution state.

**Acceptance:**

```text
go test ./...
go build ./cmd/amnesia
```

both succeed.

**Depends on:** none.

---

### P00-T02 — Add minimal CI

- [ ] Add one GitHub Actions workflow for supported Go setup, `go test ./...`, and `go vet ./...`.
- [ ] Do not add release automation, coverage services, linters, code-quality SaaS, or matrix complexity in V1.

**Acceptance:** workflow syntax is valid and local commands pass.

**Depends on:** P00-T01.

---

## P01 — Repo gate and persistence

### P01-T01 — Resolve repository root deterministically

- [ ] Implement repository-root lookup via `git rev-parse --show-toplevel`.
- [ ] Normalize the returned path.
- [ ] Return a typed/simple internal error when outside Git.
- [ ] Do not implement home-directory fallback, recently-used repository lookup, or parent-project discovery beyond Git's own root result.

**Tests:**

- command from repo root resolves root;
- command from nested directory resolves the same root;
- command outside a Git repo returns `NOT_IN_REPO` behavior.

**Depends on:** P00-T01.

---

### P01-T02 — Implement repo-local graph store

- [ ] Define `.amnesiagraph/graph.json` as the only persistent V1 state path.
- [ ] Implement load/save helpers rooted only at the resolved Git root.
- [ ] Create `.amnesiagraph/` during successful initialization.
- [ ] Save through temp-file + atomic rename.
- [ ] Do not implement file locking, database transactions, daemon state, or global state.

**Tests:**

- two temporary Git repos maintain completely independent graph files;
- operations in repo A never read repo B's state;
- uninitialized repo reports `NOT_INITIALIZED`.

**Depends on:** P01-T01.

---

## P02 — Minimal graph model and validator

### P02-T01 — Define graph/task structs

- [ ] Add graph version.
- [ ] Add task fields: `id`, `order`, `title`, `source`, `depends_on`, `verify`, `status`, `blocker`.
- [ ] Define exact allowed states: pending, active, blocked, done.
- [ ] Keep JSON stable and human-readable.
- [ ] Do not add fields outside the fixed V1 model.

**Depends on:** P01-T02.

---

### P02-T02 — Implement structural validation

- [ ] Validate version.
- [ ] Validate unique non-empty IDs.
- [ ] Validate unique orders.
- [ ] Validate non-empty titles and sources.
- [ ] Validate dependency references and reject self-dependencies.
- [ ] Validate status/blocker consistency.
- [ ] Validate non-empty verification command strings.
- [ ] Validate source path is relative, stays inside current repo, and points to an existing file.
- [ ] Support many different source files in one graph.

**Depends on:** P02-T01.

---

### P02-T03 — Implement dependency cycle detection

- [ ] Detect any cycle in `depends_on` relationships with a small in-house standard algorithm.
- [ ] Return an actionable cycle error.
- [ ] Add branched-DAG and cycle test fixtures.
- [ ] Do not add a graph library.

**Depends on:** P02-T02.

---

### P02-T04 — Implement `init` and `validate`

- [ ] `amnesia init <normalized-graph.json>` reads an agent-produced normalized graph input.
- [ ] Initialization sets omitted task statuses to `pending` and blockers to `null`.
- [ ] Validate completely before installing repo state.
- [ ] Never copy source-plan text into installed state.
- [ ] `amnesia validate` validates the installed graph using the same validator.
- [ ] Failed init must leave any existing installed graph unchanged.

**Acceptance:** malformed graphs fail; valid graphs install and round-trip.

**Depends on:** P02-T03.

---

## P03 — Deterministic execution semantics

### P03-T01 — Implement deterministic ready calculation

- [ ] A task is ready iff it is pending and every dependency is done.
- [ ] Sort ready tasks by `order` ascending.
- [ ] Add tests for roots, chains, forks, joins, blocked tasks, and multiple simultaneously ready tasks.
- [ ] Do not rank or prioritize ready tasks beyond original plan order.

**Depends on:** P02-T04.

---

### P03-T02 — Implement `start`

- [ ] Add `amnesia start <id>`.
- [ ] Allow `pending -> active` when all dependencies are done.
- [ ] Allow `blocked -> active` when all dependencies are done and clear blocker reason.
- [ ] Reject unknown IDs and invalid source states.
- [ ] Reject start when any dependency is not done and print only the relevant dependency blockers.
- [ ] Do not enforce one-active-task semantics.

**Tests:** explicitly prove that two independent ready tasks may both be active because AmnesiaGraph does not own harness/model scheduling policy.

**Depends on:** P03-T01.

---

### P03-T03 — Implement `block`

- [ ] Add `amnesia block <id> <reason>`.
- [ ] Allow only `active -> blocked`.
- [ ] Require non-empty reason.
- [ ] Persist only the short reason.
- [ ] Do not implement retry policy or automatic resolution.

**Depends on:** P03-T02.

---

## P04 — Verification-gated completion

### P04-T01 — Implement verification runner

- [ ] Execute verification strings from the Git root.
- [ ] POSIX uses `/bin/sh -c`.
- [ ] Windows uses `cmd.exe /C`.
- [ ] Execute in declared order.
- [ ] Stop on first non-zero exit.
- [ ] Return compact structured internal success/failure information.
- [ ] Do not persist stdout/stderr in the graph.

**Tests:** passing command, failing command, ordered multiple commands, execution working directory.

**Depends on:** P03-T03.

---

### P04-T02 — Implement `done`

- [ ] Add `amnesia done <id>`.
- [ ] Allow only `active -> done`.
- [ ] Re-check dependency completion.
- [ ] If `verify` is empty, complete immediately.
- [ ] If `verify` is non-empty, run every verification command before changing state.
- [ ] Failed verification leaves status `active`.
- [ ] Successful verification changes status to `done` atomically.
- [ ] Print concise pass/fail output.

**Tests:** no-verification completion, successful verified completion, failed verification state preservation, invalid transition rejection.

**Depends on:** P04-T01.

---

## P05 — Minimal context projection

### P05-T01 — Implement `ready` CLI output

- [ ] Add `amnesia ready`.
- [ ] Print only deterministic ready tasks.
- [ ] Include task ID and short title.
- [ ] Preserve `order` sorting.
- [ ] Do not print full descriptions, the full graph, or source-plan content.

**Depends on:** P03-T01.

---

### P05-T02 — Implement hot-subgraph projection

- [ ] Build a projection function that derives only immediately relevant state from the installed graph.
- [ ] Include active tasks.
- [ ] Include each active task's immediate dependencies and statuses.
- [ ] Include direct downstream dependents of active tasks.
- [ ] Include deterministic ready tasks.
- [ ] Include explicitly blocked tasks with short reasons.
- [ ] Include source pointers and verification commands for active tasks.
- [ ] Do not include unrelated completed/pending tasks by default.

**Depends on:** P04-T02, P05-T01.

---

### P05-T03 — Implement `resume`

- [ ] Add `amnesia resume` as the primary recovery command.
- [ ] Render the hot-subgraph projection compactly.
- [ ] Support zero, one, or multiple active tasks without imposing scheduling policy.
- [ ] If there is no active task, foreground currently ready work.
- [ ] Explicitly label each active task's `source` as the location of the **full original plan instructions**.
- [ ] Do not read and print the source-plan body.

**Tests:**

- chain;
- fork/join DAG;
- no active tasks;
- multiple active tasks;
- blocked task;
- unrelated graph nodes excluded;
- a distinctive sentence present in the source Markdown never appears in `resume` output.

**Depends on:** P05-T02.

---

## P06 — Portable agent skill

### P06-T01 — Write harness-neutral `SKILL.md`

- [ ] Create `skills/amnesiagraph/SKILL.md`.
- [ ] Keep it concise.
- [ ] Require a proper implementation plan before use.
- [ ] Explain normalization from arbitrary plan sources.
- [ ] Preserve existing IDs or generate deterministic IDs when absent.
- [ ] Tell the agent to decompose coarse existing planned work into executable numbered steps when necessary.
- [ ] Tell the agent to keep full instructions in the original plan and use graph `source` pointers to retrieve them.
- [ ] Teach `resume`, `ready`, `start`, `block`, and `done` workflow.
- [ ] Explicitly tell the agent not to use state from another repository.
- [ ] Do not mention a required Codex/Claude-native todo tool.
- [ ] Do not require hooks, MCP, or harness-specific APIs.
- [ ] Do not create discovered-work nodes.

**Depends on:** P05-T03.

---

### P06-T02 — Add normalization example fixtures

Add a small set of documentation/test fixtures demonstrating that the same graph contract can represent:

- [ ] a custom Markdown plan that already has task IDs;
- [ ] a custom Markdown plan without IDs, normalized to `T001...`;
- [ ] a plan whose tasks point to multiple source files;
- [ ] a Spec Kit-style task source;
- [ ] an OpenSpec-style task source.

These are examples of input shape, not dedicated parsers.

Do not implement Spec Kit/OpenSpec adapters in V1.

**Depends on:** P06-T01.

---

## P07 — End-to-end quality tests

### P07-T01 — Repo-isolation end-to-end test

- [ ] Create two temporary Git repositories.
- [ ] Initialize independent AmnesiaGraph DAGs.
- [ ] Start/complete different tasks in each.
- [ ] Verify `resume`, `ready`, and state mutations always use only the repo containing the command's working directory.
- [ ] Verify execution outside Git cannot access either repo's state.

**Depends on:** P05-T03.

---

### P07-T02 — Compaction-resume simulation

Simulate the intended product behavior without involving an actual LLM:

1. Initialize a branched plan.
2. Complete early tasks.
3. Start a middle task.
4. Drop all in-process state by invoking a fresh CLI process.
5. Run `amnesia resume` from the repo.
6. Verify it reconstructs the correct active task(s), dependencies, source pointer(s), next/ready work, blockers, and verification commands solely from repo-local state.

- [ ] Assert no source-plan body is included.
- [ ] Assert unrelated graph nodes are not included.

**Depends on:** P07-T01.

---

### P07-T03 — Invalid-transition matrix

Add table-driven tests covering every allowed and rejected state transition:

```text
pending -> active   allowed if deps done
blocked -> active   allowed if deps done
active  -> blocked  allowed with reason
active  -> done     allowed if verification passes
```

Everything else is rejected.

- [ ] Include dependency-incomplete cases.
- [ ] Include verification-failure cases.
- [ ] Include multiple-active-task case and verify it is not rejected merely because another task is active.

**Depends on:** P04-T02.

---

### P07-T04 — Full V1 test gate

- [ ] `go test ./...`
- [ ] `go vet ./...`
- [ ] `go build ./cmd/amnesia`
- [ ] run a manual CLI smoke test in a temporary Git repo using a fixture plan.

**Depends on:** P07-T02, P07-T03.

---

## P08 — Public-facing documentation

### P08-T01 — Write concise README

The README should explain the product in one screen before deeper usage details:

> AmnesiaGraph is a tiny repo-local execution graph for long-running coding agents. Your existing implementation plan stays authoritative; AmnesiaGraph remembers only enough execution state to resume correctly after context loss.

Document:

- [ ] why it exists;
- [ ] what it does not do;
- [ ] installation with `go install`;
- [ ] proper-plan prerequisite;
- [ ] normalization concept;
- [ ] the seven V1 commands;
- [ ] repo-local state path;
- [ ] source pointers back to full original plan instructions;
- [ ] one minimal end-to-end example;
- [ ] how to install/use the generic skill with a shell-capable coding agent.

Do not turn the README into general agent-framework documentation.

**Depends on:** P07-T04, P06-T02.

---

### P08-T02 — Document architecture and non-goals

- [ ] Keep this implementation plan as the detailed architecture record.
- [ ] Add only minimal extra docs if README clarity requires them.
- [ ] Explicitly state that AmnesiaGraph is not Beads/Ergo-style issue management, not an orchestrator, not agent memory, not a graph database, and not a replacement for Spec Kit/OpenSpec/custom plans.

**Depends on:** P08-T01.

---

# 11. Required behavioral examples

These examples are acceptance contracts, not optional UX sketches.

## 11.1 Dependency gate

Given:

```text
T001 pending
T002 pending depends_on=[T001]
```

then:

```text
amnesia start T002
```

must fail without mutation and identify `T001` as incomplete.

After `T001` is done, `T002` becomes ready deterministically.

## 11.2 Multiple ready/active tasks

Given:

```text
T001 done
T002 pending depends_on=[T001]
T003 pending depends_on=[T001]
```

both T002 and T003 are ready in `order` sequence.

AmnesiaGraph must not add a policy that prevents a harness/model/user from activating both.

## 11.3 Verification gate

Given active task T004 with:

```text
verify=["go test ./internal/graph/..."]
```

`amnesia done T004` may change state only if that command succeeds.

## 11.4 Repo gate

If repo A has active `A-7` and repo B has active `B-3`, then:

```text
cd repo-b
amnesia resume
```

may expose B-3 but must have no mechanism that recalls or searches for A-7.

## 11.5 Source-plan pointer

Given:

```text
source=specs/payments/tasks.md#refund-flow
```

`resume` should tell the agent to read that location for full instructions.

It must not persist or print the entire refund-flow plan section.

---

# 12. V1 completion gate

AmnesiaGraph V1 is complete only when all of the following are true:

- [ ] proper-plan prerequisite and normalization workflow are documented in the generic skill;
- [ ] arbitrary plan sources can be represented through the same minimal normalized graph contract;
- [ ] missing source task IDs can be supplied by agent normalization without modifying the source plan;
- [ ] all execution state is repo-gated under `.amnesiagraph/`;
- [ ] repo A can never be read as fallback state while operating in repo B;
- [ ] graph initialization rejects invalid references and cycles;
- [ ] `ready` is deterministic and dependency-based;
- [ ] `start` enforces dependency completion but does not enforce one-active-task policy;
- [ ] `block` persists only a short blocker reason;
- [ ] `done` enforces defined executable verification;
- [ ] failed verification leaves the task active;
- [ ] `resume` reconstructs the hot execution neighborhood from a fresh process;
- [ ] `resume` points agents to the original plan for full instructions rather than copying plan content;
- [ ] unrelated graph nodes are omitted from default resume output;
- [ ] no global/cross-repo memory exists;
- [ ] no MCP, graph database, issue tracker, orchestrator, dynamic discovered-work system, or harness-specific core is present;
- [ ] `go test ./...`, `go vet ./...`, and `go build ./cmd/amnesia` pass;
- [ ] README explains the product and usage concisely.

---

# 13. Implementation-agent protocol for this repository

Until AmnesiaGraph is implemented enough to track itself, agents executing this plan should use this document's numbered tasks and checkboxes as the durable ledger.

1. Read Section 0 before implementing anything.
2. Implement tasks in dependency order; do not create temporary architecture intended to be replaced later.
3. Before beginning a task, read that task's full section here.
4. Mark `[~]` when actively working on a task.
5. Mark `[x]` only after its acceptance/tests pass.
6. Mark `[!]` only when genuinely blocked and add one concise blocker note next to the task.
7. Keep work discovered during implementation inside the scope of closing the original planned task/goal; do not invent a second backlog.
8. Do not introduce any feature listed in V1 non-goals because it appears useful in Beads, Ergo, Conductor, `td`, Peter, or another agent framework.
9. When a design choice is not specified, choose the smallest implementation that satisfies the behavioral contracts and tests in this plan.
10. Once P05 and P06 are complete enough for safe self-use, the implementing agent may initialize AmnesiaGraph from this plan and use `amnesia resume` for the remaining phases, but self-hosting is not a requirement for V1 completion.
