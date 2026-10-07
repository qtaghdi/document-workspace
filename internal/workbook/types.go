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
	Sheet                 string              `json:"sheet"`
	Ref                   string              `json:"ref"`
	Rows                  [][]Cell            `json:"rows"`
	Merges                []string            `json:"merges,omitempty"`
	Validations           []DataValidation    `json:"validations,omitempty"`
	ConditionalFormatting []ConditionalFormat `json:"conditionalFormatting,omitempty"`
}

type DataValidation struct {
	Range            string `json:"range"`
	Type             string `json:"type"`
	Operator         string `json:"operator,omitempty"`
	Formula1         string `json:"formula1,omitempty"`
	Formula2         string `json:"formula2,omitempty"`
	AllowBlank       bool   `json:"allowBlank,omitempty"`
	ShowDropDown     bool   `json:"showDropDown,omitempty"`
	ShowErrorMessage bool   `json:"showErrorMessage,omitempty"`
	Error            string `json:"error,omitempty"`
	ErrorTitle       string `json:"errorTitle,omitempty"`
	ShowInputMessage bool   `json:"showInputMessage,omitempty"`
	Prompt           string `json:"prompt,omitempty"`
	PromptTitle      string `json:"promptTitle,omitempty"`
	Date1904         bool   `json:"date1904,omitempty"`
}

type ConditionalFormat struct {
	Range        string     `json:"range"`
	Type         string     `json:"type"`
	Criteria     string     `json:"criteria,omitempty"`
	Value        string     `json:"value,omitempty"`
	MinType      string     `json:"minType,omitempty"`
	MidType      string     `json:"midType,omitempty"`
	MaxType      string     `json:"maxType,omitempty"`
	MinValue     string     `json:"minValue,omitempty"`
	MidValue     string     `json:"midValue,omitempty"`
	MaxValue     string     `json:"maxValue,omitempty"`
	MinColor     string     `json:"minColor,omitempty"`
	MidColor     string     `json:"midColor,omitempty"`
	MaxColor     string     `json:"maxColor,omitempty"`
	BarColor     string     `json:"barColor,omitempty"`
	BarOnly      bool       `json:"barOnly,omitempty"`
	BarSolid     bool       `json:"barSolid,omitempty"`
	AboveAverage bool       `json:"aboveAverage,omitempty"`
	Percent      bool       `json:"percent,omitempty"`
	Style        *CellStyle `json:"style,omitempty"`
}

type SheetObjects struct {
	Sheet     string       `json:"sheet"`
	Images    []SheetImage `json:"images,omitempty"`
	Charts    []SheetChart `json:"charts,omitempty"`
	Truncated bool         `json:"truncated,omitempty"`
}

type SheetImage struct {
	ID       string `json:"id"`
	Name     string `json:"name,omitempty"`
	AltText  string `json:"altText,omitempty"`
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
	Row      int    `json:"row"`
	Column   int    `json:"column"`
	OffsetX  int    `json:"offsetX,omitempty"`
	OffsetY  int    `json:"offsetY,omitempty"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
}

type SheetChart struct {
	ID      string        `json:"id"`
	Title   string        `json:"title,omitempty"`
	Type    string        `json:"type"`
	Row     int           `json:"row"`
	Column  int           `json:"column"`
	OffsetX int           `json:"offsetX,omitempty"`
	OffsetY int           `json:"offsetY,omitempty"`
	Width   int           `json:"width"`
	Height  int           `json:"height"`
	Series  []ChartSeries `json:"series"`
}

type ChartSeries struct {
	Name       string    `json:"name,omitempty"`
	Categories []string  `json:"categories"`
	Values     []float64 `json:"values"`
}

type Operation struct {
	Type    string        `json:"type" jsonschema:"Operation type: set_cell, set_formula, paste_range, set_format, merge_cells, unmerge_cells, insert_rows, delete_rows, insert_columns, or delete_columns"`
	Sheet   string        `json:"sheet" jsonschema:"Worksheet name"`
	Cell    string        `json:"cell,omitempty" jsonschema:"A1-style cell address for a single-cell operation"`
	Range   string        `json:"range,omitempty" jsonschema:"A1-style destination range for paste_range"`
	Value   any           `json:"value,omitempty" jsonschema:"Cell value for set_cell"`
	Formula string        `json:"formula,omitempty" jsonschema:"Formula for set_formula, with or without a leading equals sign"`
	Cells   [][]CellInput `json:"cells,omitempty" jsonschema:"Rectangular cell matrix for paste_range"`
	Format  *CellFormat   `json:"format,omitempty" jsonschema:"Partial formatting for set_format"`
	Index   int           `json:"index,omitempty" jsonschema:"One-based row or column index for a structural operation"`
	Count   int           `json:"count,omitempty" jsonschema:"Number of rows or columns for a structural operation"`
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
	Index    int          `json:"index,omitempty"`
	Count    int          `json:"count,omitempty"`
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
	Warnings        []FeatureWarning  `json:"warnings,omitempty"`
	CanUndo         bool              `json:"canUndo"`
	CanRedo         bool              `json:"canRedo"`
	Revision        uint64            `json:"revision"`
}

type FeatureWarning struct {
	Feature string `json:"feature"`
	Message string `json:"message"`
}

type SheetDimensions struct {
	Name    string `json:"name"`
	Rows    int    `json:"rows"`
	Columns int    `json:"columns"`
}

type Session struct {
	mu           sync.RWMutex
	id           string
	path         string
	file         *excelize.File
	revision     uint64
	sequence     uint64
	history      []Event
	subscribers  map[chan Event]struct{}
	dimensions   map[string]SheetDimensions
	warnings     []FeatureWarning
	undoHistory  [][]byte
	redoHistory  [][]byte
	historyStore *historyStore
}
