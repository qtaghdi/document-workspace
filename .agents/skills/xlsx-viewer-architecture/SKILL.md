---
name: xlsx-viewer-architecture
description: Refactor or extend xlsx-viewer across its Go workbook service, MCP and HTTP adapters, realtime events, TypeScript application, and Univer boundary. Use for structural changes and cross-boundary features, not isolated documentation or trivial styling edits.
---

# xlsx-viewer Architecture

Use this workflow to keep structural work aligned with the product's data-safety
and collaboration invariants.

## Required Inputs

Before editing, read the repository `AGENTS.md`, `docs/architecture.md`,
`docs/product-plan.md`, `CHANGELOG.md`, the affected implementation, and its
tests. Identify the real entry point and whether the task changes behavior or
only internal structure.

## Boundaries

Trace the complete affected path:

```text
CLI or host -> HTTP or MCP adapter -> workbook session -> XLSX persistence
                                      -> ordered events -> browser controller
                                      -> SpreadsheetEngine -> Univer
```

Keep these ownership rules:

- `cmd/xlsx-viewer` parses process configuration and starts transports.
- Transport packages translate contracts. They do not own workbook rules.
- `internal/workbook` owns validation, revisions, atomic transactions,
  persistence, and event ordering.
- Browser application code owns orchestration and ephemeral presentation state.
- `SpreadsheetEngine` isolates the editor vendor. Univer types and commands do
  not cross that boundary.

Prefer splitting responsibilities inside the current package before adding a
new package or abstraction. Add a layer only when it creates a testable boundary
or removes a demonstrated dependency problem.

## TypeScript Work

- Keep `main.ts` as composition and startup code.
- Model operations and events as discriminated unions so invalid field
  combinations are not representable.
- Validate HTTP, MCP, and event payloads at their boundary. Use the installed
  Zod dependency rather than unchecked generic casts.
- Put session revision, write serialization, subscription lifecycle, and
  conflict reporting in an application controller instead of module globals.
- Keep HTTP and MCP App implementations behind one workbook client contract.
- Separate Univer construction, workbook mapping, command translation, and
  remote-event application when they change for different reasons.
- Stateful classes are appropriate for resources with lifecycle and disposal.
  Arrow functions versus function declarations is a local readability choice,
  not an architecture or modernity requirement.
- Do not add React or another state framework unless a concrete UI requirement
  cannot be handled clearly by the existing TypeScript architecture.

## Go Work

- Keep domain structs explicit and validate the complete operation batch before
  mutation.
- Preserve one revision advance and one atomic persistence boundary per accepted
  batch.
- Keep MCP registration, browser HTTP handlers, middleware, XLSX conversion,
  operation application, and event retention in focused files or packages.
- Accept `context.Context` at blocking or externally visible boundaries.
- Wrap errors with actionable context and preserve sentinel errors with `%w`.
- Keep stdout protocol-only in stdio mode.
- Do not weaken path, origin, token, workbook-size, range-size, or operation
  limits while moving code.

## Refactoring Safety

For behavior-preserving work, keep public tool names, schemas, HTTP routes,
event ordering, revision semantics, workbook output, and visible UI unchanged.
Add characterization tests before moving logic that lacks coverage. Do not mix a
large structural move with unrelated feature work.

When workbook serialization or mutation changes, verify a saved file can be
reopened and that a rejected batch leaves both memory and disk unchanged.

## Verification

Run the checks relevant to the affected boundaries. Before completing a
cross-boundary change, run the full set:

```bash
make verify
```

Also confirm:

- `gofmt` has been applied to changed Go files.
- Frontend OSS and type checks pass.
- Go unit, integration, race, and vet checks pass.
- The Python regression reference passes until it is formally retired.
- Browser behavior is inspected when orchestration or editor behavior changes.
- Generated assets, temporary reports, local workbooks, and secrets are not
  staged.
- New committed text contains no em dash characters.

## Expected Output

Report the boundaries changed, behavior preserved or added, tests actually run,
workbook fidelity or security implications, and any manual host or compatibility
verification that remains.
