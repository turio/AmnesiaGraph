# AmnesiaGraph

**AmnesiaGraph prevents coding agents from losing execution state so they can keep working reliably through 10+ hour implementation runs.**

Long coding jobs can outlive a model's useful context. Context gets compacted, sessions restart, and agents lose track of what is done, what is active, what is blocked, and what should happen next.

AmnesiaGraph keeps that execution state outside the model context in a small repo-local graph.

It tracks:

- completed and active work;
- blockers;
- task dependencies;
- ready work;
- required verification;
- pointers back to the original implementation plan.

The original plan remains authoritative. AmnesiaGraph stores only the execution state needed to continue it.

This is especially useful with lower-cost models such as **Luna and Qwen**, which have less context and reasoning capacity to spend reconstructing execution state during long implementation runs.

## Installation

Use a prompt like this in Claude Code or Codex:

```text
Install AmnesiaGraph from https://github.com/turio/AmnesiaGraph
as a global skill so that I can use it with slash commands.
```

### Usage

Once the skill is installed, use prompts like:

```text
Implement this plan and track the implementation using AmnesiaGraph.
```

## How it works

```text
Implementation plan (OpenSpec, Spec Kit, Claude plan mode, or any other plan)
        │
        ▼
 identify tasks + dependencies
        │
        ▼
    AmnesiaGraph
        │
 ┌──────┼────────┐
 ▼      ▼        ▼
done   active   blocked
          │
          ▼
      ready next
          │
          ▼
 implementation + verification
          │
          ▼
 context compaction / restart
          │
          ▼
 recover execution state
          │
          ▼
 continue working
```

When context is lost or execution state becomes uncertain, the agent uses:

```text
amnesia resume
```

to recover the current execution neighborhood:

```text
active work
ready work
blocked work
dependencies
verification
source pointers
```

The agent follows those source pointers back to the original plan whenever it needs the full task instructions.

## What the skill does

When asked to implement a plan using AmnesiaGraph, the skill tells the agent to:

1. identify executable tasks and dependencies;
2. preserve existing task IDs or assign deterministic IDs;
3. keep full instructions in the original plan;
4. create a separate graph for each independent plan;
5. track active, blocked, ready, and completed work;
6. run declared verification before completing tasks;
7. recover execution state after compaction, restart, or handoff.

## How it differs

**Codex `update_plan` / Claude todo tools**

Session-level working lists. AmnesiaGraph adds durable repo-local dependency, blocker, readiness, and verification state.

**Spec Kit / OpenSpec**

Define implementation plans. AmnesiaGraph tracks execution of those plans.

**Ergo**

A broader agent task graph. AmnesiaGraph stays focused on execution state beneath an existing plan.

**Beads**

A broader issue and dependency management system. AmnesiaGraph keeps only the state needed to continue implementation work.

## Manual installation

Install the CLI:

```text
go install github.com/turio/AmnesiaGraph/cmd/amnesia@latest
```

Install the canonical skill:

```text
# Codex
cp -R skills/amnesiagraph ~/.codex/skills/amnesiagraph

# Claude Code
cp -R skills/amnesiagraph ~/.claude/skills/amnesiagraph
```

On Windows, use the corresponding user skill directories and `Copy-Item -Recurse`.

The canonical skill lives at:

```text
skills/amnesiagraph/SKILL.md
```

## CLI reference

These commands are normally driven by the coding agent through the skill.

| Command | Purpose |
| --- | --- |
| `amnesia init <name> <normalized-graph.json>` | Create and select a graph |
| `amnesia list` | List graphs |
| `amnesia use <name>` | Select a graph |
| `amnesia validate` | Validate the selected graph |
| `amnesia resume` | Recover current execution state |
| `amnesia ready` | Show dependency-ready tasks |
| `amnesia start <id>` | Start a task |
| `amnesia block <id> <reason>` | Record a blocker |
| `amnesia done <id>` | Verify and complete a task |

State is stored under:

```text
.amnesiagraph/
```

inside the current Git repository.

## Execution graph format

Internally, the skill reduces the implementation plan to a small graph:

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

The coding agent creates and manages this representation. Full instructions remain in the original plan.

## Multiple plans

Independent implementation plans use independent graphs, for example:

```text
auth-refresh
renderer-v2
database-migration
```

The agent can switch between unfinished plans without losing their execution state.

Examples are available under [`skills/amnesiagraph/examples/`](skills/amnesiagraph/examples/).

## Security

Task `verify` entries are executable shell commands run from the repository root.

Only use plans and verification commands you trust.
