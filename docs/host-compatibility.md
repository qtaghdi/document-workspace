# MCP Host Compatibility

This document records the supported connection modes and the evidence required
before a host is marked as certified. A successful protocol connection does not
by itself prove that a host renders and operates the MCP App UI correctly.

## Connection Modes

| Mode | Intended use | Authentication | UI delivery |
| --- | --- | --- | --- |
| Stdio | Local Claude Desktop and Codex installations | Process ownership and local file permissions | Self-contained MCP App resource |
| Streamable HTTP | Local browser development and remote deployment | Bearer token in local mode, OAuth for hosted mode | Self-contained MCP App resource |

Stdio is the preferred local desktop path because the host starts the Go binary
and no MCP port is exposed. Streamable HTTP remains the production transport for
a hosted service. Remote Claude and ChatGPT connectors cannot reach a server
bound only to `127.0.0.1`.

## Current Certification Matrix

| Target | Tools | Resource metadata | Embedded UI | Bidirectional edits | Status |
| --- | --- | --- | --- | --- | --- |
| Go in-memory client | Passed | Passed | Bundle inspected | Passed through tool calls | Automated |
| Go subprocess over stdio | Passed | Passed | Resource readable | Workbook read passed | Automated |
| Streamable HTTP with bearer token | Passed | Passed | Resource readable | API and tool tests passed | Automated |
| Claude Desktop current release | Pending manual run | Pending manual run | Pending manual run | Pending manual run | Not certified |
| Codex Desktop current release | Pending manual run | Pending manual run | Pending manual run | Pending manual run | Not certified |

Do not change a pending host row to passed without recording the application
version, operating system, transport, workbook fixture, and observed result.

## Build the Local Host Binary

```bash
make build
```

The binary is written to `bin/xlsx-viewer`. Use absolute paths in host
configuration because desktop applications do not inherit the shell working
directory.

## Claude Desktop Stdio Configuration

Add an entry to the local Claude Desktop MCP configuration. Replace both paths
with absolute paths on the current machine.

```json
{
  "mcpServers": {
    "xlsx-viewer": {
      "command": "/absolute/path/to/xlsx-viewer/bin/xlsx-viewer",
      "args": [
        "-transport",
        "stdio",
        "-file",
        "/absolute/path/to/workbook.xlsx"
      ]
    }
  }
}
```

Fully quit and reopen Claude Desktop after changing its configuration. The
server writes MCP messages only to stdout. Diagnostics are written to stderr.

## Codex Stdio Configuration

Add the following to the applicable Codex `config.toml`. Replace both paths
with absolute paths.

```toml
[mcp_servers.xlsx_viewer]
command = "/absolute/path/to/xlsx-viewer/bin/xlsx-viewer"
args = ["-transport", "stdio", "-file", "/absolute/path/to/workbook.xlsx"]
```

Start a new Codex session after changing MCP configuration. Existing sessions
may keep their previously discovered tool list.

## Manual Certification Procedure

Use a disposable copy of a representative workbook.

1. Start a new host conversation with only `xlsx-viewer` enabled.
2. Confirm that initialization succeeds and server instructions are visible to
   the client.
3. Call `get_workbook` and `read_range` without opening the UI.
4. Call `open_workbook` and confirm the spreadsheet renders in an iframe.
5. Edit one cell in the UI and verify that the revision advances exactly once.
6. Ask the model to call `update_presence`, then `apply_operations`, and confirm
   that the AI selection and committed value appear in the UI.
7. Submit a stale revision and confirm that the write is rejected without a
   partial workbook change.
8. Close and reopen the workbook file with Excel or LibreOffice and verify the
   committed value and unaffected surrounding formatting.
9. Record host version, operating system, transport, result, and any deviation
   in this document.

## Known Constraints

- A local host configuration points to one workbook path. Workbook selection
  will move into a trusted launcher or session creation flow later.
- A hosted service requires OAuth, per-workbook authorization, stable HTTPS,
  and durable storage. The local bearer token is not a hosted identity system.
- Host support for inline MCP Apps can differ from support for ordinary MCP
  tools. Headless tools remain usable when the UI is unavailable.
