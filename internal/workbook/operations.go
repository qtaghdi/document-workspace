package workbook

import (
	"errors"
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

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

func excelColor(value string) string {
	return strings.ToUpper(strings.TrimPrefix(value, "#"))
}
