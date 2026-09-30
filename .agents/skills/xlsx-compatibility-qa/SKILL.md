---
name: xlsx-compatibility-qa
description: Verify that xlsx-viewer opens, edits, saves, and reopens representative XLSX files without losing unrelated workbook features. Use when changing workbook parsing, mutation, persistence, Excelize, or compatibility warnings.
---

# XLSX Compatibility QA

Use this workflow for changes that can alter XLSX package contents or workbook
semantics. Do not use it for isolated UI styling or documentation changes.

## Required Inputs

- Read `AGENTS.md`, `docs/architecture.md`, and the affected workbook code.
- Inspect `testdata/compatibility/README.md` for fixture provenance, covered
  features, and known gaps.
- Identify which existing fixture and feature assertions exercise the change.
  Add a focused fixture only when the current corpus cannot represent it.

## Workflow

1. Copy each source fixture to a temporary directory. Never mutate the committed
   corpus in place.
2. Inventory the features relevant to the change before opening a workbook
   session.
3. Apply an edit to a cell unrelated to the inventoried features through the
   real `workbook.Session` boundary.
4. Close and reopen the saved XLSX package.
5. Compare formulas, styles, merged ranges, validation, conditional formatting,
   drawings, links, names, comments, and other relevant package parts.
6. Treat missing or changed unrelated features as a compatibility failure. Do
   not update expectations merely to make a failing test pass.

## Safety Constraints

- Use disposable copies for every mutation test.
- Do not claim lossless compatibility from a successful open or a cell-value
  check alone.
- Distinguish formula storage from formula calculation.
- Record fixture provenance and generator versions. A generated fixture is a
  baseline, not proof of compatibility with every Excel producer.
- Keep private or customer workbooks out of the repository.
- When a feature cannot be preserved, keep the failing evidence and implement a
  warning before allowing a silent save path.

## Verification

Run the focused corpus check first:

```bash
make compatibility-test
```

For a completed workbook change, also run `make verify`. Confirm no temporary
workbooks are staged and new committed text contains no em dash characters.

## Expected Output

Report the fixtures exercised, features compared, edits applied, reopen result,
known coverage gaps, and any save warning or compatibility regression found.
