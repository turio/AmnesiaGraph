# Normalization examples

These fixtures demonstrate one normalized graph contract across plan styles. They are examples of input shape, not Spec Kit or OpenSpec adapters. Each JSON file can be passed to `amnesia init` from the repository root; source pointers resolve to the small source files in `sources/`.

- `custom-with-ids.json` preserves IDs already present in a custom Markdown plan.
- `custom-without-ids.normalized.json` shows deterministic `T001`/`T002` IDs supplied without editing the source plan.
- `multiple-sources.json` points tasks at more than one source file.
- `spec-kit.json` uses a Spec Kit-style task path without requiring a parser.
- `openspec.json` uses an OpenSpec-style task path without requiring a parser.
