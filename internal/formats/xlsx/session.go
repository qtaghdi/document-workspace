package xlsx

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"
)

var ErrRevisionConflict = errors.New("workbook revision conflict")

const maxRangeCells = 10_000
const maxOperations = 1_000
const maxEventHistory = 512
const maxWorkbookBytes = 100 << 20
const maxHistoryEntries = 10
const maxHistorySnapshotBytes = 32 << 20

func Open(path string) (*Session, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve workbook path: %w", err)
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, fmt.Errorf("resolve workbook target: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("inspect workbook: %w", err)
	}
	if info.Size() > maxWorkbookBytes {
		return nil, fmt.Errorf("workbook exceeds %d bytes", maxWorkbookBytes)
	}
	current, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("read workbook: %w", err)
	}
	historyStore, revision, undoHistory, redoHistory, err := loadHistoryStore(abs, current)
	if err != nil {
		return nil, err
	}
	f, err := excelize.OpenReader(bytes.NewReader(current))
	if err != nil {
		return nil, fmt.Errorf("open workbook: %w", err)
	}
	dimensions, err := inspectSheetDimensions(f)
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	warnings, hasFormulas, err := inspectFeatureWarnings(abs)
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	return &Session{
		currentHash:  hashBytes(current),
		persistence:  diskPersistence(),
		id:           randomID(),
		path:         abs,
		file:         f,
		revision:     revision,
		subscribers:  make(map[chan Event]struct{}),
		dimensions:   dimensions,
		warnings:     warnings,
		undoHistory:  undoHistory,
		redoHistory:  redoHistory,
		historyStore: historyStore,
		hasFormulas:  hasFormulas,
	}, nil
}

func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range s.subscribers {
		close(ch)
		delete(s.subscribers, ch)
	}
	return s.file.Close()
}

func (s *Session) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshotLocked()
}

func (s *Session) Apply(baseRevision uint64, actor string, operations []Operation) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if baseRevision != s.revision {
		return Snapshot{}, fmt.Errorf("%w: expected %d, got %d", ErrRevisionConflict, s.revision, baseRevision)
	}
	if len(operations) == 0 {
		return s.snapshotLocked(), nil
	}
	if len(operations) > maxOperations {
		return Snapshot{}, fmt.Errorf("operation count exceeds %d", maxOperations)
	}
	for i, op := range operations {
		if err := s.validateOperation(op); err != nil {
			return Snapshot{}, fmt.Errorf("operation %d: %w", i, err)
		}
	}
	unlock, err := s.acquireWriteLock()
	if err != nil {
		return Snapshot{}, err
	}
	defer unlock()
	previous, historyErr := s.readCurrentLocked()
	if historyErr != nil {
		return Snapshot{}, fmt.Errorf("capture workbook history: %w", historyErr)
	}
	if err := s.reconcileHistoryLocked(); err != nil {
		return Snapshot{}, err
	}
	// Mutate a candidate, never the authoritative in-memory file. Rollback must
	// not depend on a disk reload that can fail or import an external edit.
	candidate, err := excelize.OpenReader(bytes.NewReader(previous))
	if err != nil {
		return Snapshot{}, fmt.Errorf("prepare workbook transaction: %w", err)
	}
	original := s.file
	s.file = candidate
	committed := false
	defer func() {
		if committed {
			_ = original.Close()
		} else {
			s.file = original
			_ = candidate.Close()
		}
	}()

	nextRevision := s.revision + 1
	for _, op := range operations {
		selection := op.Cell
		if op.Range != "" {
			selection = op.Range
		}
		if isStructuralOperation(op.Type) {
			selection = structuralSelection(op)
		}
		if isObjectOperation(op.Type) && op.TargetCell != "" {
			selection = op.TargetCell
		}
		s.publishLocked(Event{Revision: s.revision, Actor: actor, Type: "presence.update", Sheet: op.Sheet, Range: selection, State: "editing"})
		if isObjectOperation(op.Type) {
			if err := s.applyObjectOperationLocked(op); err != nil {
				return Snapshot{}, err
			}
			continue
		}
		if isStructuralOperation(op.Type) {
			if err := s.applyStructuralOperationLocked(op); err != nil {
				return Snapshot{}, err
			}
			continue
		}
		if op.Type == "set_format" {
			if err := s.applyFormatLocked(op); err != nil {
				return Snapshot{}, err
			}
			continue
		}
		if op.Type == "merge_cells" || op.Type == "unmerge_cells" {
			startCol, startRow, width, height, _ := parseRange(op.Range)
			start, _ := excelize.CoordinatesToCellName(startCol, startRow)
			end, _ := excelize.CoordinatesToCellName(startCol+width-1, startRow+height-1)
			var err error
			if op.Type == "merge_cells" {
				err = s.file.MergeCell(op.Sheet, start, end)
			} else {
				err = s.file.UnmergeCell(op.Sheet, start, end)
			}
			if err != nil {
				return Snapshot{}, fmt.Errorf("apply %s to %s!%s: %w", op.Type, op.Sheet, op.Range, err)
			}
			continue
		}
		changes, _ := s.operationChanges(op)
		if len(changes) == 1 {
			s.publishLocked(Event{Revision: s.revision, Actor: actor, Type: "cell.typing", Sheet: op.Sheet, Cell: changes[0].Cell, Text: changeText(changes[0])})
		}
		for _, change := range changes {
			var err error
			if change.Formula != "" {
				err = s.file.SetCellFormula(op.Sheet, change.Cell, normalizeFormula(change.Formula))
			} else {
				err = s.file.SetCellValue(op.Sheet, change.Cell, change.Value)
			}
			if err != nil {
				return Snapshot{}, fmt.Errorf("apply %s to %s!%s: %w", op.Type, op.Sheet, change.Cell, err)
			}
		}
	}
	var structuralDimensions map[string]SheetDimensions
	if containsStructuralOperation(operations) {
		var dimensionsErr error
		structuralDimensions, dimensionsErr = inspectSheetDimensions(s.file)
		if dimensionsErr != nil {
			return Snapshot{}, dimensionsErr
		}
	}
	formulaWorkbook := s.hasFormulas || introducesFormula(operations)
	if formulaWorkbook && affectsFormulaResults(operations) {
		if err := requestNativeRecalculation(s.file); err != nil {
			return Snapshot{}, err
		}
	}
	nextContent, err := s.serializeLocked()
	if err != nil {
		return Snapshot{}, err
	}
	nextUndo := appendBoundedHistory(append([][]byte(nil), s.undoHistory...), previous)
	nextRedo := [][]byte(nil)
	if err := s.historyStore.prepare(nextRevision, nextContent, nextUndo, nextRedo); err != nil {
		return Snapshot{}, err
	}
	if err := s.replaceBytesLocked(nextContent); err != nil {
		s.historyStore.abort()
		return Snapshot{}, err
	}
	historyCommitErr := s.historyStore.commit()
	committed = true
	s.revision = nextRevision
	s.hasFormulas = formulaWorkbook
	if introducesFormula(operations) {
		s.ensureFormulaWarningLocked()
	}
	s.undoHistory = nextUndo
	s.redoHistory = nextRedo
	if historyCommitErr != nil {
		s.addHistoryWarningLocked(historyCommitErr)
	}
	if structuralDimensions != nil {
		s.dimensions = structuralDimensions
	} else {
		s.growDimensionsLocked(operations)
	}
	for _, op := range operations {
		if isObjectOperation(op.Type) {
			s.publishLocked(Event{Revision: s.revision, Actor: actor, Type: "workbook.reload", Sheet: op.Sheet, State: "objects"})
			continue
		}
		if isStructuralOperation(op.Type) {
			s.publishLocked(Event{Revision: s.revision, Actor: actor, Type: "sheet." + op.Type, Sheet: op.Sheet, Index: op.Index, Count: op.Count})
			continue
		}
		if op.Type == "set_format" {
			s.publishLocked(Event{Revision: s.revision, Actor: actor, Type: "range.format", Sheet: op.Sheet, Range: op.Range, Format: op.Format})
			continue
		}
		if op.Type == "merge_cells" || op.Type == "unmerge_cells" {
			s.publishLocked(Event{Revision: s.revision, Actor: actor, Type: "range." + op.Type, Sheet: op.Sheet, Range: op.Range})
			continue
		}
		changes, _ := s.operationChanges(op)
		if op.Type == "paste_range" {
			s.publishLocked(Event{Revision: s.revision, Actor: actor, Type: "range.commit", Sheet: op.Sheet, Range: op.Range, Cells: changes})
			continue
		}
		s.publishLocked(Event{Revision: s.revision, Actor: actor, Type: "cell.commit", Sheet: op.Sheet, Cell: op.Cell, Text: changeText(changes[0]), Cells: changes})
	}
	return s.snapshotLocked(), nil
}

func (s *Session) RestoreHistory(baseRevision uint64, actor, direction string) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if baseRevision != s.revision {
		return Snapshot{}, fmt.Errorf("%w: expected %d, got %d", ErrRevisionConflict, s.revision, baseRevision)
	}
	var source *[][]byte
	var destination *[][]byte
	switch direction {
	case "undo":
		source, destination = &s.undoHistory, &s.redoHistory
	case "redo":
		source, destination = &s.redoHistory, &s.undoHistory
	default:
		return Snapshot{}, fmt.Errorf("unsupported history direction %q", direction)
	}
	if len(*source) == 0 {
		return Snapshot{}, fmt.Errorf("nothing to %s", direction)
	}
	unlock, err := s.acquireWriteLock()
	if err != nil {
		return Snapshot{}, err
	}
	defer unlock()
	current, err := s.readCurrentLocked()
	if err != nil {
		return Snapshot{}, fmt.Errorf("capture current workbook for %s: %w", direction, err)
	}
	if err := s.reconcileHistoryLocked(); err != nil {
		return Snapshot{}, err
	}
	targetIndex := len(*source) - 1
	target := (*source)[targetIndex]
	candidate, err := excelize.OpenReader(bytes.NewReader(target))
	if err != nil {
		return Snapshot{}, fmt.Errorf("prepare history restore: %w", err)
	}
	dimensions, err := inspectSheetDimensions(candidate)
	if err != nil {
		_ = candidate.Close()
		return Snapshot{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = candidate.Close()
		}
	}()
	nextSource := append([][]byte(nil), (*source)[:targetIndex]...)
	nextDestination := appendBoundedHistory(append([][]byte(nil), (*destination)...), current)
	nextRevision := s.revision + 1
	var nextUndo, nextRedo [][]byte
	if direction == "undo" {
		nextUndo, nextRedo = nextSource, nextDestination
	} else {
		nextUndo, nextRedo = nextDestination, nextSource
	}
	if err := s.historyStore.prepare(nextRevision, target, nextUndo, nextRedo); err != nil {
		return Snapshot{}, err
	}
	if err := s.replaceBytesLocked(target); err != nil {
		s.historyStore.abort()
		return Snapshot{}, err
	}
	historyCommitErr := s.historyStore.commit()
	_ = s.file.Close()
	s.file = candidate
	committed = true
	s.dimensions = dimensions
	s.undoHistory = nextUndo
	s.redoHistory = nextRedo
	s.revision = nextRevision
	if historyCommitErr != nil {
		s.addHistoryWarningLocked(historyCommitErr)
	}
	s.publishLocked(Event{Revision: s.revision, Actor: actor, Type: "workbook.reload", State: direction})
	return s.snapshotLocked(), nil
}

func appendBoundedHistory(history [][]byte, snapshot []byte) [][]byte {
	if len(snapshot) > maxHistorySnapshotBytes {
		return history
	}
	history = append(history, snapshot)
	if len(history) > maxHistoryEntries {
		history = append([][]byte(nil), history[len(history)-maxHistoryEntries:]...)
	}
	return history
}

func (s *Session) addHistoryWarningLocked(_ error) {
	for _, warning := range s.warnings {
		if warning.Feature == "durable history" {
			return
		}
	}
	s.warnings = append(s.warnings, FeatureWarning{
		Feature: "durable history",
		Message: "the workbook was saved, but durable undo history could not be finalized; restart the session to recover it",
	})
}

func structuralSelection(operation Operation) string {
	if operation.Type == "insert_columns" || operation.Type == "delete_columns" {
		column, err := excelize.ColumnNumberToName(operation.Index)
		if err == nil {
			return column + "1"
		}
	}
	return fmt.Sprintf("A%d", operation.Index)
}

func containsStructuralOperation(operations []Operation) bool {
	for _, operation := range operations {
		if isStructuralOperation(operation.Type) {
			return true
		}
	}
	return false
}

func (s *Session) UpdatePresence(actor string, presence Presence) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.hasSheet(presence.Sheet) {
		return fmt.Errorf("unknown sheet %q", presence.Sheet)
	}
	if _, _, _, _, err := parseRange(presence.Range); err != nil {
		return err
	}
	switch presence.State {
	case "", "selecting", "editing", "idle":
	default:
		return fmt.Errorf("unsupported presence state %q", presence.State)
	}
	s.publishLocked(Event{Revision: s.revision, Actor: actor, Type: "presence.update", Sheet: presence.Sheet, Range: strings.ToUpper(presence.Range), State: presence.State})
	return nil
}

func (s *Session) hasSheet(name string) bool {
	for _, sheet := range s.file.GetSheetList() {
		if sheet == name {
			return true
		}
	}
	return false
}

func (s *Session) snapshotLocked() Snapshot {
	return Snapshot{
		Sequence:        s.sequence,
		ID:              s.id,
		Name:            filepath.Base(s.path),
		Sheets:          append([]string(nil), s.file.GetSheetList()...),
		SheetDimensions: s.sheetDimensionsLocked(),
		Warnings:        append([]FeatureWarning(nil), s.warnings...),
		CanUndo:         len(s.undoHistory) > 0,
		CanRedo:         len(s.redoHistory) > 0,
		Revision:        s.revision,
		FormulaPolicy: FormulaPolicy{
			Storage:             "preserved",
			ServerCalculation:   "none",
			BrowserCalculation:  "preview",
			NativeRecalculation: "requested_after_formula_affecting_edits",
		},
	}
}

func introducesFormula(operations []Operation) bool {
	for _, operation := range operations {
		if operation.Type == "set_formula" {
			return true
		}
		for _, row := range operation.Cells {
			for _, cell := range row {
				if cell.Formula != "" {
					return true
				}
			}
		}
	}
	return false
}

func affectsFormulaResults(operations []Operation) bool {
	for _, operation := range operations {
		switch operation.Type {
		case "set_cell", "set_formula", "paste_range", "insert_rows", "delete_rows", "insert_columns", "delete_columns":
			return true
		}
	}
	return false
}

func requestNativeRecalculation(file *excelize.File) error {
	yes := true
	if err := file.SetCalcProps(&excelize.CalcPropsOptions{
		FullCalcOnLoad: &yes,
		CalcOnSave:     &yes,
		ForceFullCalc:  &yes,
	}); err != nil {
		return fmt.Errorf("request native formula recalculation: %w", err)
	}
	return nil
}

func (s *Session) ensureFormulaWarningLocked() {
	for _, warning := range s.warnings {
		if warning.Feature == "formula calculation" {
			return
		}
	}
	s.warnings = append([]FeatureWarning{{
		Feature: "formula calculation",
		Message: "formula expressions are stored, but server values are cached or unavailable; browser results are previews until a native spreadsheet application recalculates the workbook",
	}}, s.warnings...)
}
