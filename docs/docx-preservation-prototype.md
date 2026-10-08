# DOCX Preservation Prototype Specification

Status: experiment design. No DOCX adapter, fixture corpus, engine selection,
or passing experiment is asserted by this document.
Related contracts: [product specification](product-spec.md),
[format support](format-support.md), and [architecture](architecture.md).

## Decision to Make

Determine whether a bounded structured edit can be saved as DOCX while retaining
unrelated content and reopening in Word without repair. The result should select
a persistence strategy and establish operation boundaries before a full browser
editor or common session abstraction is implemented.

Evaluate two strategies on identical inputs: targeted package/XML modification
that retains untouched content, and a higher-level document model that imports
and serializes a document. These are experimental alternatives, not selected
libraries. Research official documentation, maintenance, license, and supported
format variants before selecting candidates. The prototype requires no paid
service and must not upload documents to an external converter.

## Bounded Operations

Proposed operation names below are experimental, not registered MCP tools.

| Operation | Accepted target | Required behavior |
| --- | --- | --- |
| `replace_run_text` | One plain text run in a body paragraph | Replace explicit text; retain formatting and neighboring runs |
| `replace_cell_text` | One plain text run inside a simple unmerged table cell | Replace text without rebuilding the table or changing cell properties |
| `create_basic_document` | New output path | Create a paragraph and simple table; refuse existing destination |

Creation is a separate test arm. It must not be used as evidence that an existing
complex document can be preserved. Template creation, paragraph splitting,
cross-run replacement, lists, merged or nested table edits, tracked-change
authoring, fields, floating images, pagination, and PDF conversion are outside
the prototype. Unsupported structures may be present next to a supported target
only if they remain intact. Editing inside them must be rejected explicitly.

## Target Identity and Revision Rules

Inspection returns session-scoped opaque identifiers for paragraphs, runs,
tables, and cells, backed by the adapter's mapping to the source package.
Commands carry a base revision, target ID, expected old text, and replacement.
Never locate a target solely by matching text: repeated words are legitimate.

For this prototype, IDs are valid only for the inspected revision. A successful
commit invalidates those IDs and requires a refreshed inspection; stable IDs
across structural edits or restarts are future work. A stale revision, missing
target, mismatched old text, or unsupported target rejects the entire batch.
The browser will eventually consume the same operation model as MCP.

## Fixture Plan

Create a manifest under `testdata/docx/compatibility` when implementation starts.
This specification does not create empty adapter or fixture directories.

| Fixture | Origin requirement | Contents and purpose |
| --- | --- | --- |
| D-01 | Controlled generator with version | Minimal paragraph and simple table for operation and creation tests |
| D-02 | Microsoft Word, exact version recorded | Korean and Latin text, mixed runs, whitespace, repeated text, simple table |
| D-03 | Microsoft Word, exact version recorded | Sections, headers/footers, notes, comments, images, links, fields, tracked changes, and an isolated plain target |
| D-04 | LibreOffice, exact version recorded | Producer variation with the same supported target types |
| D-05 | Controlled malformed/adversarial copies | Truncated archive, duplicate or unsafe entry names, invalid relationships, excessive expansion, unsupported target |

Each manifest entry records SHA-256, producer/version, format variant, date,
redistribution rights, feature inventory, intended target, expected mutation,
and expected rejection cases. Use synthetic non-sensitive content; run only on
copies. If Word or an authorized Word fixture is unavailable, label native
verification blocked. Do not relabel generated output as Word-produced.

## Experiment Protocol

1. Inventory the source archive entries and hashes of their uncompressed bytes,
   relationship targets, feature counts, and selected semantic properties.
   Record source hash and a native rendering baseline before any edit.
2. Perform a no-op import/save to a new path. Reopen the result independently
   and compare it with the source before attempting mutation.
3. Independently run each supported edit on a fresh fixture copy. Use a
   same-length text replacement to isolate preservation from expected reflow.
   Run a separate longer-text case to document expected layout changes.
4. Save to a separate output, reopen with the adapter and in Word, and verify
   the intended text and all unrelated properties. Also record LibreOffice
   results; they do not substitute for Word results.
5. Repeat save/reopen twice to detect cumulative loss. Keep native application
   resaves in separate files so they cannot mask the adapter's output defects.
6. Exercise a mixed valid/invalid batch, stale target, repeated text, invalid
   XML characters, unsupported structure, and interrupted persistence.
   Rejected operations must not change content or revision.
7. Record timings, peak memory, machine, versions, input/output hashes, and
   evidence for both strategies. Do not choose an engine solely on visual appeal.

## Comparison Rules

ZIP bytes may differ because of packaging metadata, so compare uncompressed
entry payloads separately from the entire archive hash. For untouched entries,
require identical payload hashes. For a touched XML part, use a namespace-aware
structural comparison with a narrow, predeclared allowlist for the intended
text change. Preserve meaningful whitespace, run properties, element order,
unknown attributes/elements, references, and relationships outside that change.

A parser that only inventories known elements is insufficient evidence for
unknown-content preservation. Missing parts, broken relationships, dropped
extensions, or unrelated node changes fail the preservation gate. If a strategy
rewrites unrelated XML, record the exact differences and why the stronger gate
failed; do not silently normalize them away to obtain a pass.

For native reopen, require no repair prompt, expected target text, intact
surrounding features, and no missing images or unexplained formatting changes.
Compare baseline and output with the same Word version, fonts, OS, and rendering
settings. For no-op and same-length edits require unchanged page count and no
unexplained change outside the target region. Declare region masks and any
raster tolerance before examining outputs. Long-text reflow is assessed
separately for correctness, not forced into a pixel-identical comparison.

## Resource and Failure Boundaries

Before opening untrusted fixtures, choose and record hard budgets for input
bytes, expanded bytes, entry count, XML depth, text size, and execution time.
Reject archive traversal, ambiguous duplicate entries, invalid package
relationships, encrypted inputs, and external entity resolution. Do not fetch
external relationships or execute embedded content. Test budget boundaries.

Persistence uses a new destination during the experiment. Simulate write and
replacement failures; retain the original and do not report a committed result.
A caller-requested destination that already exists must remain unchanged.
Explicitly report unsupported format variants instead of guessing compatibility.

## Decision Gates and Deliverables

| Gate | Pass condition | Failure consequence |
| --- | --- | --- |
| D-G1 Package preservation | No-op and unrelated content pass structural and hash checks | Reject strategy for native editing or narrow scope explicitly |
| D-G2 Supported mutation | Both edit operations affect only declared targets across D-01 through D-04 | Fix targeting or reduce accepted structures before UI work |
| D-G3 Native reopen | Word opens outputs without repair and visual/semantic checks pass | No Word compatibility claim; unavailable check stays blocked |
| D-G4 Failure behavior | Invalid/stale batches and persistence failures leave no partial accepted change | Block writable adapter integration |
| D-G5 Basic creation | New document reopens; destination collision is non-destructive | Keep creation unavailable even if editing passes |
| D-G6 Integration readiness | License, limits, typed contracts, and reproducible evidence documented | Keep prototype isolated; no shared runtime extraction |

Produce a Markdown experiment report with candidate versions/licenses, fixture
manifest, commands, expected and actual results, per-gate pass/fail/blocked
status, structural differences, native screenshots, performance observations,
and remaining limits. Store private artifacts outside Git; commit only safe
fixtures and reproducible checks. No experiment results exist yet.

If gates pass, write an ADR selecting persistence and a bounded editor surface,
then implement DOCX inspection/editing through the real transport and browser
entry points. Validate a human edit followed by an AI edit, stale-revision
rejection, persistence, and native reopening. Extract common session behavior
only after these real XLSX and DOCX consumers demonstrate the contract.
