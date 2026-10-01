package workbook

import (
	"sync"

	"github.com/xuri/excelize/v2"
)

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
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Sheets          []string          `json:"sheets"`
	SheetDimensions []SheetDimensions `json:"sheetDimensions"`
	Revision        uint64            `json:"revision"`
}

type SheetDimensions struct {
	Name    string `json:"name"`
	Rows    int    `json:"rows"`
	Columns int    `json:"columns"`
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
	dimensions  map[string]SheetDimensions
}
