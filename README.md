# document-workspace

An AI-collaborative workspace for office documents in MCP clients.

The project provides a shared editing session where a person and an AI agent
can inspect and modify the same document with revisions, visible presence,
conflict detection, undo, and safe persistence. XLSX is the first production
format adapter. DOCX, PPTX, HWPX, and PDF are planned as separate adapters
because each format has different editing and fidelity requirements.

## Current Support

The XLSX adapter currently provides:

- a Univer-powered browser spreadsheet;
- Streamable HTTP and stdio MCP transports;
- an embedded MCP App and authenticated loopback fallback;
- workbook metadata, range reads, formulas, styles, merges, and paste;
- row and column operations;
- human and AI presence with ordered events;
- revision conflicts and durable server-authoritative undo and redo;
- image and chart preview movement, resize, deletion, and XLSX write-back;
- bounded loading for large sheets;
- compatibility notices and atomic saves.

Univer is used through open-source packages only. The build rejects
`@univerjs-pro/*` dependencies.

## Repository Layout

```text
apps/
  web-editor/                 TypeScript browser editor
cmd/
  document-workspace/         Go executable
internal/
  formats/
    xlsx/                     XLSX session, operations, and persistence
  transport/
    httpapi/                  HTTP, SSE, MCP, and embedded UI adapters
testdata/
  xlsx/compatibility/         XLSX corpus and provenance
legacy/
  xlsx-python/                Preserved Python proof of concept
docs/                         Architecture, plans, and decisions
```

Format logic belongs in `internal/formats/<format>`. Transport and UI packages
must not own format persistence rules. Shared document abstractions will be
introduced only when a second real adapter demonstrates the common contract.

## Run the XLSX Adapter

```bash
make web-install
make dev FILE=/absolute/path/to/workbook.xlsx
```

The process prints separate browser and MCP credentials. The browser URL must
be opened exactly as printed. MCP clients connect to
`http://127.0.0.1:8765/mcp` with the separately printed bearer token.

For a local desktop host using stdio:

```bash
make build
./bin/document-workspace -transport stdio -file /absolute/path/to/workbook.xlsx
```

`open_workbook` returns a single-use, two-minute `browserUrl` when the host does
not render the MCP App. Disable that fallback with `-stdio-browser=false`.

Current workbook tools remain stable during the platform transition:

- `open_workbook`
- `get_workbook`
- `read_range`
- `apply_operations`
- `restore_history`
- `update_presence`

The embedded app also uses app-only event, object, user-operation, history, and
presence tools.

## Development

```bash
make web-install
make compatibility-test
make verify
make build
```

JavaScript dependencies use pnpm 11.9 with a shared exact-version Univer
catalog. The frontend uses plain TypeScript, Vite, Tailwind CSS, and Univer.
Production output is embedded from `internal/transport/httpapi/static`.
Generated assets and the self-contained `app.html` are not committed.

The XLSX history store retains its existing `.xlsx-viewer-history` on-disk
namespace so upgrades keep local undo history. This compatibility name is an
implementation detail and does not define the product name.

## Documentation

- [Shared product specification](docs/product-spec.md)
- [Format support matrix](docs/format-support.md)
- [XLSX stable release checklist](docs/xlsx-release-checklist.md)
- [XLSX recovery operations](docs/xlsx-recovery.md)
- [XLSX formula policy](docs/xlsx-formula-policy.md)
- [DOCX preservation prototype](docs/docx-preservation-prototype.md)
- [Product plan](docs/product-plan.md)
- [Architecture](docs/architecture.md)
- [Host compatibility](docs/host-compatibility.md)
- [Spreadsheet engine decision](docs/adr/0001-spreadsheet-engine.md)
- [Contributing](CONTRIBUTING.md)
- [Changelog](CHANGELOG.md)
- [Engineering guide](AGENTS.md)
