# Architecture

## System Context

The [shared product specification](product-spec.md) defines target behavior;
the [format support matrix](format-support.md) separates current capabilities
from planned adapters and their release evidence.

document-workspace connects MCP clients, revisioned document sessions, format
adapters, and interactive browser editors. A person and an AI agent operate on
the same authoritative document revision.

```text
Claude or Codex
       |
       | MCP tools and resources
       v
Transport layer
       |
       v
Revisioned document session
       |
       +----> format adapter ----> durable source document
       |
       +----> ordered events ----> browser editor
```

XLSX is the first production format. The current process still opens one XLSX
file at startup, and the existing workbook tool contracts remain stable while
the repository becomes format-oriented.

## Architectural Principles

1. Preserve source documents before adding editing convenience.
2. A format adapter owns parsing, validation, mutation, and persistence.
3. Transport code translates contracts and never owns format rules.
4. Browser state is ephemeral. The server session is authoritative.
5. Every accepted write validates against a base revision and commits once.
6. Unsupported content must be preserved or reported before a destructive save.
7. A shared abstraction needs two real consumers. Empty format scaffolding is
   not architecture.

## Repository Boundaries

### Executable

`cmd/document-workspace` parses process configuration, opens the selected
format session, and starts HTTP or stdio transports. It contains no document
mutation rules.

### Format Adapters

`internal/formats/<format>` owns format-specific behavior. The current
`internal/formats/xlsx` package owns:

- workbook lifecycle and bounded reads;
- operation validation and transactional application;
- revision checks and ordered events;
- OOXML and Excelize compatibility behavior;
- same-directory atomic persistence;
- durable local undo and redo snapshots;
- image and chart extraction and editing.

The XLSX package does not import HTTP, MCP, or browser packages.

Future adapters will not be forced into spreadsheet operations:

| Format | Primary model | Initial editing contract |
| --- | --- | --- |
| XLSX | cells, ranges, sheets, drawings | structured workbook operations |
| DOCX | blocks, runs, tables, sections | document-tree operations |
| PPTX | slides, shapes, media | slide and shape operations |
| HWPX | sections, paragraphs, controls | HWPX package operations |
| PDF | pages, annotations, forms | inspect, annotate, fill, and regenerate |

Binary HWP requires a separately evaluated conversion or native integration
path. PDF is a fixed-layout output format, so arbitrary semantic editing is not
treated as equivalent to editing DOCX or HWPX source.

### Transport Layer

`internal/transport/httpapi` owns HTTP routing, security middleware, SSE, MCP
tool registration, MCP App resources, and the loopback browser fallback. It
depends on the active format adapter but does not implement workbook rules.

The current MCP tools are workbook-specific and remain stable:

- `open_workbook`
- `get_workbook`
- `read_range`
- `get_sheet_objects`
- `apply_operations`
- `restore_history`
- `update_presence`

New format adapters may expose format-specific tools before a proven common
document contract exists. A future discovery tool can identify the active
format and its capabilities without weakening typed operation schemas.

The MCP App resource is versioned as
`ui://document-workspace/xlsx/v1.html`. The same server supports Streamable
HTTP and stdio. Stdio mode can start an authenticated loopback browser fallback
with a single-use launch token.

### Browser Editor

`apps/web-editor` is the current XLSX browser editor. It uses TypeScript, Vite,
Tailwind CSS, and Univer open-source packages. `SpreadsheetEngine` isolates
Univer so application orchestration and transports do not depend directly on
editor-vendor types.

`WorkbookController` owns revision state, serialized writes, presence
throttling, conflicts, and subscription lifecycle. HTTP and MCP App clients
implement one `WorkbookClient` contract. Zod validates incoming payloads before
they enter application state.

A future non-spreadsheet editor may be another application or a format surface
selected by a shared shell. The current code is not renamed to a generic editor
until that second surface exists.

### Compatibility Corpus

`testdata/<format>` stores public, generated, or provenance-recorded fixtures.
The XLSX corpus lives in `testdata/xlsx/compatibility`. Tests copy fixtures to a
temporary directory, inventory unrelated features, edit through `xlsx.Session`,
save, reopen, and compare preserved features.

The corpus currently includes an Excelize-generated workbook and a LibreOffice
export. A Microsoft Excel-produced fixture remains required before broader
fidelity claims.

### Legacy Reference

`legacy/xlsx-python` preserves the original Python proof of concept and its
regression test. It remains until equivalent behavior is fully characterized by
the Go implementation.

## XLSX Session Model

Each session has a stable ID, current revision, ordered event sequence, active
subscribers, and an in-memory Excelize file. A write includes the caller's base
revision. The session validates the entire batch before mutation, persists it
atomically, advances the revision once, and then publishes ordered events.

The session retains up to ten revision snapshots. Individual snapshots larger
than 32 MB are not retained. The `.xlsx-viewer-history` directory name remains
stable for upgrade compatibility even though the product has been renamed.

Initial worksheet data loads in bounded ranges. The browser requests aligned
tiles as the viewport moves, while Univer keeps the grid virtualized.

Writes mutate a candidate workbook and retain the authoritative in-memory file
until persistence succeeds. History restoration also parses and inspects its
candidate before disk replacement. Failed preparation, writes, sync, close,
or replacement leave the previous revision and in-memory state intact. A
workbook saved successfully with a failed history-manifest finalization is
reported as saved with a history warning; the pending manifest is retained and
must reconcile before the next write.

A sidecar `write.lock` directory serializes cooperating writers and history
recovery. The source hash is checked before mutation and immediately before
replacement, including undo/redo. Changed, removed, or non-regular sources are
rejected. Symlink paths are resolved on opening. The lock does not coordinate
Excel, hard-link aliases, network filesystems, or other non-cooperating writers;
a final check-to-rename race remains. Do not edit the same file simultaneously
in an external application. A crash leaves the lock in place and fails closed.
See [recovery operations](xlsx-recovery.md) for safe manual recovery and limits.

## XLSX Compatibility

The browser maps supported validation and conditional-formatting rules to OSS
Univer APIs. Unsupported advanced rules remain in the XLSX package and produce
a visible notice when practical.

PNG, JPEG, and GIF images use Univer drawing support. Supported chart types are
rendered as SVG previews because native Univer chart editing is not open
source. Images and chart previews can be moved, resized, or deleted. AI
operations can update existing chart titles. Image insertion and chart type,
series, axis, legend, and style editing remain pending.

Formula storage and formula calculation are separate capabilities. The service
does not calculate formulas. Cached source values are labeled separately from
unavailable values, while Univer results are browser previews. Formula-affecting
edits request full native recalculation in the saved XLSX. See the
[formula policy](xlsx-formula-policy.md). Workbook content is never evaluated as
executable source code.

## Realtime Events

SSE is used for the current server-to-browser event flow. Events include a
sequence, revision, actor, type, format location, and operation-specific data.
The service retains a bounded replay window and supports reconnect with
`Last-Event-ID`.
Snapshots include an event sequence checkpoint. New browser subscriptions start
there, avoiding historical structural edits already included in the snapshot.
A replay gap or invalid future cursor returns a system `workbook.reload` event
with `state: resync`; slow subscribers are disconnected instead of silently
losing events. Browser subscriptions also track the session identity so a
restarted process cannot reuse an old cursor unnoticed.

After a failed write, connection interruption, or resync signal, the controller
pauses all queued writes and checks the server snapshot without retrying an
operation. The recovery panel leaves drafts visible and offers an explicit
reload. It never treats a newer server revision as proof that a specific
uncertain write succeeded. This is conservative reconciliation, not a durable
idempotency-receipt API.

Typing animation and cursor movement are presentation state. Durable writes
occur at operation or batch granularity, never once per displayed character.

## Security Boundaries

Documents, formulas, names, values, MCP inputs, and browser inputs are
untrusted. Local mode uses separate random browser and MCP credentials, strict
cookies, same-origin write checks, bounded requests, and loopback-only fallback
listeners. Stdio stdout remains protocol-only.

Before parsing an XLSX file, the adapter requires a regular OOXML ZIP package
and rejects encrypted entries, unsafe or duplicate part names, more than 10,000
parts, any part larger than 256 MiB when expanded, more than 512 MiB of total
expanded data, and more than 100 MiB of compressed source data. These are safety
ceilings, not demonstrated performance capacity.

Hosted mode will require identity, per-document authorization, immutable object
versions, tenant isolation, audit storage, quotas, and background processing.

## Deployment Modes

### Local

- Open a user-selected document through a supported format adapter.
- Bind browser and MCP HTTP endpoints to loopback by default.
- Support stdio launch by desktop MCP hosts.
- Persist changes to the local source document.

### Hosted

- Authenticate users and MCP clients.
- Authorize every document read and write.
- Store immutable document versions in object storage.
- Store identity, metadata, active revisions, and audit records in a relational
  database.
- Isolate conversion and large-document work in bounded background jobs.

## Decisions Pending

- The first shared session interface after a second format adapter exists.
- DOCX editing engine and round-trip strategy.
- PPTX canvas and animation preservation strategy.
- HWPX coverage and binary HWP integration strategy.
- PDF annotation, form, OCR, and regeneration boundaries.
- XLSX formula calculation strategy and remaining advanced feature mappings.
- Hosted tenancy, durable event replay, and audit persistence.

See [ADR 0001](adr/0001-spreadsheet-engine.md) for the XLSX editor decision.
