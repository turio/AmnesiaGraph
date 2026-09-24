# AmnesiaGraph

**Your coding agent forgot what it was doing. AmnesiaGraph lets it continue.**

Long coding tasks break down when context gets compacted, a session ends, or work is handed to another agent. The implementation plan may still exist, but the agent can lose the small piece of execution state that answers:

- What is already done?
- What am I working on now?
- What is blocked?
- What is ready next?
- What must pass before a task counts as complete?

AmnesiaGraph keeps that execution state outside the model context in a tiny repo-local graph.

The important part: **you do not babysit AmnesiaGraph commands while you code. Your coding agent does.**

## The intended workflow

### 1. Install it once

Ask your coding agent to install AmnesiaGraph at the user level:

```text
Install AmnesiaGraph from https://github.com/turio/AmnesiaGraph for my coding-agent environment.

Install the `amnesia` CLI globally for my user, install the canonical
`skills/amnesiagraph` skill into the correct user-level skill directory
for every supported coding-agent harness you can detect on this machine,
and verify that both the CLI and skill are available.

Do not change my current project other than any temporary files required
for installation.
```

The canonical skill in this repository is designed to work with shell-capable coding agents such as Codex and Claude Code.

If you prefer to install manually, see [Manual installation](#manual-installation).

### 2. In any repo, tell the agent to use it

If your harness exposes installed skills as slash commands, invoke the AmnesiaGraph skill and point it at the implementation plan:

```text
/amnesiagraph Implement docs/IMPLEMENTATION_PLAN.md
```

Or just say it naturally:

```text
Implement this feature based on docs/IMPLEMENTATION_PLAN.md
and track execution using AmnesiaGraph.
```

You can also give the plan inline or point at a Spec Kit, OpenSpec, Markdown, or other implementation plan.

**That is the normal human workflow.**

The agent reads the plan, creates or selects the appropriate AmnesiaGraph execution graph, tracks task state while it works, runs declared verification before marking tasks done, and uses the graph to recover after context loss or handoff.

## What happens when the agent loses context

The agent can recover the current execution neighborhood instead of reconstructing the whole job from memory or rereading the entire repository.

Under the hood, the skill has the agent run:

```text
amnesia resume
```

A resume view is intentionally small. It tells the agent which graph is active, what work is active or ready, what is blocked, what comes next, how completion is verified, and where the authoritative plan instructions live.

Conceptually:

```text
Implementation plan
        │
        ▼
 AmnesiaGraph skill
        │
        ▼
 repo-local execution state
 done / active / blocked / ready
        │
        ▼
   agent context loss
        │
        ▼
   amnesia resume
        │
        ▼
 agent continues from the right place
```

The original plan remains authoritative. AmnesiaGraph does **not** copy the whole plan into model context and does not replace your planning system.

## What AmnesiaGraph stores

Only the execution state needed to continue work:

- task IDs and dependencies;
- `pending`, `active`, `blocked`, and `done` state;
- deterministic ready work;
- short blocker reasons;
- verification commands required before completion;
- pointers back to the original plan for full instructions.

This is deliberately narrower than general agent memory. The goal is to preserve the minimum useful execution context across compaction, restarts, and handoffs.

## What the skill does for the agent

Given a real implementation plan, the canonical skill tells the coding agent to:

1. identify discrete executable tasks and their dependencies;
2. preserve existing task IDs or assign deterministic IDs when needed;
3. keep full instructions in the original plan rather than duplicating them;
4. create a separate named graph for each independent plan;
5. initialize that graph in the current repository;
6. start, block, complete, and verify tasks as implementation progresses;
7. run `amnesia resume` after context loss, handoff, or uncertainty;
8. follow source pointers back to the original plan whenever full instructions are needed.

The human should not normally need to run those lifecycle commands directly.

## How it differs

- **Codex `update_plan` / Claude todo tools** — lightweight working todo lists; AmnesiaGraph adds durable repo-local dependency, blocker, readiness, and verification state across context loss.
- **Ergo** — a lightweight agent backlog/task graph; AmnesiaGraph is a smaller execution-state overlay beneath an existing authoritative plan rather than the plan itself.
- **Beads** — a broader issue/dependency and multi-agent work-management system; AmnesiaGraph deliberately keeps only the execution context needed to resume coding work.
- **Spec Kit / OpenSpec** — create and organize implementation plans/specs; AmnesiaGraph tracks execution of those plans instead of replacing them.

AmnesiaGraph is deliberately not a project-management system, general agent memory, orchestrator, graph database, or web UI.

## Manual installation

Install the CLI:

```text
go install github.com/turio/AmnesiaGraph/cmd/amnesia@latest
```

Then install the canonical skill for the coding harness you use.

```text
# Codex user skills
cp -R skills/amnesiagraph ~/.codex/skills/amnesiagraph

# Claude Code user skills
cp -R skills/amnesiagraph ~/.claude/skills/amnesiagraph
```

On Windows, use the corresponding user skill directories and `Copy-Item -Recurse`.

The repository contains one canonical skill source at [`skills/amnesiagraph/SKILL.md`](skills/amnesiagraph/SKILL.md). Do not maintain a second harness-specific copy.

## Under the hood: CLI reference

These commands are primarily what the **agent** uses while following the AmnesiaGraph skill. They are also available for debugging or manual operation.

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

All state is stored only under `.amnesiagraph/` in the current Git repository root: one file per graph at `.amnesiagraph/graphs/<name>.json` plus a `current` pointer naming the selected graph.

Commands never fall back to a home-directory file or another repository.

## Normalized graph format

The skill converts the authoritative implementation plan into a small execution graph. Full task instructions stay in the original plan.

Example:

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

This file is an internal execution representation. In the intended workflow, the coding agent creates and manages it for you.

## Multiple plans in one repo

Independent implementation plans stay in separate named graphs. They are not merged merely because they live in the same repository.

For example, an agent working on `auth-refresh` can later switch to `renderer-v2` without destroying either plan's execution state.

Plain `amnesia resume` prefers the selected unfinished graph. If the selected graph is missing or complete, it falls back to the most recently modified unfinished graph and persists that choice.

The examples under [`skills/amnesiagraph/examples/`](skills/amnesiagraph/examples/) show custom, ID-less, multi-source, Spec Kit-style, and OpenSpec-style normalization inputs.

## Security

Task `verify` entries are executed as shell commands from the repository root (`/bin/sh -c` on Unix-like systems and `cmd.exe /C` on Windows).

Only use AmnesiaGraph with plans and verification commands you trust.
