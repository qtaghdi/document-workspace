---
name: document-workspace-architecture
description: Refactor or extend document-workspace across its format adapters, document service, MCP and HTTP transports, realtime events, and browser editors. Use for structural or cross-boundary changes, not isolated documentation or styling edits.
---

# Document Workspace Architecture

Keep structural work aligned with document safety and collaboration invariants.

## Required Inputs

Read `AGENTS.md`, `docs/architecture.md`, `docs/product-plan.md`, `CHANGELOG.md`,
the affected format adapter, and its tests. Identify whether the task changes a
shared platform contract, one format, or only internal structure.

## Boundaries

Trace the complete affected path:

```text
CLI or host -> transport adapter -> format session -> durable document
                                  -> ordered events -> browser controller
                                  -> format editor adapter -> editor engine
```

Ownership rules:

- `cmd/document-workspace` parses process configuration and starts transports.
- `internal/transport` translates contracts and does not own format rules.
- `internal/formats/<format>` owns parsing, validation, persistence, and
  format-specific compatibility behavior.
- `apps/web-editor` owns orchestration and ephemeral presentation state.
- Vendor editor types stay behind format editor boundaries.
- Shared abstractions require at least two real consumers or a demonstrated
  dependency problem. Do not add placeholder format packages.

## Current XLSX Adapter

The XLSX adapter is the first production format. Preserve one revision advance
and one atomic persistence boundary per accepted batch. Keep formula content
untrusted, preserve unsupported OOXML when possible, and validate complete
operation batches before mutation.

Keep public workbook tool names, schemas, HTTP routes, event ordering, revision
semantics, and XLSX output stable during behavior-preserving refactors.

## Browser and Transport Work

- Keep `main.ts` limited to composition and startup.
- Model operations and events as discriminated unions and validate incoming
  payloads with Zod.
- Keep revision state, write serialization, subscriptions, and conflicts in
  the application controller.
- Keep HTTP and MCP App implementations behind one client contract.
- Keep stdout protocol-only in stdio mode.
- Do not weaken path, origin, token, file-size, range-size, or operation limits.

## Verification

Run `make verify` for cross-boundary changes. Run `make compatibility-test`
when XLSX parsing, mutation, persistence, or package paths change. Also confirm:

- changed Go files are formatted;
- frontend OSS and type checks pass;
- unit, integration, race, vet, and Python regression checks pass;
- saved XLSX files reopen and rejected batches remain atomic;
- generated assets, temporary documents, and secrets are not staged;
- new committed text contains no em dash characters.

## Expected Output

Report the boundaries changed, behavior preserved or added, verification run,
format fidelity and security implications, and any manual host checks remaining.
