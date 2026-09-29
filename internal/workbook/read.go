package workbook

import (
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

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
