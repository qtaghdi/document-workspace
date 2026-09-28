# Architecture

## System Context

xlsx-viewer connects an MCP client, a workbook service, and an interactive
spreadsheet UI. The AI and the person operate on the same revisioned workbook
session.

```text
MCP client
    | get_workbook, read_range, apply_operations
    v
Go service
    | workbook session
    | validation and revision control
    | atomic XLSX persistence
    | ordered edit events
    v
Browser spreadsheet UI
```

## Components

### MCP Adapter

The MCP adapter maps goal-oriented tools to workbook domain calls. It does not
contain workbook mutation logic. Read and write tools are separate so clients
can distinguish side effects.

Current tools:

- `get_workbook`
- `read_range`
- `apply_operations`

### Workbook Domain

The workbook domain owns:

- Workbook lifecycle.
- Sheet and range validation.
- Operation validation.
- Revision checks.
- Serialization and atomic replacement.
- Ordered event publication.

The package must remain independent of HTTP, MCP, and browser concerns.

### HTTP API

The HTTP API supports the standalone browser UI. It provides workbook metadata,
range reads, human edit submission, and SSE events. Browser writes require a
valid session cookie and a same-origin request.

### Browser UI

The current embedded HTML is a vertical slice. The production UI will be a
separate TypeScript and React application with a virtualized grid. It will keep
focus, selection, and animation state locally while treating server snapshots
and revisions as authoritative.

## State Model

Each open workbook has:

- A stable session ID.
- An authoritative in-memory workbook representation.
- A monotonically increasing revision.
- A monotonically increasing event sequence.
- Zero or more event subscribers.

A write includes the caller's base revision. A mismatch rejects the full write.
A successful batch advances the revision once.

## Persistence

The service serializes the workbook to a temporary file in the same directory,
flushes it, closes it, and atomically replaces the original path. Using the same
directory avoids cross-filesystem rename behavior.

Hosted storage will preserve immutable workbook versions in object storage and
keep the active revision pointer in a relational database.

## Realtime Events

The initial transport is SSE because the dominant flow is server to browser.
Events contain a sequence, revision, actor, type, sheet, cell, and optional text.

Initial event types:

- `cursor.move`
- `cell.typing`
- `cell.commit`

The browser may animate `cell.typing`, but persistence occurs at cell or batch
granularity. A later version will add replay based on `Last-Event-ID`.

## Security Boundaries

Workbook files, formulas, sheet names, cell values, MCP inputs, and browser
inputs are untrusted. The service validates them without evaluating workbook
content as code.

Local mode uses an unguessable bearer token and strict browser session cookie.
Hosted mode will replace this with authenticated user sessions and per-workbook
authorization.

## Deployment Modes

### Local

- Bind to `127.0.0.1`.
- Open a user-selected workbook.
- Serve the browser UI and Streamable HTTP MCP endpoint.
- Store changes back to the local file.

### Hosted

- Run behind HTTPS.
- Authenticate users and MCP clients.
- Use object storage and a relational database.
- Isolate workbook sessions by user and tenant.
- Run large conversions in background workers.

## Architectural Decisions Pending

- Grid library selection after interaction prototypes and licensing review.
- Formula calculation strategy.
- Unsupported XLSX feature detection strategy.
- Operation log persistence format.
- SSE replay retention and reconnect policy.
- OAuth provider and hosted tenancy model.

