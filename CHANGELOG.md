# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project intends to follow Semantic Versioning when releases begin.

## Unreleased

### Added

- Added a shared product specification covering document lifecycle, revisioned
  collaboration, host permissions, proposed review, persistence, and recovery.
- Added a format support matrix separating current XLSX capabilities from
  planned DOCX, PPTX, HWPX, PDF, and binary HWP research, with conversion scope,
  compatibility evidence, and release gates.
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
- Added a stdio browser fallback that lets desktop hosts without MCP App
  rendering open the same workbook session in a local browser panel.
- Added a recorded Codex Browser panel certification for stdio fallback launch,
  human editing, AI presence and commits, and XLSX reopening.
- Added an integration test proving that browser and MCP edits share one
  revision sequence, ordered event log, and persisted XLSX file.
- Added the first XLSX compatibility corpus fixture and an automated unrelated
  edit round-trip test for formulas, styles, merges, validation, conditional
  formatting, drawings, links, names, and comments.
- Added a repository-local `xlsx-compatibility-qa` skill for safe corpus-based
  workbook compatibility verification.
- Added row and column insertion and deletion across MCP, HTTP, realtime events,
  XLSX persistence, and the Univer UI.
- Added bounded server-authoritative undo and redo with atomic package restore
  and revision reload events.
- Added viewport-triggered range loading beyond the initial browser data budget.
- Added compatibility detection and browser notices for charts, images,
  conditional formatting, data validation, external links, and macros.
- Added OSS Univer rendering for XLSX list validation and numeric conditional
  formatting rules loaded from the Go workbook adapter.
- Added a LibreOffice-produced workbook fixture to the compatibility corpus.
- Added restart-safe local undo and redo history with hash-addressed snapshots,
  atomic manifest recovery, and external workbook replacement detection.
- Added a bounded sheet object API for browser and MCP App image and chart
  previews.
- Added OSS Univer image rendering and custom read-only previews for bar, line,
  area, pie, and doughnut charts without adding Univer Pro dependencies.
- Added OSS Univer mappings for range-backed lists, literal whole-number,
  decimal, and date validation, custom-formula validation, prompts, and errors.
- Added OSS Univer rendering for common text, formula, rank, average,
  date-period, color-scale, data-bar, unique-value, and duplicate-value
  conditional formatting rules.
- Expanded the generated XLSX compatibility fixture and adapter assertions for
  validation metadata and conditional formatting parameters.
- Added revisioned image and chart operations for placement, resize, and
  deletion through the shared browser, HTTP, and MCP contracts.
- Added existing chart title updates for AI operations while preserving chart
  type, source references, series data, and unrelated workbook features.
- Added direct Univer drawing synchronization so human image and chart preview
  transforms persist to XLSX and remote object edits trigger an authoritative
  reload.

### Changed

- Renamed the product and repository architecture to `document-workspace` and
  repositioned XLSX as the first format adapter for a multi-format document
  collaboration platform.
- Reorganized the monorepo into `apps`, `cmd`, `internal/formats`,
  `internal/transport`, `testdata/xlsx`, and `legacy` boundaries while
  preserving existing workbook tools, revisions, events, and XLSX persistence.
- Versioned the XLSX MCP App resource under the document-workspace namespace
  and renamed the Go module, executable, frontend package, and local
  architecture skill.
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
- Updated compatibility notices to describe supported image and chart
  write-back controls and the remaining advanced chart editing limits.

### Fixed

- Updated the browser revision after AI formatting and merge operations so the
  next human edit does not submit a stale base revision.
- Loaded worksheet data in bounded range chunks using server-reported sheet
  dimensions instead of limiting every sheet to the first 200 rows.
- Kept large or unsupported sheet objects from blocking workbook startup by
  omitting them with a visible compatibility notice.

### Security

- Updated Univer to 1.0.2 after dependency audit and verified that the installed
  dependency tree reports no known vulnerabilities.
- Allowed only the data and blob image, font, and worker sources required by
  Univer while keeping scripts restricted to same-origin assets.
- Added random session tokens, strict session cookies, browser Origin checks,
  request body limits, range size limits, and baseline security headers.
- Protected stdio browser fallback links with single-use two-minute launch
  tokens, loopback-only binding, and a bounded outstanding-token set.
- Validated embedded image bytes before rendering and allowed only decoded PNG,
  JPEG, and GIF data within the preview byte and object limits.
- Bounded durable history manifests, image dimensions, chart series, points,
  and labels before loading them into application memory.

## 0.0.1 - 2026-09-28

### Added

- Added the initial Python XLSX viewer and editor proof of concept.
- Added support for worksheet tabs, direct cell edits, list validation controls,
  and a limited subset of expression-based conditional formatting.
