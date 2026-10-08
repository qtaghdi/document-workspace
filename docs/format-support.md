# Format Support Matrix

Baseline: 2026-10-08, implementation commit `4a6cfeb`.
This is the capability inventory and initial delivery scope, not a declaration
of universal compatibility. Requirements shared by all formats are defined in
the [product specification](product-spec.md).

## Status and Evidence Rules

- Partial: implemented for a bounded feature set, with remaining limitations.
- Planned: selected product scope with no available implementation.
- Research: feasibility and delivery scope are not yet accepted.
- Deferred: intentionally outside the initial delivery scope.

Promotion to verified support requires a named operation, entry point,
representative fixture, and reproducible result. Library support alone is not
product support. A feature available to AI is not necessarily exposed in the
human editor. Record these entry points separately in detailed format specs.

## Current Product Support

The process currently opens one existing XLSX file. Every other format below
is unavailable through the product today. All entries describe product support,
not the theoretical capabilities of dependencies.

| Format | Open and inspect | Create new | Render | Edit and native save | Preserve unrelated content | Convert or export |
| --- | --- | --- | --- | --- | --- | --- |
| XLSX | Partial | Planned | Partial | Partial | Partial, corpus-bounded | Planned |
| DOCX | Planned | Planned | Planned | Planned | Planned | Planned |
| PPTX | Planned | Planned | Planned | Planned | Planned | Planned |
| HWPX | Planned | Planned | Planned | Planned | Planned | Research |
| Binary HWP | Research | Deferred | Research | Research | Research | Research |
| PDF | Planned | Planned via export pipeline | Planned | Planned for bounded PDF operations | Planned | Research |

A PDF export pipeline has not been selected. PDF creation here means generating
a new output, not promising a word-processor-style blank PDF editor.
Legacy DOC, XLS, PPT, macro-enabled variants, ODF, CSV, and other extensions are
outside this initial matrix. They require separate capability decisions; they
must not inherit support from similarly named formats.

## Initial Scope by Format

| Format | First useful delivery | Content to preserve or flag | Deferred or unresolved |
| --- | --- | --- | --- |
| XLSX | Existing workbooks; cells, formulas, basic styles, ranges, sheet dimensions, existing drawings | Untouched formulas, styles, validation, conditional formatting, charts, images, links, names, comments | New images; advanced chart editing; full formula calculation; macros; encryption |
| DOCX | Create a simple document; inspect and edit paragraphs, runs, and basic tables; native save | Sections, headers, footers, comments, notes, media, relationships, and unknown parts during unrelated edits | Exact pagination; tracked-change authoring; complex fields; floating layout; engine selection |
| PPTX | Create a basic deck; inspect slides; edit text and simple shapes; move and resize images; slide ordering | Themes, masters, notes, relationships, and unsupported animation metadata | Animation authoring; embedded application objects; full presentation playback fidelity |
| HWPX | Create a basic document; inspect sections; edit text and simple tables; native save | Paragraph and character properties, controls, media, and untouched package content | Complex layout and controls; engine selection; proprietary extensions |
| Binary HWP | Feasibility prototype for inspection or conversion on copies | Original binary source must remain unchanged during evaluation | Native editing, creation, and round-trip guarantees |
| PDF | Render pages; inspect available text; annotations; supported form filling; page organization | Untouched page content, annotation relationships, and form values within tested scope | Arbitrary text reflow; OCR; secure redaction; signature handling; engine selection |

Preservation rows are verification obligations, not unconditional promises.
When an adapter cannot preserve a listed feature, it must report the restriction
and refuse unsafe native overwrite. OCR and secure redaction remain later PDF
work; a visual overlay must never be marketed as removal of underlying content.

## XLSX Baseline Detail

| Feature | Current entry points and behavior | Remaining evidence or work |
| --- | --- | --- |
| Cells, formulas, ranges, basic formatting | Human editor and AI operations commit supported edits; structured policy labels cached values and browser previews; native recalculation is requested | Excel-produced fixture and native Excel recalculation evidence remain pending |
| Row and column insertion or deletion | Human editor and AI operation paths | Expand structural-edit fidelity fixtures |
| Images | Existing images render; human and AI move, resize, and delete persist | Image insertion |
| Charts | Selected types render as previews; human and AI move, resize, and delete; AI can update existing titles | Type, series, axes, legend, style, and broader rendering coverage |
| Validation and conditional formatting | Existing common rules render or participate in browser behavior | Advanced rules and rule-authoring coverage are not complete |
| Undo and redo | Server-owned, revisioned, bounded persistent history | Not unlimited history or a general audit system |
| Preservation | Generated and LibreOffice fixtures test unrelated edit and reopen | Microsoft Excel provenance fixture and broader corpus |
| Desktop hosts | Implemented MCP transports, app resource, and browser fallback | Track individual certification modes in the host matrix |

Evidence anchors: [XLSX corpus](../testdata/xlsx/compatibility/README.md),
[corpus tests](../internal/formats/xlsx/compatibility_test.go),
[operation contracts](../internal/formats/xlsx/types.go),
[transport tests](../internal/transport/httpapi/server_test.go), and
[host certification](host-compatibility.md).

## Conversion Policy

No general conversion route is implemented or certified today. Evaluate routes
individually and record source format, target format, engine, license, expected
losses, and verification evidence before exposing them.

| Candidate direction | Status | Acceptance focus |
| --- | --- | --- |
| DOCX to PDF | Planned | Page layout, fonts, tables, images, and selectable text |
| PPTX to PDF | Planned | Slide size, object placement, fonts, and explicit loss of animation |
| XLSX to PDF | Planned | Print areas, scaling, page breaks, and formula-result policy |
| HWPX to PDF | Research | Korean text, font availability, table layout, and viable engine |
| HWP to HWPX | Research | Conversion feasibility, source preservation, and reported losses |
| PDF to editable DOCX or HWPX | Deferred | Reconstruction would be a derived document, not original-source recovery |

These candidates do not impose an engine choice or authorize external uploads.
A conversion involving a service must have an explicit data handling decision.

## Fixture and Release Gates

Each fixture record must include origin, creating application and version when
known, redistribution permission, content inventory, and expected results.
Private user documents must not enter the committed corpus. Always test copies.

| Format | Required corpus emphasis | Initial release gate |
| --- | --- | --- |
| XLSX | Generated, LibreOffice, and Microsoft Excel files; formulas, drawings, advanced rules, large sheets | Supported edits survive save and reopen; unrelated feature assertions pass; host workflows recorded |
| DOCX | Word and LibreOffice files; Korean and Latin text, tables, sections, notes, media | Simple creation and structured edits reopen in Word; untouched parts preserved; visual differences recorded |
| PPTX | PowerPoint and LibreOffice files; themes, masters, mixed text, media, notes | Supported object and slide edits reopen in PowerPoint; untouched relationships preserved |
| HWPX | Hancom-produced files with known versions; Korean text, tables, sections, controls | Creation and bounded edits reopen in Hancom Office; unsupported controls handled explicitly |
| Binary HWP | Authorized Hancom-produced fixtures covering the chosen binary variants | Research report before any support promotion; no destructive source write |
| PDF | Text, scanned, annotated, form-based, and restricted examples | Supported page and form operations reopen in an independent viewer; visual and structural checks pass |

For each adapter, exercise invalid input, rejected batches, stale revisions,
persistence failures, resource limits, and an unrelated edit beside unsupported
content. Compare structure and visible output separately. Define fixture-specific
visual tolerances, including fonts and rendering environment, before approval.

## Next Detailed Specifications

1. Execute the [XLSX release checklist](xlsx-release-checklist.md): stable subset,
   formula policy, recovery gates, corpus, and host evidence. Image insertion
   and advanced chart authoring are explicitly deferred from this first subset.
2. Execute the [DOCX preservation prototype](docx-preservation-prototype.md):
   revision-scoped targets, bounded operations, creation, and native reopen gates.
3. Extract shared runtime contracts only after XLSX and DOCX validate them.
4. Detail PPTX, HWPX, and PDF when their implementation milestone approaches.
   Keep binary HWP as a separate research decision.

Every detailed spec must name supported human and AI actions, source-save
behavior, non-goals, limits, corpus, acceptance tests, and open decisions.
Update this matrix in the same change set whenever a capability changes.
