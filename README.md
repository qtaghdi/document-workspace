# xlsx-viewer

An AI-collaborative XLSX viewer and editor for MCP clients.

The repository currently contains two implementations:

- `server.py`: the preserved Python proof of concept.
- `cmd/xlsx-viewer`: the new Go service foundation.

This is a polyglot monorepo:

- `cmd/xlsx-viewer` contains the Go executable.
- `internal/workbook` contains the Go workbook domain and XLSX persistence.
- `internal/httpapi` contains the Go HTTP, SSE, MCP, and embedded UI adapters.
- `web` contains the TypeScript and Univer browser application.
- `docs` contains product, architecture, and decision records.

Univer is used through open-source packages only. The repository rejects
`@univerjs-pro/*` dependencies during frontend builds.

## Go service

The Go service exposes:

- a Univer-powered browser spreadsheet at `/`;
- a Streamable HTTP MCP endpoint at `/mcp`;
- an MCP App resource that embeds the spreadsheet in compatible clients;
- JSON endpoints for workbook state and edits;
- an SSE event stream used to visualize AI edits;
- live human and AI selection presence with reconnect replay;
- atomic range paste, basic formatting, and merge operations;
- revision checks and atomic XLSX saves.

Run it with an existing workbook:

```bash
make dev FILE=/absolute/path/to/example.xlsx
```

The server prints separate browser and MCP credentials. Open the tokenized
browser URL exactly as printed. MCP clients should connect to
`http://127.0.0.1:8765/mcp` and send the separately printed MCP token as a
Bearer token.

Available MCP tools:

- `open_workbook`, which requests the interactive MCP App surface;
- `get_workbook`
- `read_range`
- `apply_operations`
- `update_presence`

The embedded app uses app-only `get_events`, `apply_user_operations`, and
`update_user_presence` tools for its bridge. Clients without MCP Apps can use
the ordinary workbook tools and the standalone browser URL.

Example operation payload:

```json
{
  "baseRevision": 1,
  "operations": [
    {
      "type": "set_cell",
      "sheet": "Sheet1",
      "cell": "B2",
      "value": "updated by AI"
    }
  ]
}
```

## Development

```bash
make web-install
make verify
make build
```

JavaScript dependencies are managed with pnpm 11.9. The root workspace catalog
pins every Univer package to one exact version and the lockfile records the
complete frontend dependency graph. The root Makefile is the preferred entry
point for contributors who do not need package-level commands.

The browser application uses plain TypeScript, Univer, Vite, and Tailwind CSS.
The production build is written to `internal/httpapi/static` and embedded in the
Go binary. Vite emits a code-split standalone browser build and a self-contained
`app.html` for MCP App hosts. Generated assets and `app.html` are ignored by
Git. Use `make build`, rather than a standalone `go build`, when producing a
release binary. The MCP App build removes unused non-English hyphenation data
and is approximately 7.8 MB before transport compression. React is not required.

Run a workbook locally:

```bash
make dev FILE=/absolute/path/to/workbook.xlsx
```

Project documentation:

- [`docs/product-plan.md`](docs/product-plan.md)
- [`docs/architecture.md`](docs/architecture.md)
- [`docs/adr/0001-spreadsheet-engine.md`](docs/adr/0001-spreadsheet-engine.md)
- [`CONTRIBUTING.md`](CONTRIBUTING.md)
- [`CHANGELOG.md`](CHANGELOG.md)

Repository conventions and engineering requirements are defined in
[`AGENTS.md`](AGENTS.md).
