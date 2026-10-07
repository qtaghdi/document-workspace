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
	validations, err := s.validationsInRange(sheet, c1, r1, c2, r2)
	if err != nil {
		return Range{}, err
	}
	conditionalFormatting, err := s.conditionalFormattingInRange(sheet, c1, r1, c2, r2)
	if err != nil {
		return Range{}, err
	}
	return Range{Sheet: sheet, Ref: ref, Rows: rows, Merges: merges, Validations: validations, ConditionalFormatting: conditionalFormatting}, nil
}

func (s *Session) validationsInRange(sheet string, c1, r1, c2, r2 int) ([]DataValidation, error) {
	items, err := s.file.GetDataValidations(sheet)
	if err != nil {
		return nil, fmt.Errorf("read data validations for %s: %w", sheet, err)
	}
	date1904 := false
	if properties, propertiesErr := s.file.GetWorkbookProps(); propertiesErr == nil && properties.Date1904 != nil {
		date1904 = *properties.Date1904
	}
	result := make([]DataValidation, 0)
	for _, item := range items {
		for _, ref := range strings.Fields(item.Sqref) {
			if rangeIntersects(ref, c1, r1, c2, r2) {
				result = append(result, DataValidation{
					Range: ref, Type: item.Type, Operator: item.Operator,
					Formula1: item.Formula1, Formula2: item.Formula2,
					AllowBlank: item.AllowBlank, ShowDropDown: item.ShowDropDown,
					ShowErrorMessage: item.ShowErrorMessage, Error: optionalString(item.Error),
					ErrorTitle: optionalString(item.ErrorTitle), ShowInputMessage: item.ShowInputMessage,
					Prompt: optionalString(item.Prompt), PromptTitle: optionalString(item.PromptTitle),
					Date1904: date1904,
				})
			}
		}
	}
	return result, nil
}

func (s *Session) conditionalFormattingInRange(sheet string, c1, r1, c2, r2 int) ([]ConditionalFormat, error) {
	formats, err := s.file.GetConditionalFormats(sheet)
	if err != nil {
		return nil, fmt.Errorf("read conditional formatting for %s: %w", sheet, err)
	}
	result := make([]ConditionalFormat, 0)
	for ref, options := range formats {
		if !rangeIntersects(ref, c1, r1, c2, r2) {
			continue
		}
		for _, option := range options {
			item := ConditionalFormat{
				Range: ref, Type: option.Type, Criteria: option.Criteria, Value: option.Value,
				MinType: option.MinType, MidType: option.MidType, MaxType: option.MaxType,
				MinValue: option.MinValue, MidValue: option.MidValue, MaxValue: option.MaxValue,
				MinColor: option.MinColor, MidColor: option.MidColor, MaxColor: option.MaxColor,
				BarColor: option.BarColor, BarOnly: option.BarOnly, BarSolid: option.BarSolid,
				AboveAverage: option.AboveAverage, Percent: option.Percent,
			}
			if option.Format != nil {
				style, styleErr := s.file.GetConditionalStyle(*option.Format)
				if styleErr != nil {
					return nil, fmt.Errorf("read conditional style for %s!%s: %w", sheet, ref, styleErr)
				}
				item.Style = cellStyleFromExcelize(style)
			}
			result = append(result, item)
		}
	}
	return result, nil
}

func optionalString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func rangeIntersects(ref string, c1, r1, c2, r2 int) bool {
	rc1, rr1, width, height, err := parseRange(ref)
	if err != nil {
		return false
	}
	rc2, rr2 := rc1+width-1, rr1+height-1
	return rc1 <= c2 && rc2 >= c1 && rr1 <= r2 && rr2 >= r1
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
	return cellStyleFromExcelize(style), nil
}

func cellStyleFromExcelize(style *excelize.Style) *CellStyle {
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
	return result
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
