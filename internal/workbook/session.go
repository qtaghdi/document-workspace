package workbook

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/xuri/excelize/v2"
)

var ErrRevisionConflict = errors.New("workbook revision conflict")

const maxRangeCells = 10_000

type Cell struct {
	Address string `json:"address"`
	Value   string `json:"value"`
	Formula string `json:"formula,omitempty"`
}

type Range struct {
	Sheet string   `json:"sheet"`
	Ref   string   `json:"ref"`
	Rows  [][]Cell `json:"rows"`
}

type Operation struct {
	Type    string `json:"type" jsonschema:"Operation type: set_cell or set_formula"`
	Sheet   string `json:"sheet" jsonschema:"Worksheet name"`
	Cell    string `json:"cell" jsonschema:"A1-style cell address"`
	Value   any    `json:"value,omitempty" jsonschema:"Cell value for set_cell"`
	Formula string `json:"formula,omitempty" jsonschema:"Formula for set_formula, with or without a leading equals sign"`
}

type Event struct {
	Sequence uint64 `json:"sequence"`
	Revision uint64 `json:"revision"`
	Actor    string `json:"actor"`
	Type     string `json:"type"`
	Sheet    string `json:"sheet,omitempty"`
	Cell     string `json:"cell,omitempty"`
	Text     string `json:"text,omitempty"`
}

type Snapshot struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Sheets   []string `json:"sheets"`
	Revision uint64   `json:"revision"`
}

type Session struct {
	mu          sync.RWMutex
	id          string
	path        string
	file        *excelize.File
	revision    uint64
	sequence    uint64
	subscribers map[chan Event]struct{}
}

func Open(path string) (*Session, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve workbook path: %w", err)
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

func (s *Session) ReadRange(sheet, ref string) (Range, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.hasSheet(sheet) {
		return Range{}, fmt.Errorf("unknown sheet %q", sheet)
	}
	parts := strings.Split(strings.ToUpper(strings.TrimSpace(ref)), ":")
	if len(parts) == 1 {
		parts = append(parts, parts[0])
	}
	if len(parts) != 2 {
		return Range{}, fmt.Errorf("invalid range %q", ref)
	}
	c1, r1, err := excelize.CellNameToCoordinates(parts[0])
	if err != nil {
		return Range{}, fmt.Errorf("invalid range %q", ref)
	}
	c2, r2, err := excelize.CellNameToCoordinates(parts[1])
	if err != nil || c1 > c2 || r1 > r2 {
		return Range{}, fmt.Errorf("invalid range %q", ref)
	}
	if (c2-c1+1)*(r2-r1+1) > maxRangeCells {
		return Range{}, fmt.Errorf("range exceeds %d cells", maxRangeCells)
	}
	rows := make([][]Cell, 0, r2-r1+1)
	for row := r1; row <= r2; row++ {
		cells := make([]Cell, 0, c2-c1+1)
		for col := c1; col <= c2; col++ {
			address, _ := excelize.CoordinatesToCellName(col, row)
			value, valueErr := s.file.GetCellValue(sheet, address)
			if valueErr != nil {
				return Range{}, fmt.Errorf("read %s!%s: %w", sheet, address, valueErr)
			}
			formula, formulaErr := s.file.GetCellFormula(sheet, address)
			if formulaErr != nil {
				return Range{}, fmt.Errorf("read formula %s!%s: %w", sheet, address, formulaErr)
			}
			cells = append(cells, Cell{Address: address, Value: value, Formula: formula})
		}
		rows = append(rows, cells)
	}
	return Range{Sheet: sheet, Ref: ref, Rows: rows}, nil
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
	for i, op := range operations {
		if err := s.validateOperation(op); err != nil {
			return Snapshot{}, fmt.Errorf("operation %d: %w", i, err)
		}
	}

	nextRevision := s.revision + 1
	for _, op := range operations {
		text := fmt.Sprint(op.Value)
		if op.Type == "set_formula" {
			text = op.Formula
			if len(text) > 0 && text[0] == '=' {
				text = text[1:]
			}
		}
		s.publishLocked(Event{Revision: nextRevision, Actor: actor, Type: "cursor.move", Sheet: op.Sheet, Cell: op.Cell})
		s.publishLocked(Event{Revision: nextRevision, Actor: actor, Type: "cell.typing", Sheet: op.Sheet, Cell: op.Cell, Text: text})
		var err error
		switch op.Type {
		case "set_cell":
			err = s.file.SetCellValue(op.Sheet, op.Cell, op.Value)
		case "set_formula":
			err = s.file.SetCellFormula(op.Sheet, op.Cell, text)
		}
		if err != nil {
			s.reloadLocked()
			return Snapshot{}, fmt.Errorf("apply %s to %s!%s: %w", op.Type, op.Sheet, op.Cell, err)
		}
	}
	if err := s.persistLocked(); err != nil {
		s.reloadLocked()
		return Snapshot{}, err
	}
	s.revision = nextRevision
	for _, op := range operations {
		text := fmt.Sprint(op.Value)
		if op.Type == "set_formula" {
			text = op.Formula
		}
		s.publishLocked(Event{Revision: s.revision, Actor: actor, Type: "cell.commit", Sheet: op.Sheet, Cell: op.Cell, Text: text})
	}
	return s.snapshotLocked(), nil
}

func (s *Session) Subscribe() (<-chan Event, func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch := make(chan Event, 64)
	s.subscribers[ch] = struct{}{}
	return ch, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if _, ok := s.subscribers[ch]; ok {
			delete(s.subscribers, ch)
			close(ch)
		}
	}
}

func (s *Session) validateOperation(op Operation) error {
	if op.Type != "set_cell" && op.Type != "set_formula" {
		return fmt.Errorf("unsupported type %q", op.Type)
	}
	if !s.hasSheet(op.Sheet) {
		return fmt.Errorf("unknown sheet %q", op.Sheet)
	}
	if _, _, err := excelize.CellNameToCoordinates(op.Cell); err != nil {
		return fmt.Errorf("invalid cell %q", op.Cell)
	}
	if op.Type == "set_formula" && op.Formula == "" {
		return errors.New("formula is required")
	}
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

func (s *Session) publishLocked(event Event) {
	s.sequence++
	event.Sequence = s.sequence
	for ch := range s.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

func (s *Session) persistLocked() error {
	buffer, err := s.file.WriteToBuffer()
	if err != nil {
		return fmt.Errorf("serialize workbook: %w", err)
	}
	dir := filepath.Dir(s.path)
	tmp, err := os.CreateTemp(dir, ".xlsx-viewer-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary workbook: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if info, statErr := os.Stat(s.path); statErr == nil {
		_ = tmp.Chmod(info.Mode())
	}
	if _, err = tmp.Write(buffer.Bytes()); err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("write temporary workbook: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("replace workbook atomically: %w", err)
	}
	return nil
}

func (s *Session) reloadLocked() {
	reloaded, err := excelize.OpenFile(s.path)
	if err != nil {
		return
	}
	_ = s.file.Close()
	s.file = reloaded
}

func randomID() string {
	var value [8]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "workbook"
	}
	return hex.EncodeToString(value[:])
}
