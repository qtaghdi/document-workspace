# Product Plan

## Vision

document-workspace is an AI-collaborative workspace for office documents in
Claude, Codex, and other MCP clients. A person and an AI agent work against one
revisioned document session with visible activity, explicit commits, conflicts,
undo, and format-aware persistence.

The product targets useful, safe interoperability rather than claiming that
one editor can reproduce every proprietary desktop feature. Compatibility is
measured separately for source preservation, rendering, and editing.

## Product Principles

1. Preserve the user's source document before adding convenience.
2. Give every format its own typed operations and compatibility contract.
3. Keep revision, approval, history, and presence behavior consistent.
4. Show unsupported or potentially lossy features before saving.
5. Keep MCP tools useful when an embedded UI is unavailable.
6. Verify fidelity with provenance-recorded compatibility corpora.
7. Do not execute macros, formulas, or embedded content as code.

## Compatibility Levels

See the [shared product specification](product-spec.md) for lifecycle and
collaboration requirements and the [format support matrix](format-support.md)
for current capabilities, target scope, and evidence gates.

Each format reports progress against three independent levels:

1. Preservation: unsupported content survives an unrelated save.
2. Rendering: people can inspect the document with useful visual fidelity.
3. Editing: supported changes can be saved back to the source format.

Passing one level does not imply the next level.

## Phase 1: XLSX Production Adapter

Status: in progress and usable for local workbook collaboration.

Completed foundations include:

- Go workbook sessions with revisions and atomic saves;
- Streamable HTTP and stdio MCP transports;
- an embedded MCP App and loopback browser fallback;
- direct human edits and ordered AI events;
- range paste, styles, merges, row and column operations;
- durable bounded undo and redo;
- large-sheet viewport loading;
- validation and conditional-format rendering for common rules;
- image and chart preview movement, resize, deletion, and write-back;
- generated and LibreOffice compatibility fixtures.

Remaining XLSX release work:

1. Close persistence, external-writer, and disconnect/retry safety gates.
2. Add a provenance-recorded Microsoft Excel fixture and native reopen evidence.
3. Certify the claimed Claude and Codex modes, distinguishing embedded UI from
   browser fallback rather than requiring unsupported host capabilities.
4. Complete native Excel evidence for the implemented
   [formula policy](xlsx-formula-policy.md), including cache and recalculation
   behavior.
5. Complete candidate UI, resource-limit, access, and packaging checks.

The [XLSX release checklist](xlsx-release-checklist.md) defines the first stable
subset. Image insertion, chart type/series/axis/legend/style editing, and broader
advanced rule coverage remain follow-up work outside that initial subset.

The initial XLSX release does not promise macro execution, encrypted workbook
editing, a complete Excel formula engine, or preservation of every proprietary
extension.

## Phase 2: Shared Document Platform

Status: repository boundaries established, shared runtime extraction pending a
second adapter.

- Keep formats under `internal/formats/<format>`.
- Keep HTTP, MCP, authentication, and UI delivery under `internal/transport`.
- Define capability discovery for active format sessions.
- Extract common revision, history, approval, and event contracts only after a
  second adapter validates the model.
- Add document-level audit records and immutable version metadata.
- Introduce a browser shell only when two format surfaces exist.

Exit criteria:

- XLSX behavior remains unchanged through the new repository boundaries.
- A second adapter reuses real session infrastructure without spreadsheet-only
  concepts leaking into its operation model.
- Format capabilities are discoverable and validated at transport boundaries.

## Phase 3: DOCX Adapter

- Inventory paragraphs, runs, tables, sections, comments, headers, footers, and
  embedded media.
- Provide structured text and table operations.
- Preserve unsupported OOXML parts during unrelated edits.
- Select an OSS browser editor or a constrained custom surface after a fidelity
  prototype.
- Build Word and LibreOffice provenance fixtures.

DOCX is the recommended second adapter because it tests the shared session
model without requiring fixed-layout reconstruction.
The [preservation prototype](docx-preservation-prototype.md) defines the first
experiment. Its results are pending; no DOCX engine has been selected.

## Phase 4: PPTX Adapter

- Model slides, shapes, text, images, charts, notes, and ordering.
- Provide a slide canvas with explicit object operations.
- Preserve themes, masters, transitions, and unsupported animation metadata.
- Build PowerPoint and LibreOffice provenance fixtures.

## Phase 5: HWPX and HWP

- Implement HWPX package inspection and structured editing first.
- Build fixtures with known Hancom Office provenance.
- Evaluate binary HWP conversion or native integration separately.
- Never imply binary HWP round-trip support from HWPX support.

## Phase 6: PDF Workflows

PDF is treated as a fixed-layout document, not a normal source editor.

- Inspect text, images, pages, outlines, metadata, annotations, and forms.
- Support annotation, form filling, page organization, redaction workflows, and
  export verification.
- Add OCR as an explicit derived-content workflow.
- Regenerate a new PDF for semantic edits and retain the original source.

## Phase 7: Hosted Service

- Add OAuth or OIDC identity.
- Authorize document ownership on every request and tool call.
- Store immutable document versions in object storage.
- Store identity, metadata, revisions, and audit records in a relational
  database.
- Add bounded conversion workers, tracing, metrics, quotas, and recovery.

## Immediate Next Work

1. Review the implemented [recovery protections](xlsx-recovery.md) against the
   release candidate, including documented external-writer and crash limits.
2. Add Excel evidence and close candidate host, UI, and packaging gates. Verify
   the implemented formula policy during native Excel reopening.
3. Release the verified XLSX subset, then extend object editing separately.
4. Run the DOCX preservation experiment before selecting its editor or extracting
   a generic document session. The experiment can proceed independently on copies.
