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
- Added a Go service foundation using the official MCP Go SDK and Excelize.
- Added `get_workbook`, `read_range`, and `apply_operations` MCP tools.
- Added revision-based conflict detection for workbook writes.
- Added same-directory temporary writes and atomic workbook replacement.
- Added an authenticated browser session and a token-protected MCP endpoint.
- Added a browser spreadsheet prototype with direct cell editing.
- Added an SSE stream for AI cursor, typing, and commit events.
- Added workbook persistence and formula round-trip tests.
- Added repository architecture, product planning, and engineering guidance.

### Changed

- Defined the repository as a polyglot monorepo with Go as the authoritative
  workbook and collaboration service.
- Replaced the planned Univer Pro evaluation with an OSS-only presence and XLSX
  integration strategy owned by this repository.

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
