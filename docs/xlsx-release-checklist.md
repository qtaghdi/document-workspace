# XLSX Stable Release Checklist

Status: release planning, not a release certification.
Baseline: implementation `4a6cfeb`, shared specification `e79e7e0`.
Requirements refer to the [shared specification](product-spec.md).
Capabilities remain bounded by the [format matrix](format-support.md).

## Release Scope

The first stable milestone supports one existing local XLSX workbook per
process, human and AI edits through the same revisioned session, and native
XLSX persistence. Include cells, formula storage, basic formatting, rectangular
paste, merges, row and column operations, existing image and chart placement,
resize and deletion, AI chart title updates, and bounded undo and redo.

Exclude blank creation, templates, format conversion, hosted collaboration,
staged proposal review, macro execution, encrypted editing, image insertion,
advanced chart authoring, and complete Excel formula calculation. Image
insertion and advanced chart editing remain planned follow-ups. This explicitly
narrows the stable milestone from the earlier object-editing backlog; it does
not remove those features from the roadmap.

Formula policy: preserve formula expressions for supported edits and distinguish
stored or cached values from browser-calculated results. Do not describe browser
results as authoritative Excel calculation. Before release, verify the behavior
of changed formulas, changed dependencies, unsupported functions, external
references, and stale caches, including reopening in Excel. The release must
document when recalculation is required and show a limitation where a result
cannot be established. No server calculation engine is selected by this spec.

## Evidence Status

Recovery implementation now has [dedicated tests and an operator guide](xlsx-recovery.md).
X-03 through X-05 have implementation evidence for candidate rollback, failure
injection, source hash guards, cooperating writer locks, replay reset, and paused
browser queues. They remain candidate sign-off gates, with the external-writer
race and crash-durability limits documented in that guide.

Existing evidence means a test or historical record exists, not that this
release candidate has passed it. Every checkbox remains open until evidence is
recorded against the candidate commit. A documentation-only change does not
require rerunning the application suite.

| Gate | Requirements | Existing evidence | Remaining closure condition |
| --- | --- | --- | --- |
| X-01 Input boundaries | DOC-01 | Size checks in `xlsx.Open`; parser errors | Reject corrupt, oversized, restricted, and unsupported inputs without source mutation; declare bounded expanded-package handling |
| X-02 Transactions | EDIT-01, EDIT-03, SAVE-01 | Persistence, stale-revision, paste-shape, and shared-session tests | Mixed valid/invalid batch; simultaneous same-revision writes; verify content, revision, and commit events |
| X-03 Persistence failures | SAVE-01, REC-01 | Same-directory temporary write and rename | Inject temporary creation, write, sync, and rename failures; verify disk and memory recovery; no false committed event |
| X-04 External writers | REC-01 | History reset test after external replacement | Detect changes during a live session before overwrite; define remaining race and single-writer policy |
| X-05 Disconnect and retry | REC-01 | Bounded event replay | Test lost response after commit, exhausted replay, slow consumer, and reconnect; reconcile before retry without duplicating edits |
| X-06 History | HIST-01 | Undo/redo, restart, and replacement tests | Test snapshot size/count limits and unavailable history in UI; confirm monotonically increasing revision |
| X-07 Native fidelity | COMP-01 | Generated and LibreOffice corpus; object round-trip tests | Add provenance-recorded Excel fixture; verify native reopen and unrelated content across all supported edit families |
| X-08 Formula expectations | COMP-01 | Formula storage and read tests | Verify and expose formula policy above; no unsupported result presented as verified calculation |
| X-09 Host workflows | HOST-01, AUTH-01 | Historical tools and Codex browser fallback records | Candidate-level Claude and Codex tests with explicit mode, versions, permissions, human/AI edits, conflict, and native reopen |
| X-10 UI and limits | EDIT-02, COMP-01 | Virtualized loading, presence, compatibility notices | Keyboard editing, paste, history, reduced motion, laptop viewport, and mobile fallback; visible pending/conflict/failure states |
| X-11 Access and diagnostics | AUTH-01 | Separate tokens, loopback, origin checks | Negative access tests and review of logs, limits, external resources, and launch-token handling |
| X-12 Packaging | HOST-01 | Build and transport tests | Fresh checkout build; release binary serves both bundles; absolute-path host configuration works |

Source anchors: [session tests](../internal/formats/xlsx/session_test.go),
[corpus tests](../internal/formats/xlsx/compatibility_test.go),
[persistence](../internal/formats/xlsx/persistence.go), and
[transport tests](../internal/transport/httpapi/server_test.go).

## Safety Gates That Cannot Be Waived by a Warning

Silent source loss, partial commits, stale writes accepted as current, and false
persistence success block release. If a feature cannot meet a gate, disable its
write path or restrict the supported mode and update the scope and matrix.
A warning alone does not make unsafe overwrite acceptable.

For X-04, a file hash check alone does not eliminate the race between checking
and replacement. Record the concurrency strategy and test the promised boundary.
Do not claim coordination with Excel or another process without evidence.
For X-03, include the history manifest and rollback/reload failure paths. A
successful file rename does not alone prove crash-consistent session history.
For X-11, separate deliberate credential delivery at startup from routine logs;
record any remediation needed before publishing a release binary.

## Candidate Verification Procedure

1. Record commit, OS, Go and pnpm versions, dependency lockfile, and build mode.
2. From a clean checkout, run `make web-install`, `make verify`,
   `make compatibility-test`, and `make go-build`. The verify target builds the
   browser bundles and runs Go tests, race tests, vet, and Python regression.
3. Exercise missing failure scenarios in the gate table with reproducible
   tests. Record failures as open gates, not as expected passes.
4. On disposable corpus copies, perform human and AI edits and reopen the saved
   outputs in Excel and LibreOffice. Record application versions and differences.
5. Execute the [host procedure](host-compatibility.md) for each claimed mode.
   Native MCP App rendering and browser fallback are separate results. If a
   host only passes fallback, advertise fallback explicitly. Claude's UI path
   remains pending in the historical record; neither host has certified native
   embedded rendering there.
6. Capture UI evidence at desktop, small laptop, and mobile fallback sizes.
   Record actual viewport sizes, keyboard behavior, and reduced-motion behavior.
7. Reconcile README, format matrix, host matrix, release limitations, and dated
   changelog. Keep private workbooks, credentials, and temporary reports out of Git.

Known configured bounds are 100 MiB input size, 10,000 cells per bounded range,
1,000 operations per batch, 512 retained events, ten history snapshots, and
32 MiB per retained history snapshot. These are code limits, not demonstrated
performance capacity. Benchmark representative files and record open latency,
edit latency, peak memory, fixture hash, and machine details before making
performance claims. Establish acceptable budgets before evaluating the results.

## Sign-Off Record

- [ ] X-01 through X-12 closed or explicitly restricted with a safe tested mode.
- [ ] Microsoft Excel fixture provenance and native reopen evidence recorded.
- [ ] No unexplained loss of unrelated workbook features.
- [ ] Claimed Claude and Codex modes certified against the candidate.
- [ ] Formula behavior and history/resource limits match product copy.
- [ ] No open source-loss, authorization, or false-success defect.
- [ ] Candidate checks pass and release documentation is consistent.

For each gate record: gate ID, candidate commit, date, reviewer, fixture hash,
command or manual procedure, expected result, observed result, evidence path,
and pass/fail/blocked status. A blocked native application check remains blocked;
a package parser reopen must not substitute for Excel verification.

## Implementation Order

1. Resolve X-03 through X-05 recovery and overwrite gaps with focused tests.
2. Complete Excel corpus coverage and formula policy checks.
3. Close candidate host, UI, access, and packaging gates.
4. Publish only the verified scope; schedule image insertion and advanced chart
   authoring as separate follow-ups with their own fidelity tests.
