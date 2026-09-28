# Contributing

## Repository Model

xlsx-viewer is a polyglot monorepo. Go owns workbook state, persistence,
revision control, realtime events, and MCP tools. TypeScript owns the browser
interaction layer. Both sides must change together when a contract changes.

Univer open-source packages are allowed. Univer Pro packages and services are
not allowed unless the repository owner approves a new architecture decision.

## Branches

Use lowercase branch names in this form:

```text
<type>/<short-description>
```

Examples:

```text
feat/ai-presence
fix/revision-conflict
docs/oss-architecture
```

Use the same types accepted for commits: `feat`, `fix`, `docs`, `refactor`,
`test`, `build`, `ci`, `chore`, `perf`, `revert`, and `security`.

## Commits

Use Conventional Commits with an English imperative subject:

```text
<type>(<scope>): <summary>
```

Recommended scopes include `go`, `web`, `mcp`, `workbook`, `realtime`, `docs`,
`build`, and `deps`.

Examples:

```text
feat(web): render AI range presence
fix(workbook): reject stale batch revisions
docs(architecture): define OSS collaboration path
build(web): update embedded assets
```

Keep the subject at 72 characters or fewer. Do not end it with a period. Use a
body when the reason, migration, compatibility risk, or operational impact is
not obvious from the diff.

Use `!` and a `BREAKING CHANGE:` footer for breaking contracts:

```text
feat(mcp)!: replace cell edit payload

BREAKING CHANGE: apply_operations now requires operation IDs.
```

## Development

Install browser dependencies:

```bash
make web-install
```

Run the standard verification suite:

```bash
make verify
```

Build the embedded UI and Go binary:

```bash
make build
```

Run the production dependency audit:

```bash
make audit
```

## Pull Requests

- Use a Conventional Commits title.
- Explain the user or system problem before the implementation details.
- Include verification commands and results.
- Call out XLSX compatibility, revision, security, and MCP contract impact.
- Update `CHANGELOG.md` for material changes.
- Update architecture documents when a system boundary changes.
- Do not commit generated files under `internal/httpapi/static/assets`.
- Use `make build` for release binaries so the frontend is generated before the
  Go binary embeds it.
- Keep unrelated cleanup out of the pull request.

## Documentation and Text

Committed documentation, comments, commit messages, changelog entries, issue
templates, and pull request templates must use English. Do not use em dashes.
