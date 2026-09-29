# Architecture Refactor

## Goal

Restructure the Go and TypeScript implementation around the repository's
existing domain, transport, application, and spreadsheet-engine boundaries
without changing workbook behavior, public MCP tools, HTTP routes, or visible
UI behavior.

## Current State

- `web/src/main.ts` owns startup, revision state, write serialization,
  subscriptions, presence throttling, and status rendering.
- `web/src/api.ts` branches between HTTP and MCP App transport inside one class.
- `web/src/univer-engine.ts` owns Univer construction, event translation,
  remote-event application, A1 helpers, and workbook mapping.
- `internal/httpapi/server.go` combines MCP registration, browser handlers,
  middleware, SSE, and response helpers.
- `internal/workbook/session.go` combines domain types, session lifecycle,
  reads, operation validation and application, styles, events, ranges, and
  persistence.

## Scope

- Add and apply the repository-local `xlsx-viewer-architecture` skill.
- Split TypeScript orchestration, transport, UI status, spreadsheet mapping,
  and Univer-specific behavior into focused modules.
- Replace optional-field operation types with discriminated unions.
- Validate data received over HTTP, SSE, and the MCP App bridge.
- Split Go code into focused files within existing packages.
- Preserve behavior and existing public contracts.

## Non-Goals

- No visual redesign.
- No React or additional state framework.
- No new workbook operations.
- No hosted persistence or authentication changes.
- No removal of the Python regression implementation.

## Constraints

- Workbook mutations must remain atomic and revision checked.
- Stdio stdout must remain protocol-only.
- Univer must remain behind `SpreadsheetEngine`.
- Generated frontend assets remain ignored.
- Committed documentation and comments use English and contain no em dashes.

## Decisions

- Keep current Go package boundaries and split large files by responsibility
  before considering new packages.
- Use stateful classes for browser and editor resources with explicit disposal.
- Use Zod, which is already installed, only at external TypeScript boundaries.
- Treat function declaration versus arrow function as a local readability
  choice, not an architecture rule.

## Architecture

```text
main.ts
  -> WorkbookController
      -> WorkbookClient
          -> HTTP transport or MCP App transport
      -> AppView
      -> SpreadsheetEngine
          -> Univer adapter, mappers, and event application

Go CLI
  -> HTTP or stdio adapter
      -> workbook.Session
          -> validation and operations
          -> XLSX reads and persistence
          -> ordered event stream
```

## Implementation Plan

1. Create the repository-local architecture skill and validate it.
2. Introduce strict TypeScript contracts and boundary schemas.
3. Split workbook transports and application orchestration.
4. Split Univer mapping and command translation from the engine lifecycle.
5. Split Go HTTP and workbook files by responsibility without moving package
   ownership.
6. Update architecture documentation and the changelog.
7. Run full verification and browser regression checks.

## Verification

- `make verify`
- Browser startup and workbook rendering
- Human cell edit persistence
- AI event rendering and revision update
- Desktop and narrow viewport smoke checks
- `git diff --check`
- Em dash scan

## Risks / Open Questions

- Strict boundary validation may expose previously accepted malformed payloads.
- Univer callback types are inferred from vendor APIs and must not leak into the
  application boundary.
- Manual Claude Desktop and Codex UI certification remains separate because it
  requires host restart and configuration.

## Result

Completed on 2026-09-29.

- Added and validated the repository-local `xlsx-viewer-architecture` skill.
- Introduced `WorkbookController`, `WorkbookClient`, separate HTTP and MCP App
  transports, `AppView`, Zod boundary schemas, and strict operation and event
  unions.
- Extracted A1 conversion, Univer workbook mapping, and command mapping from
  the editor lifecycle adapter.
- Split Go HTTP, MCP, middleware, workbook types, reads, operations, events,
  and persistence into focused files without changing package ownership.
- Passed `make verify`, `git diff --check`, and the em dash scan.
- Verified the real Go HTTP entry point in the browser with workbook rendering,
  a human edit saved at revision 2, an MCP AI edit rendered at revision 3 with
  visible AI presence, and a 640 by 800 viewport smoke check.
- Claude Desktop and Codex host restart certification remains a separate manual
  check because it depends on each installed host configuration.
