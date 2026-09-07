# AmnesiaGraph

**A durable execution context graph for coding agents.**

Coding agents can lose track of execution state when context is compacted, a session ends, or work is handed off. AmnesiaGraph keeps the minimum useful state outside the model context so the agent can resume correctly without rereading an entire implementation plan.

It stores a small repo-local graph of:

- task IDs and dependencies;
- `pending`, `active`, `blocked`, and `done` state;
- deterministic ready work;
- short blocker reasons;
- verification commands required before completion;
- pointers back to the original plan for full instructions.

`amnesia resume` reconstructs only the current execution neighborhood: what is active, what it depends on, what is ready or blocked, what comes next, how completion is verified, and where to read the full task instructions. The original Spec Kit, OpenSpec, Markdown, or other implementation plan remains authoritative; AmnesiaGraph does not copy the whole plan into agent context.

## How it differs

- **Codex `update_plan` / Claude todo tools** — lightweight working todo lists; AmnesiaGraph adds durable repo-local dependency, blocker, readiness, and verification state across context loss.
- **Ergo** — a lightweight agent backlog/task graph; AmnesiaGraph is a smaller execution-state overlay beneath an existing authoritative plan rather than the plan itself.
- **Beads** — a broader issue/dependency and multi-agent work-management system; AmnesiaGraph deliberately keeps only the execution context needed to resume coding work.
- **Spec Kit / OpenSpec** — create and organize implementation plans/specs; AmnesiaGraph tracks execution of those plans instead of replacing them.

AmnesiaGraph is deliberately not a project-management system, graph database, general agent memory, orchestrator, or web UI.

## Install

```text
go install github.com/turio/AmnesiaGraph/cmd/amnesia@latest
```

AmnesiaGraph requires a real implementation plan with discrete executable tasks. Normalize that plan once into a small JSON input: preserve existing stable IDs, assign deterministic `T001`-style IDs when needed, convert ordering into `depends_on` edges, and point each task's `source` back to the original plan. Keep full instructions in the original plan; do not copy them into the graph.

## Commands

| Command | Purpose |
|---|---|
| `amnesia init <name> <normalized-graph.json>` | Validate, install, and select a named graph |
| `amnesia list` | List named graphs, marking the selected one |
| `amnesia use <name>` | Select an existing unfinished graph |
| `amnesia validate` | Validate the selected graph |
| `amnesia resume` | Show the compact active/ready/blocked neighborhood for the selected graph |
| `amnesia ready` | List pending tasks whose dependencies are done |
| `amnesia start <id>` | Start a dependency-ready task |
| `amnesia block <id> <reason>` | Block an active task with a short reason |
| `amnesia done <id>` | Run declared verification and complete an active task |

All state is stored only under `.amnesiagraph/` in the current Git repository root: one file per graph at `.amnesiagraph/graphs/<name>.json` plus a `current` pointer naming the selected graph. Commands never fall back to a home-directory file or another repository. `resume` shows source pointers labeled as the location of the full original plan instructions; it does not print the plan body.

## Minimal workflow

Create a normalized input such as:

```json
{
  "version": 1,
  "tasks": [
    {
      "id": "T001",
      "order": 1,
      "title": "Define the request contract",
      "source": "docs/IMPLEMENTATION_PLAN.md#contract",
      "depends_on": [],
      "verify": []
    },
    {
      "id": "T002",
      "order": 2,
      "title": "Implement the request flow",
      "source": "docs/IMPLEMENTATION_PLAN.md#flow",
      "depends_on": ["T001"],
      "verify": ["go test ./..."]
    }
  ]
}
```

Then run from the repository:

```text
amnesia init plan-a normalized-graph.json
amnesia resume
amnesia ready
amnesia start T001
amnesia done T001
amnesia start T002
amnesia done T002
```

### Multiple plans in one repo

Keep unrelated plans in separate named graphs; never merge them into one graph merely because they share a repository. Choose a short lowercase name such as `auth-refresh` or `renderer-v2` and initialize each plan on its own:

```text
amnesia init plan-a plan-a.json
amnesia start A-T001
amnesia done A-T001
amnesia start A-T002
amnesia init plan-b plan-b.json
amnesia resume
```

Initializing `plan-b` selects it automatically without touching `plan-a`: `A-T002` stays active in its own graph. Return later with:

```text
amnesia use plan-a
amnesia resume
```

Plain `amnesia resume` prefers the selected (`current`) graph while it is unfinished. If `current` is missing, points at a missing graph, or points at a completed graph, `resume` falls back to the most recently modified unfinished graph and persists that choice. Use `amnesia list` only when the graph identity is unclear.

Use the generic procedure in [`skills/amnesiagraph/SKILL.md`](skills/amnesiagraph/SKILL.md) with any shell-capable coding agent. The examples under [`skills/amnesiagraph/examples/`](skills/amnesiagraph/examples/) show custom, ID-less, multi-source, Spec Kit-style, and OpenSpec-style normalization inputs; they are examples, not dedicated adapters.

### Install the canonical skill

The repository contains one canonical skill source; it is not automatically discovered merely because it is checked in. Copy that same directory into the user skill storage for the harness you use:

```text
# Codex user skills
cp -R skills/amnesiagraph ~/.codex/skills/amnesiagraph

# Claude Code user skills
cp -R skills/amnesiagraph ~/.claude/skills/amnesiagraph
```

On Windows, use the corresponding user skill directories and `Copy-Item -Recurse`. Keep the source body in `skills/amnesiagraph/SKILL.md`; do not maintain a second harness-specific copy.
