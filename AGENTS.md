# xlsx-viewer Engineering Guide

This file is the repository-level source of truth for agents and contributors.
It applies to the entire repository unless a more specific `AGENTS.md` exists in
a subdirectory.

## Product Mission

xlsx-viewer is an AI-collaborative spreadsheet workspace for MCP clients. A
person and an AI agent can inspect and edit the same workbook while the UI shows
the AI's cursor, selection, typing, formatting, and commit progress.

The product is not only an XLSX converter. During an editing session, the source
of truth is a revisioned workbook session and its operation log. XLSX is the
durable import and export format.

## Architecture

The system has four boundaries:

1. The MCP boundary exposes goal-oriented workbook tools to AI clients.
2. The workbook domain owns validation, revisions, atomic persistence, and
   compatibility behavior.
3. The realtime boundary publishes edit events to connected UIs.
4. The browser UI renders the workbook and lets people edit it directly.

The current implementation is a single Go process:

```text
MCP client
    | Streamable HTTP tools
    v
Go service
    | workbook session, revision, operation log
    | atomic XLSX persistence
    +----------> SSE edit events ----------> Browser spreadsheet UI
```

The server owns authoritative workbook data. UI state such as focus, selection,
open panels, and in-progress animation remains ephemeral in the browser.

This repository is a polyglot monorepo. Keep the Go service, TypeScript UI,
tests, and architecture documents versioned and verified together. Vite output
under `internal/httpapi/static/assets` is generated locally and is not tracked.
Do not split the source into separate repositories without a deployment or
ownership requirement.

Read [`docs/architecture.md`](docs/architecture.md) before changing a system
boundary or adding infrastructure.

## Technology

- Go 1.25 or newer for the service.
- Official MCP Go SDK for MCP transports and tool contracts.
- Excelize for XLSX package access.
- `net/http` for the current HTTP API and SSE transport.
- TypeScript with Univer for the spreadsheet surface. Keep Univer behind the
  repository's `SpreadsheetEngine` boundary.
- Univer open-source packages only. Do not add `@univerjs-pro/*` packages or
  depend on Univer Pro services without an explicit repository owner decision.
- Vite for browser bundling and Tailwind CSS for application shell styling.
  React is not required by the application architecture.
- pnpm 11.9 is the JavaScript package manager. Use the workspace catalog to
  keep all Univer packages on one exact version. Do not add npm or Bun lockfiles.
- SSE for server-to-UI edit events. Add WebSocket only when bidirectional
  presence or latency requirements justify the operational cost.
- A relational database for identities, workbook metadata, and revision
  metadata when hosted persistence is introduced.
- Object storage for uploaded workbook versions when hosted persistence is
  introduced.

Do not introduce an additional framework, datastore, queue, or transport without
a concrete requirement and a short architecture note.

## Repository Layout

- `cmd/xlsx-viewer`: executable entry point.
- `internal/workbook`: workbook domain and persistence.
- `internal/httpapi`: HTTP, SSE, browser UI, and MCP adapters.
- `web`: TypeScript spreadsheet UI and engine adapters.
- `docs`: English Markdown architecture and product documents.
- `server.py`, `test_server.py`: preserved Python MVP and regression reference.

Keep domain logic out of HTTP handlers and UI code. Transport-specific types may
depend on domain types. The workbook domain must not depend on HTTP or MCP.

## Design Principles

1. Preserve user data before optimizing convenience.
2. Validate every operation before mutating a workbook.
3. Require a base revision for every write.
4. Persist with a same-directory temporary file and atomic replacement.
5. Treat external XLSX files and formulas as untrusted input.
6. Never execute workbook formulas as source code.
7. Keep model tools useful when a host cannot render an MCP App.
8. Keep AI animation separate from authoritative workbook commits.
9. Prefer coherent batch operations over one tool call per cell.
10. Design for graceful fallback when a host lacks optional capabilities.

## Workbook Compatibility

XLSX round-trip fidelity is a product requirement. Every workbook engine change
must be checked against representative files that cover formulas, styles,
merged cells, validation, conditional formatting, charts, images, external
links, and large sheets.

Do not claim lossless compatibility without evidence. If a feature cannot be
preserved, detect it when practical and warn before saving.

Formula calculation and formula storage are separate concerns. Storing a
formula does not imply that the service can calculate the same result as Excel.

## Operations and Events

Operations are validated commands such as `set_cell`, `set_formula`,
`set_format`, `paste_range`, `insert_rows`, and `delete_rows`.

Every accepted write must:

1. Match the current base revision.
2. Validate the complete batch.
3. Apply as one logical transaction.
4. Persist safely.
5. Advance the revision once.
6. Publish ordered events with sequence and revision numbers.

Typing animation is presentation. Do not save a workbook once per character.
Publish animation intent, then commit at cell or batch granularity.

## Security

- Bind local development servers to `127.0.0.1` by default.
- Require an unguessable session token for browser and MCP access.
- Validate `Origin` on browser writes.
- Apply request size, range size, operation count, and workbook size limits.
- Never use `eval`, shell execution, templates, or interpreters on workbook
  content.
- Normalize and authorize every storage path on the server.
- In hosted mode, verify workbook ownership on every request and tool call.
- Do not log workbook cell contents, credentials, or bearer tokens.

## Go Conventions

- Keep packages small and named for their domain responsibility.
- Accept `context.Context` at blocking or externally visible boundaries.
- Wrap errors with actionable context and preserve sentinel errors with `%w`.
- Use explicit structs for tool and API contracts.
- Avoid global mutable state.
- Protect shared session state with clear locking ownership.
- Run `gofmt`, `go test ./...`, `go test -race ./...`, and `go vet ./...` before
  completing a Go change.
- Use the repository's declared Go toolchain. Set a workspace-local `GOCACHE`
  when the execution environment restricts the default cache directory.

## Frontend Conventions

- Use TypeScript and keep editor-specific calls behind `SpreadsheetEngine`.
- Use Tailwind CSS utilities for application-owned styling. Do not grow a
  monolithic handwritten stylesheet.
- Keep the grid virtualized. Never render an entire large worksheet at once.
- Keyboard navigation, range selection, copy and paste, undo, and accessible
  focus states are core behavior.
- Keep business data on the server. Browser state is for presentation and
  drafts only.
- Respect `prefers-reduced-motion` for AI cursor and typing animations.
- Do not fake successful persistence. Show pending, committed, conflicted, and
  failed states distinctly.
- Keep all Univer packages on the same version and audit production
  dependencies after an upgrade.
- Run the OSS dependency check whenever frontend dependencies change.
- Run frontend scripts through pnpm or the root Makefile.
- Test current desktop, a small laptop viewport, and a mobile fallback.

## Comments and Documentation

Comments must explain why a decision or invariant exists. Do not narrate obvious
syntax. Public contracts, concurrency invariants, unusual XLSX behavior, and
security boundaries deserve comments.

All committed documentation, code comments, commit messages, changelog entries,
and user-visible product copy must use English unless a product localization
file explicitly requires another language.

Temporary HTML reports shown directly to the repository owner must be written in
Korean. Store them outside the repository, such as under `/tmp`, and never
commit or push them. Repository documentation must be Markdown, not HTML.

Do not use em dashes. Use a period, comma, colon, semicolon, or parentheses
instead. This rule applies to documentation, comments, product copy, changelog
entries, commit messages, and generated plans.

## Changelog

Maintain [`CHANGELOG.md`](CHANGELOG.md) using Keep a Changelog style.

- Add every user-visible, architectural, security, compatibility, or operational
  change under `Unreleased` in the same change set.
- Use `Added`, `Changed`, `Deprecated`, `Removed`, `Fixed`, and `Security`
  headings as needed.
- Do not add trivial formatting or internal refactor notes unless they affect
  contributors or behavior.
- Move entries into a dated version section when preparing a release.

## Skill Policy

Use an installed skill when its scope clearly matches the task. Read its full
instructions before acting. Use the smallest set of skills that covers the work.

Create a repository-local skill under `.agents/skills/<skill-name>/SKILL.md`
when at least one of these conditions is true:

- A specialized workflow has repeated and is easy to perform inconsistently.
- A safety-critical workflow needs a fixed checklist.
- A compatibility or release process requires repository-specific commands and
  evidence.
- The workflow contains stable project knowledge that should not be copied into
  every prompt.

Do not create speculative skills for one-off tasks. For a one-off workflow,
follow this guide and record durable decisions in `docs` instead.

Likely future skills include:

- `xlsx-compatibility-qa` for workbook corpus verification.
- `mcp-app-release` for capability, packaging, and release checks.
- `spreadsheet-ui-fidelity` for browser interaction and visual verification.

Use the installed skill creator when adding or changing a skill. A skill must
state its triggers, required inputs, workflow, safety constraints, verification,
and expected outputs.

## Git and Change Management

- Preserve the initial Python MVP until its behavior is covered by Go tests.
- Use Conventional Commits with an English imperative subject:
  `<type>(<scope>): <summary>`.
- Allowed commit types are `feat`, `fix`, `docs`, `refactor`, `test`, `build`,
  `ci`, `chore`, `perf`, `revert`, and `security`.
- Prefer scopes such as `go`, `web`, `mcp`, `workbook`, `realtime`, `docs`,
  `build`, and `deps`. Omit the scope only when a change truly spans the whole
  repository.
- Use lowercase branch names in the form `<type>/<short-description>`, such as
  `feat/ai-presence` or `fix/revision-conflict`.
- Keep commit subjects at 72 characters or fewer and do not end them with a
  period.
- Use the repository issue and pull request templates. Pull request titles must
  follow the same Conventional Commits format.
- Make focused commits. Do not commit generated frontend assets under
  `internal/httpapi/static/assets`.
- Do not combine unrelated formatting or cleanup with a feature change.
- Do not rewrite shared history unless the repository owner explicitly asks.
- Update documentation and the changelog in the same commit as the behavior.

## Definition of Done

A change is complete only when:

- The requested behavior works through its real entry point.
- Relevant unit, integration, race, and static checks pass.
- Failure and conflict paths were considered.
- Security and workbook fidelity implications were reviewed.
- `CHANGELOG.md` reflects material changes.
- Repository documentation is still accurate.
- No temporary HTML reports or QA artifacts are staged.
- New text contains no em dash characters.
