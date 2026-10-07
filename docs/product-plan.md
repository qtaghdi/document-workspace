# Product Plan

## Context

xlsx-viewer will be an AI-collaborative spreadsheet workspace for MCP clients.
A person can edit cells directly while an AI agent reads and changes the same
workbook. The UI will show the AI's cursor, selection, typing, formatting, and
commit progress.

The Python proof of concept established the basic interaction. The Go service is
the foundation for safe persistence, revision control, realtime events, MCP
tools, authentication, and production deployment.

## Product Principles

1. A revisioned workbook session is authoritative during editing.
2. XLSX is a durable import and export format.
3. AI edits are validated operations, not arbitrary code execution.
4. The UI visualizes AI work without saving once per character.
5. MCP tools remain useful without an embedded UI.
6. Workbook fidelity is measured with a representative compatibility corpus.

## Phase 1: Safe Go Foundation

Status: complete for local single-workbook mode.

- Open one XLSX workbook as a session.
- Expose workbook metadata and range reads.
- Apply validated cell and formula operations.
- Reject writes based on stale revisions.
- Persist through a temporary file and atomic replacement.
- Expose Streamable HTTP MCP tools.
- Publish ordered edit events through SSE.
- Protect local browser and MCP access with separate tokens.

Exit criteria:

- Unit, race, vet, API smoke, MCP handshake, and XLSX reopen checks pass.
- Malformed operations do not partially modify the workbook.
- The browser reflects AI edit events.

## Phase 2: Spreadsheet Editing Experience

Status: in progress. Range paste, basic styles, merged cells, presence, and
reconnect replay are implemented. The first generated XLSX compatibility corpus
fixture is covered by an automated unrelated-edit round-trip test. Initial
worksheet data now loads in bounded chunks using server-reported sheet
dimensions, then loads aligned tiles as the user scrolls. Row and column
operations, durable bounded server-authoritative undo and redo, unsupported
feature notices, and a LibreOffice-produced compatibility fixture are
implemented. Images and chart previews can be moved, resized, and deleted with
XLSX write-back, and AI operations can update existing chart titles. Image
insertion, full chart definition editing, and a Microsoft Excel fixture remain.

- Integrate Univer behind a replaceable `SpreadsheetEngine` boundary.
- Bundle the TypeScript application with Vite and style the application shell
  with Tailwind CSS.
- Use only Univer open-source packages.
- Implement AI presence and remote selections through the Go realtime layer and
  an OSS UI overlay.
- Extend the Excelize snapshot adapter for styles, merges, validations, and
  other workbook features.
- Add cell and range selection.
- Add keyboard navigation.
- Add multi-cell copy and paste.
- Add a formula bar.
- Add row and column operations. Complete.
- Add undo and redo backed by bounded revision history. Complete for the local
  session, including restart recovery and external replacement detection.
- Add distinct pending, committed, conflicted, and failed states.
- Add accessible AI cursor and typing animation.

Exit criteria:

- The primary editing workflow works without a mouse.
- Large sheets do not require rendering every cell.
- Human and AI edits produce deterministic conflict behavior.

## Phase 3: MCP App Packaging

Status: in progress. The resource, tool metadata, dual transport adapter,
protocol integration tests, local stdio transport, and first bundle reduction
are implemented. Live host UI certification remains.

- Register an MCP App UI resource. Complete.
- Associate UI metadata only with tools that need the spreadsheet surface.
  Complete.
- Use the official MCP App postMessage bridge inside an embedded host. Complete.
- Keep ordinary workbook tools available when UI is unavailable. Complete.
- Serve an authenticated loopback browser fallback from stdio sessions and
  return a single-use launch URL from `open_workbook`. Complete.
- Reduce the self-contained Univer bundle before broad host certification.
  Complete for the initial target, from 12.7 MB to approximately 7.8 MB.
  Validation, conditional formatting, and drawing support currently bring it
  to about 8.4 MB before transport compression.
- Verify Claude Desktop and Codex host behavior and document any capability
  differences. Claude Desktop Code and Codex stdio tools, approved writes,
  conflicts, and XLSX reopening are certified. The Codex loopback browser
  fallback is also certified for launch, rendering, human writes, AI presence,
  AI commits, and XLSX reopening. Embedded UI remains pending in both tested
  host paths.
- Show partial tool input as uncommitted ghost text only when the host supports
  it.

## Phase 4: Hosted Service

- Add OAuth or OIDC identity.
- Authorize workbook ownership on every request and tool call.
- Store workbook versions in object storage.
- Store identity, metadata, and revisions in a relational database.
- Move large imports and exports to background jobs.
- Add request tracing, metrics, audit events, and recovery workflows.
- Add quotas for workbook size, operation count, and active sessions.

## Out of Scope for the Initial Release

- A complete Excel-compatible formula engine.
- Google Sheets scale multi-user CRDT collaboration.
- Macro execution.
- Editing encrypted workbooks.
- A guarantee that every proprietary Excel extension is preserved.

## Milestones

| Milestone | Scope | Estimate |
| --- | --- | --- |
| M1 | Go backend, MCP tools, atomic saves, revisions, SSE | 3 to 5 days |
| M2 | Production grid, selection, paste, AI presence | 1 to 2 weeks |
| M3 | MCP App packaging and host compatibility | 3 to 5 days |
| M4 | Identity, durable storage, deployment, operations | 1 to 2 weeks |

## Immediate Next Work

1. Add a provenance-recorded Microsoft Excel fixture to the XLSX compatibility
   corpus. LibreOffice coverage is complete.
2. Add image insertion and chart type, series, axis, legend, and style editing.
   Existing images and chart previews already support placement, resize, and
   deletion write-back, and existing chart titles can be changed by AI tools.
   Named-range lists, formula-backed bounds, icon sets, and other advanced
   workbook rules remain future compatibility work.
3. Certify the loopback browser fallback and browser-originated edits in the
   current Claude Desktop browser panel. Native embedded MCP App rendering
   remains pending in both hosts.
4. Continue measuring startup cost and remove additional plugins only when the
   editing workflow remains intact.
