# AmnesiaGraph — First-Pass Remediation Plan

> Repository: `turio/AmnesiaGraph`
>
> Purpose: remediate only concrete correctness and packaging gaps found in the first V1 implementation, while using the current AmnesiaGraph implementation itself as the execution-state system for this remediation.

---

## 0. Audit verdict

The first pass implemented the intended core architecture well enough to remediate rather than rebuild: repo-local Git-root gating, a tiny JSON DAG, deterministic readiness, dependency-gated transitions, verification execution, compact `resume`, tests, CI, a README, and a harness-neutral skill source all exist.

The following gaps are material and in scope:

1. **`init` can bypass execution gates.** A normalized input may currently seed `active`, `blocked`, or `done` tasks directly. That lets initialization create states that were never reached through `start`/`block`/`done`, including `done` tasks whose verification was never run.
2. **Installed-state validation allows impossible dependency state.** A task may validate as `active`, `blocked`, or `done` even while one of its dependencies is not `done`. Such a graph can release downstream work from corrupted temporal state.
3. **An empty graph is accepted.** This conflicts with the product rule that a proper executable plan is a hard prerequisite.
4. **The shipped `SKILL.md` is documentation-shaped rather than a valid installable Agent Skill.** It lacks the required YAML frontmatter (`name` and `description`). Keep one canonical harness-neutral skill source, but make it structurally valid and document small install/copy examples for Codex and Claude Code.
5. **The repo's root `AGENTS.md` is context-heavy and contradicts AmnesiaGraph's own product goal.** It is roughly 9 KB and tells agents to read the full implementation plan before changing behavior. Replace it with a short repo guide that tells agents to use `amnesia resume` when state exists and read only the source section it points to.
6. **The project has not actually dogfooded its own execution graph.** The current repository has no checked-in/runtime `.amnesiagraph/graph.json`; the first pass relied on plan checkboxes and synthetic tests. This remediation must exercise AmnesiaGraph against this real repository and real remediation work.

### Explicit non-goals

Do not add or revisit:

- MCP;
- hooks;
- graph databases;
- global or cross-repository state;
- plan hashes or plan-change reconciliation;
- discovered-work nodes;
- retry systems;
- priorities, labels, assignees, comments, history, or journals;
- multi-agent claiming/locking/coordination;
- single-active-task enforcement;
- token/context instrumentation;
- web UI/editor integration;
- Spec Kit/OpenSpec parsers;
- new persisted graph fields.

Fix the concrete defects above without turning AmnesiaGraph into a general task manager.

---

# 1. Mandatory dogfood protocol

This remediation is also the first real-world acceptance test of AmnesiaGraph.

The normalized graph for this plan is checked in at:

```text
docs/REMEDIATION_GRAPH.json
```

## 1.1 Bootstrap

Before editing product code:

```text
go test ./...
go vet ./...
go build ./cmd/amnesia
go run ./cmd/amnesia init docs/REMEDIATION_GRAPH.json
go run ./cmd/amnesia resume
```

Use `go run ./cmd/amnesia ...` during remediation so every graph operation executes the current source rather than a stale previously built binary.

The first `resume` must show `R00-T01` as ready. Start it:

```text
go run ./cmd/amnesia start R00-T01
go run ./cmd/amnesia resume
```

The second `resume` must show `R00-T01` as active and point to this plan section for full instructions.

## 1.2 Execution rule for every remediation task

From this point onward, AmnesiaGraph—not checkboxes in this document—is the authoritative execution-state ledger.

For every task:

1. Begin with a fresh `go run ./cmd/amnesia resume` invocation.
2. Use the graph's ready/dependency state rather than reconstructing task ordering from memory.
3. Start the selected ready task with `go run ./cmd/amnesia start <id>`.
4. Run `resume` again and read only the source section it names for full implementation instructions. Do not reread this entire remediation plan merely to recover state.
5. Implement the task.
6. Complete it with `go run ./cmd/amnesia done <id>` so the graph-defined verification gate runs before state changes to `done`.
7. Run `resume` again before choosing the next task.

If a normal implementation discovery is necessary to close the current planned goal, handle it inside that task. Do not create a new backlog or discovered-work node.

If AmnesiaGraph itself fails in a way that prevents its own remediation, use this document temporarily only long enough to repair the current task, then return to the graph immediately.

## 1.3 Fresh-context dogfood checkpoint

Task `R05-T01` is a deliberate recovery test.

After all of its dependencies are done:

1. start `R05-T01`;
2. hand work to a fresh agent/session/context if the harness supports that, or otherwise reset/compact the working context as far as practical;
3. the continuing agent must begin from the repository with `go run ./cmd/amnesia resume`;
4. it must identify the active remediation task, its completed dependencies, its source pointer, its verification command, and its downstream task without relying on prior conversational memory;
5. it should read only the pointed section below and continue.

This checkpoint tests the product behavior; it does not add any harness-specific runtime feature.

---

# 2. Numbered remediation execution plan

## R00 — Establish the self-hosted baseline

### R00-T01 bootstrap and initialize dogfood

Goal: prove the current implementation is healthy enough to track its own remediation before changing behavior.

Steps:

1. Run the current full test, vet, and build gates.
2. Initialize this repository from `docs/REMEDIATION_GRAPH.json` using the current CLI source.
3. Confirm `resume` initially exposes only ready/blocked execution state and not this plan's body.
4. Start `R00-T01` and run `resume` again.
5. Confirm the active output includes:
   - `R00-T01`;
   - its source pointer to this section;
   - its dependency state (`none`);
   - its verification commands;
   - its immediate downstream task(s).
6. Do not modify product behavior in this task.

Acceptance:

```text
go test ./...
go vet ./...
go build ./cmd/amnesia
```

all pass, and real repo-local state has been initialized through the CLI rather than hand-written into `.amnesiagraph/graph.json`.

Depends on: none.

---

## R01 — Close state-integrity backdoors

### R01-T01 prevent init from seeding progressed states

Goal: `init` must create a new execution run, not bypass `start`, `block`, or `done`.

Required behavior:

1. Continue allowing omitted status to normalize to `pending`.
2. Allow an explicitly supplied `pending` status.
3. Reject normalized input that contains any task initially marked `active`, `blocked`, or `done`.
4. Reject any initial blocker reason.
5. Failed `init` must continue leaving an existing installed graph unchanged.
6. Keep the persisted schema exactly as-is; do not add provenance/evidence/history fields.

Add focused tests proving:

- omitted status initializes as pending;
- explicit pending initializes successfully;
- explicit active is rejected;
- explicit blocked + reason is rejected;
- explicit done is rejected even when `verify` is empty;
- explicit done is rejected when `verify` is non-empty, proving verification cannot be bypassed through `init`;
- failed re-initialization preserves the previously installed graph byte-for-byte or semantically unchanged according to the existing store test style.

Keep the rule in deterministic CLI/domain validation code, not in the skill prompt alone.

Depends on: R00-T01.

---

### R01-T02 reject impossible installed dependency state and empty graphs

Goal: a loaded execution graph must remain reachable through legal V1 transitions.

Required validation additions:

1. Reject a graph with zero tasks.
2. For any task whose status is `active`, `blocked`, or `done`, require every `depends_on` task to be `done`.
3. Keep pending tasks free to depend on pending/active/blocked/done tasks according to the existing ready calculation.
4. Do not add reverse transitions, repair logic, or automatic state rewriting.
5. Error output must identify the inconsistent task and dependency concisely.

Add table-driven validation tests for at least:

```text
parent pending -> child active   invalid
parent pending -> child blocked  invalid
parent pending -> child done     invalid
parent active  -> child done     invalid
parent blocked -> child done     invalid
parent done    -> child active   valid
parent done    -> child blocked  valid when blocker reason exists
parent done    -> child done     valid
empty tasks                         invalid
```

The existing legal CLI transitions should remain unchanged.

Depends on: R01-T01.

---

## R02 — Make the shipped skill actually portable

### R02-T01 make canonical SKILL.md valid and document two install paths

Goal: keep one harness-neutral skill body while making it structurally usable by current Codex and Claude Code.

Required changes:

1. Keep `skills/amnesiagraph/SKILL.md` as the single canonical skill source.
2. Add minimal YAML frontmatter containing:
   - `name: amnesiagraph`;
   - a concise `description` that states what the skill does and when to use it.
3. Keep the body concise and preserve the current product workflow: proper plan prerequisite, normalize once, repo-local state, `resume`, `ready`, `start`, `block`, `done`, original-plan source pointers, no discovered-work subsystem.
4. Do **not** create duplicate maintained skill copies under harness-specific repo directories.
5. Add concise README installation examples showing how a user can copy/install the same canonical skill directory into:
   - Codex user skill storage;
   - Claude Code user skill storage.
6. Keep all core Go code and graph semantics harness-neutral.
7. Do not add hooks, MCP, harness SDKs, plugins, or an installer program.

Acceptance:

- `SKILL.md` begins with valid frontmatter and has non-empty `name` and `description`;
- the same file is the source for both documented harness installations;
- README does not imply that `skills/amnesiagraph/` is automatically discovered merely because the repository contains it;
- `go test ./...` still passes.

Depends on: R00-T01.

---

## R03 — Remove self-inflicted context bloat

### R03-T01 reduce root AGENTS.md to a small execution guide

Goal: the repository's own automatic agent instructions must reinforce AmnesiaGraph rather than consume context and tell agents to reread large documents.

Replace the current long `AGENTS.md` with a concise repo guide containing only information needed before the agent can recover through AmnesiaGraph.

It should communicate approximately these invariants:

1. AmnesiaGraph is a small standard-library Go CLI; do not expand V1 scope casually.
2. State is repo-local under `.amnesiagraph/`.
3. If execution state exists, begin or recover with the local AmnesiaGraph CLI (`go run ./cmd/amnesia resume` while developing this repo).
4. Read the full instructions for the active task only from the source pointer printed by `resume`.
5. If state is not initialized, use the relevant approved implementation/remediation plan rather than inventing work.
6. Run focused tests plus the full Go gates before considering a behavioral change complete.

Remove:

- the full repository architecture map;
- long command/reference tables;
- duplicated explanations already present in README/implementation plan;
- the instruction to read the entire `docs/IMPLEMENTATION_PLAN.md` before changing product behavior.

Do not add a separate `CLAUDE.md` or another harness-specific instruction file as part of this remediation.

Acceptance:

- `AGENTS.md` is roughly one screen rather than a mini-manual;
- a fresh agent can learn how to recover current work from it;
- full architecture remains available in the existing docs rather than being auto-injected into every Codex session;
- `go test ./...` still passes.

Depends on: R00-T01.

---

## R04 — Regress the real failure modes

### R04-T01 add end-to-end tests for the repaired temporal invariants

Goal: make the concrete first-pass defects non-regressible without expanding the product surface.

Add/extend CLI and graph tests that exercise the public behavior rather than only helper functions:

1. `init` rejects active/blocked/done seed state.
2. `init` rejects an empty graph.
3. `validate` rejects impossible progressed-child/incomplete-parent state if `.amnesiagraph/graph.json` is malformed or manually corrupted.
4. A legal chain created from all-pending init still progresses through `start` then `done` and releases the next ready task.
5. Verified `done` remains impossible through normal CLI flow unless the declared verification command succeeds.
6. Existing two-repository isolation test remains passing.

Do not add recovery/migration features for corrupted state; rejection is sufficient in V1.

Depends on: R01-T02.

---

## R05 — Real fresh-context dogfood

### R05-T01 recover this remediation from AmnesiaGraph state

Goal: demonstrate that the product actually solves the problem on its own real repository after several remediation tasks have already changed the codebase.

Before the context handoff/reset, this task must already be `active` in `.amnesiagraph/graph.json` through the normal `start` command.

The continuing agent must:

1. begin with only the repository and the instruction to continue the approved remediation;
2. run `go run ./cmd/amnesia resume` before reading this entire plan;
3. confirm `resume` identifies `R05-T01` as active;
4. confirm it shows this section's source pointer;
5. confirm dependencies `R02-T01`, `R03-T01`, and `R04-T01` are shown as done;
6. confirm the downstream final gate is identified;
7. confirm the output does not contain unrelated remediation-plan prose;
8. read this source section only after recovering it from `resume`;
9. run `go run ./cmd/amnesia validate` against the real current repo state;
10. complete this task using `go run ./cmd/amnesia done R05-T01`.

If this recovery is confusing, overly verbose, points to the wrong source, loses state, or requires reading the whole plan to understand what to do next, treat that as a failure of this task and fix the smallest underlying resume/skill/documentation issue needed to make the dogfood flow usable. Keep any such fix within this task's original goal; do not create new backlog nodes.

Depends on: R02-T01, R03-T01, R04-T01.

---

## R06 — Final quality gate

### R06-T01 complete self-hosted remediation and verify terminal state

Goal: close the remediation only after both the code and the real self-hosted execution state prove healthy.

Required final checks:

```text
go test ./...
go vet ./...
go build ./cmd/amnesia
go run ./cmd/amnesia validate
go run ./cmd/amnesia resume
```

Before marking `R06-T01` done, `resume` should show `R06-T01` active and all dependencies satisfied.

Then complete through AmnesiaGraph itself:

```text
go run ./cmd/amnesia done R06-T01
go run ./cmd/amnesia resume
```

Final `resume` acceptance:

- no active task remains;
- no remediation task is ready;
- no remediation task is blocked;
- `READY` reports `none`;
- the graph remains valid;
- the full source plan has never been copied into `.amnesiagraph/graph.json`.

Also verify:

- the canonical skill is structurally valid and the README explains Codex/Claude Code installation without adding harness logic to the engine;
- root agent instructions are compact;
- `init` cannot bypass temporal transitions;
- invalid progressed-child/incomplete-parent state is rejected;
- repo isolation and verification-gated completion tests still pass.

Do not add any unrelated cleanup or features in this final task.

Depends on: R05-T01.

---

# 3. Expected dogfood DAG

```text
R00-T01
   |
   +-------------------+-------------------+
   |                   |                   |
   v                   v                   v
R01-T01            R02-T01             R03-T01
   |
   v
R01-T02
   |
   v
R04-T01
   |                   |                   |
   +-------------------+-------------------+
                       |
                       v
                   R05-T01
                       |
                       v
                   R06-T01
```

`R05-T01` is a join over the engine-integrity, skill-portability, context-diet, and regression-test branches. This intentionally exercises multiple ready tasks and a dependency join without adding scheduling policy to AmnesiaGraph.

---

# 4. Remediation completion criteria

This remediation is complete only when:

- the current repository was initialized from `docs/REMEDIATION_GRAPH.json` through the actual CLI;
- remediation state was advanced through `start`/`done`, not by hand-editing `.amnesiagraph/graph.json`;
- `init` can no longer seed progressed states;
- empty graphs are rejected;
- loaded graphs reject progressed tasks whose dependencies are incomplete;
- the canonical skill has valid `name`/`description` frontmatter;
- README gives concise Codex and Claude Code installation paths while core behavior remains harness-neutral;
- root `AGENTS.md` is compact and points agents toward `resume` rather than a full-plan reread;
- a fresh-context agent successfully recovered `R05-T01` from this repository's real AmnesiaGraph state;
- all Go tests, vet, and build pass;
- `R06-T01` is closed by AmnesiaGraph itself and final `resume` reports no remaining work.
