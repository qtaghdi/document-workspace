package workbook

import (
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

func inspectSheetDimensions(file *excelize.File) (map[string]SheetDimensions, error) {
	result := make(map[string]SheetDimensions)
	for _, sheet := range file.GetSheetList() {
		dimensions := SheetDimensions{Name: sheet}
		rows, err := file.Rows(sheet)
		if err != nil {
			return nil, fmt.Errorf("inspect rows for %s: %w", sheet, err)
		}
		for rows.Next() {
			dimensions.Rows++
			columns, columnsErr := rows.Columns(excelize.Options{RawCellValue: true})
			if columnsErr != nil {
				_ = rows.Close()
				return nil, fmt.Errorf("inspect columns for %s: %w", sheet, columnsErr)
			}
			if len(columns) > dimensions.Columns {
				dimensions.Columns = len(columns)
			}
		}
		if err := rows.Error(); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("iterate rows for %s: %w", sheet, err)
		}
		if err := rows.Close(); err != nil {
			return nil, fmt.Errorf("close row reader for %s: %w", sheet, err)
		}

		if ref, err := file.GetSheetDimension(sheet); err == nil && ref != "" {
			growDimensionsFromRange(&dimensions, ref)
		}
		tables, err := file.GetTables(sheet)
		if err != nil {
			return nil, fmt.Errorf("inspect tables for %s: %w", sheet, err)
		}
		for _, table := range tables {
			growDimensionsFromRange(&dimensions, table.Range)
		}
		merges, err := file.GetMergeCells(sheet, true)
		if err != nil {
			return nil, fmt.Errorf("inspect merged cells for %s: %w", sheet, err)
		}
		for _, merge := range merges {
			growDimensionsFromRange(&dimensions, merge[0])
		}
		result[sheet] = dimensions
	}
	return result, nil
}

func growDimensionsFromRange(dimensions *SheetDimensions, ref string) {
	parts := strings.Split(strings.ToUpper(strings.TrimSpace(ref)), ":")
	end := parts[len(parts)-1]
	column, row, err := excelize.CellNameToCoordinates(end)
	if err != nil {
		return
	}
	if row > dimensions.Rows {
		dimensions.Rows = row
	}
	if column > dimensions.Columns {
		dimensions.Columns = column
	}
}

func (s *Session) sheetDimensionsLocked() []SheetDimensions {
	result := make([]SheetDimensions, 0, len(s.dimensions))
	for _, sheet := range s.file.GetSheetList() {
		dimensions := s.dimensions[sheet]
		dimensions.Name = sheet
		result = append(result, dimensions)
	}
	return result
}

func (s *Session) growDimensionsLocked(operations []Operation) {
	for _, operation := range operations {
		dimensions := s.dimensions[operation.Sheet]
		dimensions.Name = operation.Sheet
		ref := operation.Cell
		if operation.Range != "" {
			ref = operation.Range
		}
		growDimensionsFromRange(&dimensions, ref)
		s.dimensions[operation.Sheet] = dimensions
	}
}
