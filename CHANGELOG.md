# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project intends to follow Semantic Versioning when releases begin.

## Unreleased

### Added

- Added a Go service foundation using the official MCP Go SDK and Excelize.
- Added `get_workbook`, `read_range`, and `apply_operations` MCP tools.
- Added revision-based conflict detection for workbook writes.
- Added same-directory temporary writes and atomic workbook replacement.
- Added an authenticated browser session and a token-protected MCP endpoint.
- Added a browser spreadsheet prototype with direct cell editing.
- Added an SSE stream for AI cursor, typing, and commit events.
- Added workbook persistence and formula round-trip tests.
- Added repository architecture, product planning, and engineering guidance.

### Security

- Added random session tokens, strict session cookies, browser Origin checks,
  request body limits, range size limits, and baseline security headers.

## 0.0.1 - 2026-09-28

### Added

- Added the initial Python XLSX viewer and editor proof of concept.
- Added support for worksheet tabs, direct cell edits, list validation controls,
  and a limited subset of expression-based conditional formatting.

