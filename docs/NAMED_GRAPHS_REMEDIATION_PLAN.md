# AmnesiaGraph — Named-Graph Selection Integrity Remediation Plan

> Repository: `turio/AmnesiaGraph`
>
> Purpose: fix one correctness gap in the named-graphs implementation without expanding product scope.

---

## 0. Audit verdict

The named-graphs implementation is substantially correct and should be remediated, not redesigned.

It correctly implements:

- multiple independent graph files under `.amnesiagraph/graphs/`;
- an atomic repo-local `current` pointer;
- explicit `init <name>`, `list`, and `use <name>` commands;
- current-scoped `validate`, `ready`, `start`, `block`, and `done`;
- preservation of interrupted Plan A when Plan B is initialized;
- isolation when different graphs reuse the same task IDs;
- fallback from a missing/completed current graph to the newest unfinished graph;
- one-case migration from legacy `.amnesiagraph/graph.json` to `graphs/default.json`;
- unchanged graph schema, readiness, transition, projection, verification, and repo-gating behavior.

### Material defect

`resolveResume` currently treats too many failures as if the selected graph were merely absent.

The intended fallback conditions are narrow:

```text
current pointer missing
OR current target graph missing
OR current graph complete
        ↓
scan unfinished named graphs and choose fallback
```

But the current implementation also falls through to fallback when:

- reading `.amnesiagraph/current` fails for a reason other than the file being absent;
- `current` contains an invalid graph name;
- loading the selected graph fails because its JSON is malformed or the file is otherwise unreadable.

That can silently switch the agent to a different implementation plan when the selected execution context is corrupted. For AmnesiaGraph, silent context switching is worse than a hard failure because the tool's core job is durable execution-context correctness.

### Required rule

`amnesia resume` may automatically change selection only for these three cases:

1. no `current` pointer exists;
2. `current` names a valid graph that no longer exists;
3. the selected graph exists, validates successfully, and is complete.

All other failures in the selected context must surface as errors and must leave `current` unchanged.

### Explicit non-goals

Do not add:

- repair commands;
- graph history or backups;
- checksums;
- file locking;
- global state;
- graph deletion/rename/clone;
- new graph metadata;
- a new graph schema version;
- fallback policy for malformed non-current graphs beyond what is already implemented;
- automatic corruption recovery.

This remediation is only about preventing silent plan switching when the selected context itself is broken.

---

# 1. Implementation constraints

Keep the existing storage layout and CLI surface unchanged:

```text
.amnesiagraph/
  graphs/
    <name>.json
  current
```

No new user-facing command is required.

Prefer a small typed/sentinel distinction in `internal/store` so the CLI can tell the difference between:

```text
selected graph does not exist   -> allowed resume fallback
selected graph cannot be read   -> hard error
```

For example, introduce an `ErrGraphNotFound` sentinel wrapped by `Load`, or an equivalently small `errors.Is`-compatible mechanism. Do not parse error strings in `resolveResume`.

---

# 2. Numbered execution plan

Checkbox meanings:

```text
[ ] not started
[~] in progress
[x] implemented and verified
[!] blocked
```

## NGR00 — Bootstrap and reproduce

### NGR00-T01 — Run the current baseline and encode the failing behavior first

- [ ] Run:

```text
go test ./...
go vet ./...
go build ./cmd/amnesia
```

- [ ] Add focused CLI tests that create two valid unfinished named graphs and select one of them.
- [ ] Corrupt the selected graph JSON while leaving the other graph valid.
- [ ] Assert the current implementation demonstrates the bug: `resume` would otherwise skip the selected graph and recover the other one.
- [ ] Add a separate test for malformed/invalid contents in `.amnesiagraph/current`.
- [ ] Keep the existing missing-current and missing-target fallback tests intact; those cases must continue to succeed.

**Acceptance:** the new corruption tests fail against the pre-fix resolver for the expected reason, while the existing test suite still identifies no unrelated regression.

---

## NGR01 — Make selected-context errors explicit

### NGR01-T01 — Distinguish missing graph from other load failures

- [ ] In `internal/store`, make a missing named graph detectable with `errors.Is` or an equally small typed error.
- [ ] Preserve concise user-facing output such as:

```text
GRAPH_NOT_FOUND <name>
```

- [ ] Do not change malformed-JSON errors, general `LOAD_FAILED` errors, or graph validation errors into the missing-graph condition.
- [ ] Add focused store tests proving the distinction.

**Acceptance:** callers can deterministically branch on "target absent" without string matching and without conflating malformed/unreadable graphs with absence.

### NGR01-T02 — Restrict `resume` fallback to the three allowed cases

Refactor `resolveResume` so current selection is handled explicitly:

```text
read current
  |
  +-- ErrNoCurrent ------------------------------> fallback scan
  |
  +-- any other current-read error --------------> return error
  |
  v
validate current graph name
  |
  +-- invalid -----------------------------------> return error
  |
  v
load selected graph
  |
  +-- graph not found ----------------------------> fallback scan
  +-- any other load error -----------------------> return error
  |
  v
validate selected graph
  |
  +-- invalid ------------------------------------> return error
  |
  v
complete?
  |
  +-- yes ----------------------------------------> fallback scan
  +-- no -----------------------------------------> resume selected graph
```

- [ ] Do not change fallback scanning/ranking for non-current graphs.
- [ ] Do not mutate `current` on any hard error.
- [ ] Preserve `GRAPH <name>` and the existing hot-subgraph output on success.

**Acceptance:** a damaged selected graph never causes silent recovery into another plan.

---

## NGR02 — Regression coverage

### NGR02-T01 — Lock the selection-integrity contract with end-to-end tests

Add/retain tests for all of these cases:

1. current exists + unfinished + valid -> resume it;
2. current missing -> fallback to newest unfinished and persist selection;
3. current points to missing target -> fallback and persist selection;
4. current graph complete -> fallback and persist selection;
5. current pointer contains invalid graph name -> error, no fallback, pointer unchanged;
6. current graph contains malformed JSON -> error, no fallback, pointer unchanged;
7. current graph fails installed-state validation -> error, no fallback;
8. another valid unfinished graph existing beside the corrupted current graph must not change cases 5–7 into successful fallback;
9. all graphs complete -> `NO_ACTIVE_GRAPH`;
10. no graphs -> `NOT_INITIALIZED`.

Where practical, assert the exact selected graph before and after each command.

**Acceptance:** `go test ./...` passes and directly proves that automatic context switching happens only for explicitly allowed fallback conditions.

---

## NGR03 — Dogfood and final gate

### NGR03-T01 — Complete the remediation through a separate named graph

Use this remediation itself as another same-repository execution context rather than modifying the completed `default` graph.

- [ ] Normalize this plan into a named graph such as:

```text
selection-integrity-remediation
```

- [ ] Initialize it with the current named-graph CLI.
- [ ] Use normal `resume` / `start` / `done` operations while implementing the remediation.
- [ ] Before final completion, keep at least one other graph present so the corruption regression tests prove that `resume` does not silently jump to it.
- [ ] Run the final gate:

```text
gofmt -w cmd internal
go test ./...
go vet ./...
go build ./cmd/amnesia
```

- [ ] Run `amnesia validate` against the selected remediation graph before closing its final task.

**Acceptance:** remediation completes without changing the graph schema, command surface, storage layout, or fallback ranking rules.

---

# 3. Files expected to change

Likely:

```text
internal/store/store.go
internal/store/store_test.go
internal/cli/cli.go
internal/cli/cli_test.go
```

Documentation changes are unnecessary unless implementation changes user-visible error wording. Do not rewrite README/skill text simply to restate behavior they already describe correctly.

---

# 4. Final success criteria

The implementation is complete when all of the following are true:

- an absent selected graph can still trigger the intended fallback;
- a completed selected graph can still trigger the intended fallback;
- no `current` pointer can still trigger the intended fallback;
- malformed/unreadable selected state never silently switches implementation plans;
- invalid selected graph state never silently switches implementation plans;
- `current` remains unchanged on those hard errors;
- Plan A / Plan B interruption behavior remains correct;
- same-task-ID graph isolation remains correct;
- legacy migration remains correct;
- the graph schema remains version `1`;
- `go test ./...`, `go vet ./...`, and `go build ./cmd/amnesia` pass.
