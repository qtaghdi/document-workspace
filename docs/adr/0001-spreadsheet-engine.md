# ADR 0001: Use Univer Behind a Spreadsheet Engine Boundary

- Status: Accepted for technical validation
- Date: 2026-09-28

## Context

xlsx-viewer needs an Excel-like spreadsheet surface that can run inside an MCP
App, support direct human editing, and make AI activity visible as cursor,
selection, typing, and committed changes. The UI must also remain usable through
MCP tools when a host does not render an embedded application.

The two leading candidates were Univer and ONLYOFFICE Docs. Building the grid,
formula engine, selection model, clipboard behavior, accessibility model, and
collaboration layer from scratch was also considered.

## Decision Drivers

1. First-class programmatic cell and range operations.
2. A practical path to AI presence and remote selection rendering.
3. Compatibility with an MCP App iframe without requiring another nested
   editor iframe.
4. XLSX import and export fidelity.
5. Keyboard, clipboard, formula, formatting, and large-sheet behavior.
6. License and deployment fit for a proprietary hosted service.
7. Ability to replace the editor without rewriting the Go workbook domain.

## Comparison

| Criterion | Univer | ONLYOFFICE Docs | Custom engine |
| --- | --- | --- | --- |
| Excel-like feature coverage | Strong core, with several advanced features in Pro | Strongest ready-made coverage | Initially weak |
| XLSX fidelity | Requires exchange service and compatibility testing | Mature document conversion and editing path | Must be built and maintained |
| AI cell and range control | Direct Facade API and command system | Plugin and Office API access | Complete control after substantial work |
| Remote cursor and selection | Collaboration presence supports sheet ranges | Mature human co-editing, but an AI identity needs a session or custom overlay | Must be built |
| MCP App embedding | Can render directly in the MCP App frame | Normally embeds its own editor frame, which creates nested-frame policy and review work | Can render directly |
| Custom AI interaction | High | Medium | High |
| Initial engineering cost | Medium | Low to medium for standard editing | Very high |
| Commercial considerations | OSS core is Apache-2.0; collaboration and exchange features require Pro review | Proprietary product integration normally requires Developer licensing | Internal ownership, with high staffing cost |
| Engine replaceability | Good through an adapter | Good through an adapter | Not applicable |

## Decision

Use Univer as the first production candidate, isolated behind a
`SpreadsheetEngine` interface. Use the open-source core for the first editor
spike. Evaluate Univer Pro collaboration, presence, and XLSX exchange before a
commercial commitment.

The Go service remains authoritative for the current phase. It owns revisions,
validated operations, conflict detection, persistence, authentication, and MCP
tools. Univer is a rendering and interaction engine, not the source of truth.

React is not an architectural requirement. The initial integration uses plain
TypeScript. Vite bundles the browser code, and Tailwind CSS provides application
shell styling. Univer supplies its own editor styles.

## Consequences

### Positive

- The editor runs directly in the application frame.
- The Facade API maps naturally to MCP operations.
- Univer's command model provides a path to undo, redo, and collaboration.
- The engine interface preserves an exit path to ONLYOFFICE or a custom engine.

### Negative

- XLSX import, export, collaboration, and presence require a separate licensing
  and infrastructure decision.
- Preset mode produces a large initial bundle. Plugin mode and code splitting
  must be evaluated before release.
- The current bridge synchronizes confirmed single-cell edits. Multi-cell paste,
  structural changes, formatting, and remote selection rendering still need
  operation adapters.
- Formula behavior and XLSX fidelity require a representative compatibility
  corpus. They must not be assumed from API availability.

## Rejection Conditions

Re-evaluate this decision if any of these conditions are met:

- Required Pro licensing is incompatible with the product business model.
- The compatibility corpus shows unacceptable XLSX loss.
- MCP App host constraints prevent the Univer surface from operating reliably.
- Bundle size cannot meet an agreed startup budget after plugin-mode work.
- Presence cannot represent an AI actor without weakening authorization or
  revision guarantees.

If Univer fails, evaluate ONLYOFFICE behind the same engine boundary. Build a
custom engine only for missing layers that cannot be supplied or replaced, not
as the first implementation of the entire spreadsheet stack.

## References

- [Univer installation and preset mode](https://docs.univer.ai/guides/sheets/getting-started/installation)
- [Univer range and selection API](https://docs.univer.ai/guides/sheets/features/core/range-selection)
- [Univer collaboration presence](https://docs.univer.ai/server/collaboration/presence)
- [Univer import and export](https://docs.univer.ai/guides/sheets/features/import-export)
- [ONLYOFFICE co-editing](https://api.onlyoffice.com/docs/docs-api/get-started/how-it-works/co-editing/)
- [ONLYOFFICE spreadsheet API](https://api.onlyoffice.com/docs/office-api/usage-api/spreadsheet-api/Api/Methods/GetSelection/)
- [OpenAI MCP App UI guidance](https://developers.openai.com/plugins/build/chatgpt-ui)
