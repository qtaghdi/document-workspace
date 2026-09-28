# xlsx-viewer

An AI-collaborative XLSX viewer and editor for MCP clients.

The repository currently contains two implementations:

- `server.py`: the preserved Python proof of concept.
- `cmd/xlsx-viewer`: the new Go service foundation.

## Go service

The Go service exposes:

- a browser spreadsheet at `/`;
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
go test ./...
go build ./cmd/xlsx-viewer
python3 test_server.py
```

Project documentation:

- [`docs/product-plan.md`](docs/product-plan.md)
- [`docs/architecture.md`](docs/architecture.md)
- [`CHANGELOG.md`](CHANGELOG.md)

Repository conventions and engineering requirements are defined in
[`AGENTS.md`](AGENTS.md).

