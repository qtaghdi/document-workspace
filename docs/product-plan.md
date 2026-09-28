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

Status: in progress.

- Open one XLSX workbook as a session.
- Expose workbook metadata and range reads.
- Apply validated cell and formula operations.
- Reject writes based on stale revisions.
- Persist through a temporary file and atomic replacement.
- Expose Streamable HTTP MCP tools.
- Publish ordered edit events through SSE.
- Protect local browser and MCP access with a session token.

Exit criteria:

- Unit, race, vet, API smoke, MCP handshake, and XLSX reopen checks pass.
- Malformed operations do not partially modify the workbook.
- The browser reflects AI edit events.

## Phase 2: Spreadsheet Editing Experience

- Replace the temporary embedded UI with TypeScript, React, and Vite.
- Add a virtualized grid.
- Add cell and range selection.
- Add keyboard navigation.
- Add multi-cell copy and paste.
- Add a formula bar.
- Add row and column operations.
- Add undo and redo backed by the operation log.
- Add distinct pending, committed, conflicted, and failed states.
- Add accessible AI cursor and typing animation.

Exit criteria:

- The primary editing workflow works without a mouse.
- Large sheets do not require rendering every cell.
- Human and AI edits produce deterministic conflict behavior.

## Phase 3: MCP App Packaging

- Register an MCP App UI resource.
- Associate UI metadata only with tools that need the spreadsheet surface.
- Use host capability detection.
- Return useful text and structured content when UI is unavailable.
- Show partial tool input as uncommitted ghost text only when the host supports
  it.
- Verify supported hosts and document fallback behavior.

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

1. Add HTTP and MCP integration tests.
2. Add operation count and workbook size limits.
3. Add event replay with `Last-Event-ID`.
4. Create the TypeScript and React UI workspace.
5. Build the first XLSX compatibility corpus.

