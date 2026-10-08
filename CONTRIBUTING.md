# Contributing

## Repository Model

document-workspace is a polyglot monorepo. Go owns format sessions,
persistence, revision control, realtime events, and MCP tools. TypeScript owns
browser interaction surfaces. A format contract change must be coordinated
across every affected boundary.

Univer open-source packages are allowed. Univer Pro packages and services are
not allowed unless the repository owner approves a new architecture decision.

## Required Workflow

All repository changes use an issue and a pull request, including documentation,
maintenance, CI, and small fixes. Do not commit or push directly to `main`.

1. Search open issues for the same problem and scope.
2. Create or refine an issue with the problem, outcome, acceptance criteria,
   scope, exclusions, risks, and required evidence.
3. Create a branch from current `main` with the issue number in its name.
4. Make focused commits and keep the issue updated if the scope changes.
5. Open a pull request using the repository template and `Closes #<number>`.
6. Record verification results, compatibility and security impact, manual
   checks, and follow-up work in the pull request.
7. Merge only after required checks pass and the repository owner approves it.

If unrelated work is discovered, create a separate issue and pull request. A
small change may use a short issue, but it does not bypass this workflow.

## Branches

Use lowercase branch names in this form:

```text
<type>/<issue-number>-<short-description>
```

Examples:

```text
feat/42-ai-presence
fix/57-revision-conflict
docs/61-oss-architecture
```

Use the same types accepted for commits: `feat`, `fix`, `docs`, `refactor`,
`test`, `build`, `ci`, `chore`, `perf`, `revert`, and `security`.

## Commits

Use Conventional Commits with an English imperative subject:

```text
<type>(<scope>): <summary>
```

Recommended scopes include `go`, `web`, `mcp`, `xlsx`, `realtime`, `docs`,
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

- Link exactly the issue that defines the pull request scope by including
  `Closes #<number>` in the linked-issue section.
- Use a Conventional Commits title.
- Explain the user or system problem before the implementation details.
- State what is in scope and out of scope.
- Include verification commands and results.
- Call out XLSX compatibility, revision, security, and MCP contract impact.
- Update `CHANGELOG.md` for material changes.
- Update architecture documents when a system boundary changes.
- Do not commit generated files under `internal/transport/httpapi/static/assets`.
- Use `make build` for release binaries so the frontend is generated before the
  Go binary embeds it.
- Keep unrelated cleanup out of the pull request.
- Keep the linked issue and pull request description current when the work
  changes. Open follow-up issues for deferred or unrelated work.

## Documentation and Text

Committed documentation, comments, commit messages, changelog entries, issue
templates, and pull request templates must use English. Do not use em dashes.
