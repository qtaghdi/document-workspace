package workbook

import (
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

func Open(path string) (*Session, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve workbook path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("inspect workbook: %w", err)
	}
	if info.Size() > maxWorkbookBytes {
		return nil, fmt.Errorf("workbook exceeds %d bytes", maxWorkbookBytes)
	}
	f, err := excelize.OpenFile(abs)
	if err != nil {
		return nil, fmt.Errorf("open workbook: %w", err)
	}
	return &Session{
		id:          randomID(),
		path:        abs,
		file:        f,
		revision:    1,
		subscribers: make(map[chan Event]struct{}),
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
	return Snapshot{
		ID:       s.id,
		Name:     filepath.Base(s.path),
		Sheets:   append([]string(nil), s.file.GetSheetList()...),
		Revision: s.revision,
	}
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

	nextRevision := s.revision + 1
	for _, op := range operations {
		selection := op.Cell
		if op.Range != "" {
			selection = op.Range
		}
		s.publishLocked(Event{Revision: nextRevision, Actor: actor, Type: "presence.update", Sheet: op.Sheet, Range: selection, State: "editing"})
		if op.Type == "set_format" {
			if err := s.applyFormatLocked(op); err != nil {
				s.reloadLocked()
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
				s.reloadLocked()
				return Snapshot{}, fmt.Errorf("apply %s to %s!%s: %w", op.Type, op.Sheet, op.Range, err)
			}
			continue
		}
		changes, _ := s.operationChanges(op)
		if len(changes) == 1 {
			s.publishLocked(Event{Revision: nextRevision, Actor: actor, Type: "cell.typing", Sheet: op.Sheet, Cell: changes[0].Cell, Text: changeText(changes[0])})
		}
		for _, change := range changes {
			var err error
			if change.Formula != "" {
				err = s.file.SetCellFormula(op.Sheet, change.Cell, normalizeFormula(change.Formula))
			} else {
				err = s.file.SetCellValue(op.Sheet, change.Cell, change.Value)
			}
			if err != nil {
				s.reloadLocked()
				return Snapshot{}, fmt.Errorf("apply %s to %s!%s: %w", op.Type, op.Sheet, change.Cell, err)
			}
		}
	}
	if err := s.persistLocked(); err != nil {
		s.reloadLocked()
		return Snapshot{}, err
	}
	s.revision = nextRevision
	for _, op := range operations {
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
	return Snapshot{ID: s.id, Name: filepath.Base(s.path), Sheets: append([]string(nil), s.file.GetSheetList()...), Revision: s.revision}
}
