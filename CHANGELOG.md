# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project intends to follow Semantic Versioning when releases begin.

## Unreleased

### Added

- Added a Univer-powered spreadsheet UI behind a replaceable engine interface.
- Added Vite and Tailwind CSS build tooling without adding a React application
  dependency.
- Added human single-cell persistence and committed AI edit synchronization in
  the Univer adapter.
- Added an integration test for authenticated delivery of the embedded UI and
  its JavaScript assets.
- Added an architecture decision comparing Univer, ONLYOFFICE, and a custom
  spreadsheet engine.
- Added repository ignore rules for local workbooks, secrets, dependencies, and
  build artifacts.
- Added root monorepo commands for frontend, Go, build, test, audit, and local
  development workflows.
- Added an OSS-only dependency guard that rejects Univer Pro packages.
- Added contribution rules and GitHub templates for issues and pull requests.
- Ignored generated Vite assets while retaining the embedded Go build flow.
- Added a Go service foundation using the official MCP Go SDK and Excelize.
- Added `get_workbook`, `read_range`, and `apply_operations` MCP tools.
- Added revision-based conflict detection for workbook writes.
- Added same-directory temporary writes and atomic workbook replacement.
- Added an authenticated browser session and a token-protected MCP endpoint.
- Added a browser spreadsheet prototype with direct cell editing.
- Added an SSE stream for AI cursor, typing, and commit events.
- Added bidirectional human and AI range presence with OSS selection rendering.
- Added bounded SSE event replay using `Last-Event-ID`.
- Added atomic rectangular paste, basic cell formatting, and merge operations.
- Added initial XLSX style and merged-cell rendering in the Univer adapter.
- Added separate browser and MCP credentials for local role separation.
- Added workbook and operation count limits.
- Added workbook persistence and formula round-trip tests.
- Added repository architecture, product planning, and engineering guidance.
- Added an MCP App resource, `open_workbook` tool metadata, and app-only
  collaboration tools using the official MCP Apps bridge.
- Added separate browser and self-contained MCP App production bundles.
- Added an in-memory MCP protocol test for the interactive tool and resource.
- Added stdio MCP transport for local desktop hosts and a subprocess protocol
  test that exercises workbook reads through the real transport boundary.
- Added a host compatibility matrix and local Claude Desktop and Codex launch
  configuration examples.
- Added a repository-local architecture skill for coordinated Go, MCP, realtime,
  TypeScript, and Univer changes.
- Added runtime validation for workbook snapshots, ranges, operations responses,
  and realtime events received by the browser.
- Added a recorded Codex Desktop and CLI stdio certification covering workbook
  reads, approved writes, revision conflicts, and XLSX reopening.
- Added a recorded Claude Desktop and Code stdio certification covering tool
  reads, one-time approvals, writes, revision conflicts, and XLSX reopening.

### Changed

- Defined the repository as a polyglot monorepo with Go as the authoritative
  workbook and collaboration service.
- Replaced the planned Univer Pro evaluation with an OSS-only presence and XLSX
  integration strategy owned by this repository.
- Replaced the Univer sheets preset with explicit OSS plugin registration and
  reduced the self-contained MCP App bundle from 12.7 MB to about 7.8 MB.
- Migrated JavaScript dependency management from npm to a pnpm workspace with a
  shared exact-version catalog for Univer packages.
- Versioned the MCP App resource URI, added server usage instructions, declared
  complete tool safety annotations, and added the OpenAI compatibility alias.
- Split browser orchestration, HTTP and MCP App transports, UI presentation,
  workbook mapping, and Univer command mapping into focused TypeScript modules.
- Split the Go HTTP adapter and workbook domain into focused files while
  preserving routes, tool contracts, event ordering, revision semantics, and
  atomic XLSX persistence.
- Replaced loose frontend operation and event interfaces with discriminated
  unions that encode required fields for each command and event type.
- Documented host-owned manual, automatic, one-time, and persistent approval
  behavior for workbook MCP tools.

### Fixed

- Updated the browser revision after AI formatting and merge operations so the
  next human edit does not submit a stale base revision.

### Security

- Updated Univer to 1.0.2 after dependency audit and verified that the installed
  dependency tree reports no known vulnerabilities.
- Allowed only the data and blob image, font, and worker sources required by
  Univer while keeping scripts restricted to same-origin assets.
- Added random session tokens, strict session cookies, browser Origin checks,
  request body limits, range size limits, and baseline security headers.

## 0.0.1 - 2026-09-28

### Added

- Added the initial Python XLSX viewer and editor proof of concept.
- Added support for worksheet tabs, direct cell edits, list validation controls,
  and a limited subset of expression-based conditional formatting.
