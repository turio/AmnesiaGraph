---
name: amnesiagraph
description: Maintain a tiny repo-local execution graph beneath an existing implementation plan, and recover dependency-aware coding work after context loss.
---

# AmnesiaGraph

Use AmnesiaGraph only underneath an existing, proper implementation plan. The plan may be custom Markdown, Spec Kit, OpenSpec, or another planning format; AmnesiaGraph does not invent requirements or architecture.

## Normalize the plan once

1. Confirm that the source plan contains discrete executable tasks and their intended ordering/dependencies.
2. Preserve usable task IDs already in the plan. If there are none, assign deterministic IDs such as `T001`, `T002`, and `T003` without rewriting the plan.
3. Decompose coarse planned work into numbered executable steps when it is not yet trackable.
4. Convert ordering into explicit `depends_on` IDs and keep source order in `order`.
5. Store only short titles, dependencies, repo-relative `source` pointers, and clearly defined executable `verify` commands. Keep the full instructions in the original plan.
6. Install the normalized input with `amnesia init <normalized-graph.json>` from the repository.

Example input (initial status fields are added by `init`):

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

## Continue or recover work

Run `amnesia resume` at the beginning of work, after context loss, after handoff, or whenever execution state is uncertain. Read the complete original-plan instructions at each active task's `source` pointer. Use `amnesia ready` for dependency-gated work instead of reconstructing readiness from memory.

When beginning a task, run `amnesia start <id>`. If an active task cannot progress, run `amnesia block <id> <short reason>`. After implementation is complete, run `amnesia done <id>`; defined verification commands must pass before the CLI records `done`.

AmnesiaGraph state is repository-local. Never use state from another repository. Do not create discovered-work nodes for ordinary implementation discoveries; resolve them within the original plan's scope. No hooks, MCP integration, or harness-specific API is required.
