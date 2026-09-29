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
const maxOperations = 1_000
const maxEventHistory = 512
const maxWorkbookBytes = 100 << 20

type Cell struct {
	Address string     `json:"address"`
	Value   string     `json:"value"`
	Formula string     `json:"formula,omitempty"`
	Style   *CellStyle `json:"style,omitempty"`
}

type CellStyle struct {
	Bold         bool    `json:"bold,omitempty"`
	Italic       bool    `json:"italic,omitempty"`
	FontFamily   string  `json:"fontFamily,omitempty"`
	FontSize     float64 `json:"fontSize,omitempty"`
	FontColor    string  `json:"fontColor,omitempty"`
	FillColor    string  `json:"fillColor,omitempty"`
	NumberFormat string  `json:"numberFormat,omitempty"`
}

type Range struct {
	Sheet  string   `json:"sheet"`
	Ref    string   `json:"ref"`
	Rows   [][]Cell `json:"rows"`
	Merges []string `json:"merges,omitempty"`
}

type Operation struct {
	Type    string        `json:"type" jsonschema:"Operation type: set_cell, set_formula, paste_range, set_format, merge_cells, or unmerge_cells"`
	Sheet   string        `json:"sheet" jsonschema:"Worksheet name"`
	Cell    string        `json:"cell,omitempty" jsonschema:"A1-style cell address for a single-cell operation"`
	Range   string        `json:"range,omitempty" jsonschema:"A1-style destination range for paste_range"`
	Value   any           `json:"value,omitempty" jsonschema:"Cell value for set_cell"`
	Formula string        `json:"formula,omitempty" jsonschema:"Formula for set_formula, with or without a leading equals sign"`
	Cells   [][]CellInput `json:"cells,omitempty" jsonschema:"Rectangular cell matrix for paste_range"`
	Format  *CellFormat   `json:"format,omitempty" jsonschema:"Partial formatting for set_format"`
}

type CellFormat struct {
	Bold         *bool    `json:"bold,omitempty"`
	Italic       *bool    `json:"italic,omitempty"`
	FontFamily   *string  `json:"fontFamily,omitempty"`
	FontSize     *float64 `json:"fontSize,omitempty"`
	FontColor    *string  `json:"fontColor,omitempty"`
	FillColor    *string  `json:"fillColor,omitempty"`
	NumberFormat *string  `json:"numberFormat,omitempty"`
}

type CellInput struct {
	Value   any    `json:"value,omitempty" jsonschema:"Cell value"`
	Formula string `json:"formula,omitempty" jsonschema:"Formula with or without a leading equals sign"`
}

type CellChange struct {
	Cell    string `json:"cell"`
	Value   any    `json:"value,omitempty"`
	Formula string `json:"formula,omitempty"`
}

type Event struct {
	Sequence uint64       `json:"sequence"`
	Revision uint64       `json:"revision"`
	Actor    string       `json:"actor"`
	Type     string       `json:"type"`
	Sheet    string       `json:"sheet,omitempty"`
	Cell     string       `json:"cell,omitempty"`
	Range    string       `json:"range,omitempty"`
	Text     string       `json:"text,omitempty"`
	State    string       `json:"state,omitempty"`
	Cells    []CellChange `json:"cells,omitempty"`
	Format   *CellFormat  `json:"format,omitempty"`
}

type Presence struct {
	Sheet string `json:"sheet" jsonschema:"Worksheet name"`
	Range string `json:"range" jsonschema:"Selected A1-style cell or range"`
	State string `json:"state,omitempty" jsonschema:"Presence state: selecting, editing, or idle"`
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
	history     []Event
	subscribers map[chan Event]struct{}
}

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
			style, styleErr := s.readCellStyle(sheet, address)
			if styleErr != nil {
				return Range{}, styleErr
			}
			cells = append(cells, Cell{Address: address, Value: value, Formula: formula, Style: style})
		}
		rows = append(rows, cells)
	}
	merges, err := s.mergesInRange(sheet, c1, r1, c2, r2)
	if err != nil {
		return Range{}, err
	}
	return Range{Sheet: sheet, Ref: ref, Rows: rows, Merges: merges}, nil
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

func (s *Session) EventsAfter(after uint64) ([]Event, Snapshot) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	events := make([]Event, 0)
	for _, event := range s.history {
		if event.Sequence > after {
			events = append(events, event)
		}
	}
	return events, s.snapshotLocked()
}

func (s *Session) Subscribe(after uint64) (<-chan Event, func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	replay := make([]Event, 0)
	for _, event := range s.history {
		if event.Sequence > after {
			replay = append(replay, event)
		}
	}
	ch := make(chan Event, len(replay)+64)
	for _, event := range replay {
		ch <- event
	}
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
	if op.Type != "set_cell" && op.Type != "set_formula" && op.Type != "paste_range" && op.Type != "set_format" && op.Type != "merge_cells" && op.Type != "unmerge_cells" {
		return fmt.Errorf("unsupported type %q", op.Type)
	}
	if !s.hasSheet(op.Sheet) {
		return fmt.Errorf("unknown sheet %q", op.Sheet)
	}
	if op.Type == "paste_range" {
		_, _, width, height, err := parseRange(op.Range)
		if err != nil {
			return err
		}
		if len(op.Cells) != height {
			return fmt.Errorf("paste range requires %d rows, got %d", height, len(op.Cells))
		}
		for row, cells := range op.Cells {
			if len(cells) != width {
				return fmt.Errorf("paste row %d requires %d cells, got %d", row, width, len(cells))
			}
		}
		return nil
	}
	if op.Type == "set_format" {
		if op.Format == nil {
			return errors.New("format is required")
		}
		_, _, _, _, err := parseRange(op.Range)
		return err
	}
	if op.Type == "merge_cells" || op.Type == "unmerge_cells" {
		_, _, width, height, err := parseRange(op.Range)
		if err != nil {
			return err
		}
		if op.Type == "merge_cells" && width == 1 && height == 1 {
			return errors.New("merge range must contain more than one cell")
		}
		return nil
	}
	if _, _, err := excelize.CellNameToCoordinates(op.Cell); err != nil {
		return fmt.Errorf("invalid cell %q", op.Cell)
	}
	if op.Type == "set_formula" && op.Formula == "" {
		return errors.New("formula is required")
	}
	return nil
}

func (s *Session) readCellStyle(sheet, cell string) (*CellStyle, error) {
	styleID, err := s.file.GetCellStyle(sheet, cell)
	if err != nil {
		return nil, fmt.Errorf("read style %s!%s: %w", sheet, cell, err)
	}
	if styleID == 0 {
		return nil, nil
	}
	style, err := s.file.GetStyle(styleID)
	if err != nil {
		return nil, fmt.Errorf("read style definition %s!%s: %w", sheet, cell, err)
	}
	result := &CellStyle{}
	if style.CustomNumFmt != nil {
		result.NumberFormat = *style.CustomNumFmt
	}
	if style.Font != nil {
		result.Bold = style.Font.Bold
		result.Italic = style.Font.Italic
		result.FontFamily = style.Font.Family
		result.FontSize = style.Font.Size
		result.FontColor = cssColor(style.Font.Color)
	}
	if len(style.Fill.Color) > 0 {
		result.FillColor = cssColor(style.Fill.Color[0])
	}
	return result, nil
}

func (s *Session) applyFormatLocked(op Operation) error {
	startCol, startRow, width, height, _ := parseRange(op.Range)
	for row := startRow; row < startRow+height; row++ {
		for col := startCol; col < startCol+width; col++ {
			cell, _ := excelize.CoordinatesToCellName(col, row)
			styleID, err := s.file.GetCellStyle(op.Sheet, cell)
			if err != nil {
				return fmt.Errorf("read style %s!%s: %w", op.Sheet, cell, err)
			}
			style, err := s.file.GetStyle(styleID)
			if err != nil {
				return fmt.Errorf("read style definition %s!%s: %w", op.Sheet, cell, err)
			}
			if style.Font == nil {
				style.Font = &excelize.Font{}
			}
			format := op.Format
			if format.Bold != nil {
				style.Font.Bold = *format.Bold
			}
			if format.Italic != nil {
				style.Font.Italic = *format.Italic
			}
			if format.FontFamily != nil {
				style.Font.Family = *format.FontFamily
			}
			if format.FontSize != nil {
				style.Font.Size = *format.FontSize
			}
			if format.FontColor != nil {
				style.Font.Color = excelColor(*format.FontColor)
			}
			if format.FillColor != nil {
				if *format.FillColor == "" {
					style.Fill = excelize.Fill{}
				} else {
					style.Fill = excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{excelColor(*format.FillColor)}}
				}
			}
			if format.NumberFormat != nil {
				if *format.NumberFormat == "" {
					style.CustomNumFmt = nil
					style.NumFmt = 0
				} else {
					style.CustomNumFmt = format.NumberFormat
				}
			}
			newStyleID, err := s.file.NewStyle(style)
			if err != nil {
				return fmt.Errorf("create style for %s!%s: %w", op.Sheet, cell, err)
			}
			if err := s.file.SetCellStyle(op.Sheet, cell, cell, newStyleID); err != nil {
				return fmt.Errorf("set style %s!%s: %w", op.Sheet, cell, err)
			}
		}
	}
	return nil
}

func (s *Session) mergesInRange(sheet string, c1, r1, c2, r2 int) ([]string, error) {
	mergeCells, err := s.file.GetMergeCells(sheet, true)
	if err != nil {
		return nil, fmt.Errorf("read merged cells for %s: %w", sheet, err)
	}
	merges := make([]string, 0)
	for _, merge := range mergeCells {
		mc1, mr1, _, _, parseErr := parseRange(merge[0])
		if parseErr != nil {
			continue
		}
		parts := strings.Split(merge[0], ":")
		mc2, mr2, coordErr := excelize.CellNameToCoordinates(parts[len(parts)-1])
		if coordErr == nil && mc1 <= c2 && mc2 >= c1 && mr1 <= r2 && mr2 >= r1 {
			merges = append(merges, merge[0])
		}
	}
	return merges, nil
}

func cssColor(value string) string {
	value = strings.TrimPrefix(value, "#")
	if len(value) == 8 {
		value = value[2:]
	}
	if len(value) != 6 {
		return ""
	}
	return "#" + strings.ToUpper(value)
}

func excelColor(value string) string {
	return strings.ToUpper(strings.TrimPrefix(value, "#"))
}

func (s *Session) operationChanges(op Operation) ([]CellChange, error) {
	if op.Type != "paste_range" {
		return []CellChange{{Cell: strings.ToUpper(op.Cell), Value: op.Value, Formula: op.Formula}}, nil
	}
	startCol, startRow, _, _, err := parseRange(op.Range)
	if err != nil {
		return nil, err
	}
	changes := make([]CellChange, 0, len(op.Cells)*len(op.Cells[0]))
	for rowOffset, row := range op.Cells {
		for colOffset, cell := range row {
			address, _ := excelize.CoordinatesToCellName(startCol+colOffset, startRow+rowOffset)
			changes = append(changes, CellChange{Cell: address, Value: cell.Value, Formula: cell.Formula})
		}
	}
	return changes, nil
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
	s.history = append(s.history, event)
	if len(s.history) > maxEventHistory {
		s.history = append([]Event(nil), s.history[len(s.history)-maxEventHistory:]...)
	}
	for ch := range s.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

func parseRange(ref string) (startCol, startRow, width, height int, err error) {
	parts := strings.Split(strings.ToUpper(strings.TrimSpace(ref)), ":")
	if len(parts) == 1 {
		parts = append(parts, parts[0])
	}
	if len(parts) != 2 {
		return 0, 0, 0, 0, fmt.Errorf("invalid range %q", ref)
	}
	startCol, startRow, err = excelize.CellNameToCoordinates(parts[0])
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("invalid range %q", ref)
	}
	endCol, endRow, coordErr := excelize.CellNameToCoordinates(parts[1])
	if coordErr != nil || startCol > endCol || startRow > endRow {
		return 0, 0, 0, 0, fmt.Errorf("invalid range %q", ref)
	}
	width = endCol - startCol + 1
	height = endRow - startRow + 1
	if width*height > maxRangeCells {
		return 0, 0, 0, 0, fmt.Errorf("range exceeds %d cells", maxRangeCells)
	}
	return startCol, startRow, width, height, nil
}

func normalizeFormula(formula string) string {
	return strings.TrimPrefix(formula, "=")
}

func changeText(change CellChange) string {
	if change.Formula != "" {
		return change.Formula
	}
	if change.Value == nil {
		return ""
	}
	return fmt.Sprint(change.Value)
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
