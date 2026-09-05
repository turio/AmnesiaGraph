# AmnesiaGraph

AmnesiaGraph is a tiny repo-local execution graph for long-running coding agents. Your existing implementation plan stays authoritative; AmnesiaGraph remembers only enough execution state to resume correctly after context loss.

It is deliberately not Beads/Ergo-style issue management, a project-management system, a graph database, agent memory, an orchestrator, or a web UI. It does not replace Spec Kit, OpenSpec, or a good custom implementation plan.

## Install

```text
go install github.com/turio/AmnesiaGraph/cmd/amnesia@latest
```

AmnesiaGraph requires a real implementation plan with discrete executable tasks. Normalize that plan once into a small JSON input: preserve existing stable IDs, assign deterministic `T001`-style IDs when needed, convert ordering into `depends_on` edges, and point each task's `source` back to the original plan. Keep full instructions in the original plan; do not copy them into the graph.

## Commands

| Command | Purpose |
|---|---|
| `amnesia init <normalized-graph.json>` | Validate and install normalized execution state |
| `amnesia validate` | Validate the installed graph |
| `amnesia resume` | Show the compact active/ready/blocked execution neighborhood |
| `amnesia ready` | List pending tasks whose dependencies are done |
| `amnesia start <id>` | Start a dependency-ready task |
| `amnesia block <id> <reason>` | Block an active task with a short reason |
| `amnesia done <id>` | Run declared verification and complete an active task |

All state is stored only at `.amnesiagraph/graph.json` under the current Git repository root. Commands never fall back to a home-directory file or another repository. `resume` shows source pointers labeled as the location of the full original plan instructions; it does not print the plan body.

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
amnesia init normalized-graph.json
amnesia resume
amnesia ready
amnesia start T001
amnesia done T001
amnesia start T002
amnesia done T002
```

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
