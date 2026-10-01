# Architecture

## System Context

xlsx-viewer connects an MCP client, a workbook service, and an interactive
spreadsheet UI. The AI and the person operate on the same revisioned workbook
session.

```text
MCP client
    | open_workbook, get_workbook, read_range, apply_operations
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

- `open_workbook`
- `get_workbook`
- `read_range`
- `apply_operations`
- `update_presence`

`open_workbook` advertises the versioned
`ui://xlsx-viewer/workbook/v1.html` resource using the MCP Apps metadata
contract. The resource is a self-contained HTML bundle. It uses app-only tools
for event polling, human operations, and human presence. The ordinary tools
remain available when a client does not render MCP Apps.

The same MCP server supports Streamable HTTP and stdio. HTTP is the default for
the standalone browser and future hosted deployment. Stdio lets local desktop
hosts launch the Go binary. In stdio mode, the process also opens an ephemeral
loopback HTTP listener by default so hosts without MCP App rendering can show
the standalone UI in their browser panel. Both transports use the same tool
contracts, instructions, resource, workbook session, and tests.

`open_workbook` returns workbook metadata and, when the loopback fallback is
active, a short-lived `browserUrl`. The URL contains a single-use launch token
that expires after two minutes. Its first request exchanges the token for the
HTTP-only browser session cookie and invalidates the launch token. The reusable
browser credential is never included in MCP tool output. The fallback listener
accepts loopback addresses only and stops with the stdio session.

### Workbook Domain

The workbook domain owns:

- Workbook lifecycle.
- Sheet and range validation.
- Operation validation.
- Revision checks.
- Serialization and atomic replacement.
- Ordered event publication.

The package must remain independent of HTTP, MCP, and browser concerns.
The package is organized by responsibility within that boundary:

- `session.go` owns lifecycle and transactional batch application.
- `operations.go` owns operation validation and XLSX mutation mapping.
- `read.go` owns XLSX range, style, and merged-cell reads.
- `persistence.go` owns same-directory atomic replacement and reload behavior.
- `events.go` owns bounded event retention and subscriber delivery.
- `types.go` owns the domain and wire contract structs.

### HTTP API

The HTTP API supports the standalone browser UI. It provides workbook metadata,
range reads, human edit submission, and SSE events. Browser writes require a
valid session cookie and a same-origin request.

The adapter keeps routing and server construction in `server.go`, MCP tool and
resource registration in `mcp.go`, browser handlers and SSE in `browser.go`,
and authentication plus security headers in `middleware.go`.

### Browser UI

The browser UI is a TypeScript application bundled with Vite. Univer is the
selected spreadsheet engine for the current validation phase. Application
styling uses Tailwind CSS, while Univer provides editor-specific styles.

`main.ts` only composes dependencies. `WorkbookController` owns revision state,
serialized writes, presence throttling, and subscription lifecycle. HTTP and
MCP App transports implement one `WorkbookClient` contract, and all incoming
payloads are checked with Zod before entering application state. `AppView` owns
status and presence presentation. Pure A1, command, and workbook mapping code
is separate from the stateful Univer lifecycle adapter.

All editor-specific behavior is isolated behind `SpreadsheetEngine`. The
adapter receives workbook snapshots, emits confirmed human cell edits, and
applies committed remote events. This boundary preserves the option to replace
Univer with ONLYOFFICE or a custom engine without moving validation and
persistence out of Go.

The Go service remains authoritative. Browser focus, draft text, presence, and
animation are ephemeral. The adapter synchronizes confirmed cell edits,
rectangular paste, basic formatting, merged cells, and committed AI edits. AI
selections are rendered with Univer's OSS range highlight API.

Workbook snapshots include sheet dimensions. The browser divides the initial
used ranges into requests that stay within the server's range-size limit, then
maps each range at its original row and column offset. Univer receives the full
sheet dimensions so its grid remains virtualized even when the source contains
more rows than the first visible viewport. An initial data budget prevents an
unbounded workbook from forcing a full import into browser memory.

Only Univer open-source packages are allowed. The Go service will provide
collaboration, presence state, operation ordering, and XLSX persistence. The UI
will render AI cursors and selections through an OSS adapter or a custom overlay.

The same TypeScript application has two production entries. The standalone
entry uses code-split same-origin assets and SSE. The MCP App entry is bundled
as one HTML resource and uses the official postMessage bridge plus bounded
event polling. Both entries call the same Go workbook domain and operation
contracts.

The Univer integration registers required plugins explicitly instead of using
the complete sheets preset. The MCP App build retains English hyphenation data
and removes unused language dictionaries from Univer's renderer. This keeps the
self-contained resource below 8 MB without changing the standalone browser
build or loading runtime code from a CDN.

See [`adr/0001-spreadsheet-engine.md`](adr/0001-spreadsheet-engine.md) for the
engine comparison and decision.

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

Compatibility tests copy committed fixtures from `testdata/compatibility` to a
temporary directory, inventory unrelated workbook features, apply an edit
through `workbook.Session`, then reopen and compare the saved package. The first
fixture is an Excelize-generated baseline covering formulas, styles, merges,
validation, conditional formatting, drawings, links, names, and comments.
Producer-specific Excel and LibreOffice fixtures remain required before making
broader fidelity claims.

Hosted storage will preserve immutable workbook versions in object storage and
keep the active revision pointer in a relational database.

## Realtime Events

The initial transport is SSE because the dominant flow is server to browser.
Events contain a sequence, revision, actor, type, sheet, cell or range, and
operation-specific data.

Initial event types:

- `presence.update`
- `cell.typing`
- `cell.commit`
- `range.commit`
- `range.format`
- `range.merge_cells`
- `range.unmerge_cells`

The browser may animate `cell.typing`, but persistence occurs at cell or batch
granularity. The service retains a bounded in-memory event history and replays
events after the browser's `Last-Event-ID` on reconnect.

## Security Boundaries

Workbook files, formulas, sheet names, cell values, MCP inputs, and browser
inputs are untrusted. The service validates them without evaluating workbook
content as code.

Local mode uses separate unguessable browser and MCP tokens plus a strict
browser session cookie. The credentials separate human UI access from AI tool
access, but they are not a hosted identity system.
Hosted mode will replace this with authenticated user sessions and per-workbook
authorization.

## Deployment Modes

### Local

- Bind to `127.0.0.1`.
- Open a user-selected workbook.
- Serve the browser UI and Streamable HTTP MCP endpoint, or let a desktop host
  launch the process over stdio with an ephemeral loopback browser fallback.
- Store changes back to the local file.

### Hosted

- Run behind HTTPS.
- Authenticate users and MCP clients.
- Use object storage and a relational database.
- Isolate workbook sessions by user and tenant.
- Run large conversions in background workers.

## Architectural Decisions Pending

- Extended Excelize-to-Univer mappings for validation, conditional formatting,
  charts, images, and unsupported feature warnings.
- Further production JavaScript startup reductions beyond the current plugin
  mode and locale pruning.
- Formula calculation strategy.
- Unsupported XLSX feature detection strategy.
- Operation log persistence format.
- Durable event replay retention policy.
- OAuth provider and hosted tenancy model.
