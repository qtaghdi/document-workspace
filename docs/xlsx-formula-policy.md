# XLSX Formula Policy

## Capability Boundary

document-workspace stores and preserves XLSX formula expressions. The Go service
does not calculate formula results. A value returned beside a formula is either
a cached value from the source XLSX package or unavailable. It is never labeled
as a result calculated by the service.

The browser uses Univer's open-source formula engine to provide an interactive
preview. That preview is useful while editing, but it is not authoritative for
Microsoft Excel compatibility. Function support, dynamic arrays, external
references, locale behavior, dates, precision, and calculation order can differ.

After an edit that can affect formulas, the service writes XLSX calculation
properties requesting a full native recalculation on load and on save. It marks
calculation for a forced full pass. Microsoft Excel or another native
spreadsheet application remains responsible for producing authoritative cached
results.

## API Contract

Workbook snapshots expose `formulaPolicy`:

- `storage: preserved`
- `serverCalculation: none`
- `browserCalculation: preview`
- `nativeRecalculation: requested_after_formula_affecting_edits`

Formula cells expose `formulaValueStatus`. `cached` means the value came from
the source package. `unavailable` means no cached source value was available.
The status does not validate correctness or freshness.

Edits that request native recalculation are cell values, formulas, rectangular
paste, and row or column insertion or deletion in a workbook known to contain a
formula. Adding a formula also enables the policy and warning for that session.
Formatting, merge, image, and chart-only changes do not request recalculation.

## User Experience

When a workbook contains formulas, the compatibility notice explains that
browser results are previews. AI clients receive the same policy and formula
value status through structured tool results. Product copy must not describe a
displayed browser result or cached source value as verified Excel calculation.

After editing formulas or their dependencies, reopen the saved workbook in the
target spreadsheet application and allow it to recalculate before relying on
results. External links may remain unavailable until the native application can
resolve them under the user's security settings.

## Verification

Automated tests verify expression round-trip, formula value status, policy
metadata, calculation properties after dependency changes, warnings for newly
added formulas, and preservation through the compatibility corpus. These tests
do not establish Microsoft Excel calculation equivalence.

The remaining release evidence is a Microsoft Excel-produced fixture and native
Excel reopening of changed formulas, dependencies, unsupported functions,
external references, and stale cached values. Record the Excel version and
observed recalculation behavior in the release evidence.
