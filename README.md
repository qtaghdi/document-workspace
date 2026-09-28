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
- JSON endpoints for workbook state and edits;
- an SSE event stream used to visualize AI edits;
- revision checks and atomic XLSX saves.

Run it with an existing workbook:

```bash
go run ./cmd/xlsx-viewer -file ./example.xlsx
```

The server prints a tokenized local URL. Open that exact URL in a browser. MCP
clients should connect to `http://127.0.0.1:8765/mcp` and send the printed token
as a Bearer token.

Available MCP tools:

- `get_workbook`
- `read_range`
- `apply_operations`

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

The browser application uses plain TypeScript, Univer, Vite, and Tailwind CSS.
The production build is written to `internal/httpapi/static` and embedded in the
Go binary. Generated files under `internal/httpapi/static/assets` are ignored by
Git. Use `make build`, rather than a standalone `go build`, when producing a
release binary. React is not required.

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
