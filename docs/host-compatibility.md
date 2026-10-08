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
and the optional browser fallback binds only to an ephemeral loopback port.
Streamable HTTP remains the production transport for a hosted service. Remote
Claude and ChatGPT connectors cannot reach a server bound only to `127.0.0.1`.

## Current Certification Matrix

| Target | Tools | Resource metadata | Embedded UI | Browser fallback | Bidirectional edits | Status |
| --- | --- | --- | --- | --- | --- | --- |
| Go in-memory client | Passed | Passed | Bundle inspected | Launch exchange passed | Passed through tool calls | Automated |
| Go subprocess over stdio | Passed | Passed | Resource readable | Real listener passed | Workbook read passed | Automated |
| Streamable HTTP with bearer token | Passed | Passed | Resource readable | Not applicable | API and tool tests passed | Automated |
| Claude Desktop 2.7032.0 with Code 2.1.280 | Passed | Advertised, not rendered | Not rendered | Pending | Tool writes passed, UI path pending | Tools certified, UI pending |
| Codex Desktop 26.924.22138 | Passed | Advertised, not surfaced by CLI | Not rendered | Passed in Browser panel | Human and AI edits passed | Tools and browser fallback certified |

Do not change a pending host row to passed without recording the application
version, operating system, transport, workbook fixture, and observed result.

## Certification Records

### Claude Desktop and Code, 2026-09-30

- Host: Claude Desktop 2.7032.0 with its integrated Claude Code 2.1.280 local
  session.
- Operating system: macOS 26.5.1, build 25F80.
- Transport: local stdio using a project-scoped temporary MCP configuration
  and the absolute release binary path.
- Fixture: a disposable XLSX workbook containing a formula, style, merged
  range, and list validation.
- Read result: `get_workbook` and `read_range` completed successfully and
  returned workbook metadata, formulas, styles, and merged-cell data.
- Permission result: Claude Desktop Auto mode allowed the host to decide tool
  execution. Manual mode displayed Deny, Always allow, and Allow once for both
  read and write tool calls. Certification selected Allow once only. No
  persistent allow rule was created.
- Write result: approved writes advanced revisions 1 to 2 and 2 to 3. A write
  using stale revision 2 was rejected with the expected revision 3 conflict,
  and the rejected value was not persisted.
- Reopen result: Excel-compatible package reopening preserved both committed
  values, the formula, style, merge, and validation. The rejected cell remained
  empty.
- UI result: `open_workbook` returned structured workbook metadata, but Claude
  Desktop Code did not render the MCP App resource in this run. Keep embedded
  UI and direct UI editing pending until the spreadsheet is visibly rendered.

### Codex Desktop and CLI, 2026-09-29

- Host: Codex Desktop production release 26.924.22138, build 11645.
- CLI: `codex-cli 0.145.0`, which shares the desktop `config.toml` MCP
  configuration.
- Operating system: macOS 26.5.1, build 25F80.
- Transport: local stdio using the absolute release binary path.
- Fixture: a disposable XLSX workbook containing a formula, style, merged
  range, and list validation.
- Read result: `get_workbook`, `read_range`, and `open_workbook` completed
  successfully with structured content.
- Safety result: Codex recognized the destructive tool annotation and required
  approval before `apply_operations`.
- Write result: an approved cell write advanced revision 1 to revision 2 once.
  A second write using revision 1 was rejected, and no partial cell change was
  persisted.
- Reopen result: Excel-compatible package reopening preserved the committed
  value, formula, style, merge, and validation.
- UI result: the Codex CLI returned the `open_workbook` structured result but
  did not render the MCP App resource. The current OpenAI UI documentation
  describes iframe rendering in ChatGPT and portable behavior in compatible MCP
  Apps hosts, but does not establish Codex Desktop iframe support. Keep the
  Codex embedded UI row pending until it is visibly rendered and edited in the
  desktop host.

### Codex Browser Fallback, 2026-09-30

- Host: Codex Desktop production release 26.924.22138, build 11645.
- Operating system: macOS 26.5.1, build 25F80.
- Transport: the release binary served MCP over stdio and an authenticated UI
  on an ephemeral `127.0.0.1` listener owned by the same process.
- Fixture: a disposable XLSX workbook with visible values in cells A1 and B2.
- Launch result: `open_workbook` returned a single-use launch URL. The Codex
  Browser panel exchanged it for an HTTP-only session cookie and redirected to
  the workbook root.
- Render result: the Browser panel visibly rendered the workbook name, revision
  1, the worksheet tab, the value `Browser fallback` in A1, and the value
  `Live workbook session` in B2.
- Human edit result: a direct Browser panel edit wrote `Human committed` to F15
  and advanced the shared session from revision 1 to revision 2.
- AI edit result: `update_presence` selected C3, then `apply_operations` wrote
  `AI committed`. The Browser panel displayed the new value and reported
  `AI change saved` at revision 3.
- Reopen result: reopening the XLSX package after the stdio process exited
  preserved both the human value in F15 and the AI value in C3.
- Scope: this certifies launch, rendering, browser-originated persistence, MCP
  presence, and committed AI event synchronization through the loopback browser
  fallback. Native MCP App rendering remains a separate pending check.

References:

- [OpenAI MCP server and UI quickstart](https://developers.openai.com/plugins/build/app-quickstart)
- [OpenAI MCP App UI guidance](https://developers.openai.com/plugins/build/chatgpt-ui)

## Build the Local Host Binary

```bash
make build
```

The binary is written to `bin/document-workspace`. Use absolute paths in host
configuration because desktop applications do not inherit the shell working
directory.

## Claude Desktop Stdio Configuration

Add an entry to the local Claude Desktop MCP configuration. Replace both paths
with absolute paths on the current machine.

```json
{
  "mcpServers": {
    "document-workspace": {
      "command": "/absolute/path/to/document-workspace/bin/document-workspace",
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
When native MCP App rendering is unavailable, ask Claude to call
`open_workbook` and open its single-use `browserUrl` in the Browser panel.

## Codex Stdio Configuration

Add the following to the applicable Codex `config.toml`. Replace both paths
with absolute paths.

```toml
[mcp_servers.document_workspace]
command = "/absolute/path/to/document-workspace/bin/document-workspace"
args = ["-transport", "stdio", "-file", "/absolute/path/to/workbook.xlsx"]
```

Start a new Codex session after changing MCP configuration. Existing sessions
may keep their previously discovered tool list.
When native MCP App rendering is unavailable, ask Codex to call
`open_workbook` and open its single-use `browserUrl` in the in-app browser.

## Host Approval Modes

The MCP server classifies workbook reads as read-only, presence updates as
non-destructive, and workbook writes as destructive. These annotations give a
host the information it needs to choose an approval policy. They do not grant
permission and cannot force a host to display a particular button.

For normal workbook editing, prefer the host's manual approval mode and choose
Allow once after reviewing the complete operation batch. Always allow is a
host-side persistent rule for the selected tool or scope. Use it only when the
workbook, server command, and future operation scope are all trusted. Auto mode
lets the host decide whether each call needs confirmation and can execute a
write without showing a prompt.

An approval applies only to the operation request that was shown. The server
still validates the base revision, complete batch, workbook limits, and atomic
persistence after approval. A revision conflict requires a refreshed proposal
instead of reusing an older approved write.

## Manual Certification Procedure

Use a disposable copy of a representative workbook.

1. Start a new host conversation with only `document-workspace` enabled.
2. Confirm that initialization succeeds and server instructions are visible to
   the client.
3. Call `get_workbook` and `read_range` without opening the UI.
4. Call `open_workbook` and confirm the spreadsheet renders in an iframe. If
   the host lacks MCP App rendering, open the returned `browserUrl` in its local
   browser panel and confirm the redirect removes the launch token from the
   address bar.
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
- The stdio browser fallback requires a host that can reach the same machine's
  loopback interface. Remote MCP connectors cannot use its launch URL.
