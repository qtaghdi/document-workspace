package workbook

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"path"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
)

const maxSheetObjects = 10
const maxSheetObjectBytes = 8 << 20
const maxSheetObjectDimension = 4096
const maxChartSeries = 8
const maxChartPoints = 200
const maxChartLabelBytes = 128
const maxChartTitleBytes = 256

func (s *Session) ReadSheetObjects(sheet string) (SheetObjects, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.hasSheet(sheet) {
		return SheetObjects{}, fmt.Errorf("unknown sheet %q", sheet)
	}
	images, truncated, err := s.readSheetImagesLocked(sheet)
	if err != nil {
		return SheetObjects{}, err
	}
	charts, err := s.readSheetChartsLocked(sheet)
	if err != nil {
		return SheetObjects{}, err
	}
	remaining := maxSheetObjects - len(images)
	if len(charts) > remaining {
		charts = charts[:remaining]
		truncated = true
	}
	return SheetObjects{Sheet: sheet, Images: images, Charts: charts, Truncated: truncated}, nil
}

func (s *Session) readSheetImagesLocked(sheet string) ([]SheetImage, bool, error) {
	cells, err := s.file.GetPictureCells(sheet)
	if err != nil {
		return nil, false, fmt.Errorf("list images for %s: %w", sheet, err)
	}
	result := make([]SheetImage, 0, len(cells))
	totalBytes := 0
	truncated := false
	for _, cell := range cells {
		pictures, err := s.file.GetPictures(sheet, cell)
		if err != nil {
			return nil, false, fmt.Errorf("read images for %s!%s: %w", sheet, cell, err)
		}
		column, row, err := excelize.CellNameToCoordinates(cell)
		if err != nil {
			truncated = true
			continue
		}
		for index, picture := range pictures {
			if len(result) >= maxSheetObjects || totalBytes+len(picture.File) > maxSheetObjectBytes {
				truncated = true
				continue
			}
			mimeType, width, height, ok := safeImageMetadata(picture.File, picture.Format)
			if !ok {
				truncated = true
				continue
			}
			totalBytes += len(picture.File)
			item := SheetImage{
				ID:       fmt.Sprintf("image-%s-%d", strings.ToLower(cell), index+1),
				MimeType: mimeType,
				Data:     base64.StdEncoding.EncodeToString(picture.File),
				Row:      row - 1,
				Column:   column - 1,
				Width:    width,
				Height:   height,
			}
			if picture.Format != nil {
				item.Name = picture.Format.Name
				item.AltText = picture.Format.AltText
				item.OffsetX = picture.Format.OffsetX
				item.OffsetY = picture.Format.OffsetY
			}
			result = append(result, item)
		}
	}
	return result, truncated, nil
}

func safeImageMetadata(content []byte, options *excelize.GraphicOptions) (string, int, int, bool) {
	config, format, err := image.DecodeConfig(bytes.NewReader(content))
	if err != nil {
		return "", 0, 0, false
	}
	mimeType := map[string]string{
		"gif":  "image/gif",
		"jpeg": "image/jpeg",
		"png":  "image/png",
	}[format]
	if mimeType == "" {
		return "", 0, 0, false
	}
	scaleX, scaleY := 1.0, 1.0
	if options != nil {
		if options.ScaleX > 0 {
			scaleX = options.ScaleX
		}
		if options.ScaleY > 0 {
			scaleY = options.ScaleY
		}
	}
	width := min(maxSheetObjectDimension, max(1, int(float64(config.Width)*scaleX)))
	height := min(maxSheetObjectDimension, max(1, int(float64(config.Height)*scaleY)))
	return mimeType, width, height, true
}

type packageRelationship struct {
	ID     string `xml:"Id,attr"`
	Target string `xml:"Target,attr"`
	Type   string `xml:"Type,attr"`
}

type packageRelationships struct {
	Items []packageRelationship `xml:"Relationship"`
}

type workbookPackage struct {
	Sheets []struct {
		Name string `xml:"name,attr"`
		RID  string `xml:"id,attr"`
	} `xml:"sheets>sheet"`
}

type worksheetPackage struct {
	Drawing struct {
		RID string `xml:"id,attr"`
	} `xml:"drawing"`
}

type drawingMarker struct {
	Column       int `xml:"col"`
	ColumnOffset int `xml:"colOff"`
	Row          int `xml:"row"`
	RowOffset    int `xml:"rowOff"`
}

type drawingAnchor struct {
	From         drawingMarker `xml:"from"`
	To           drawingMarker `xml:"to"`
	GraphicFrame struct {
		Properties struct {
			ID   string `xml:"id,attr"`
			Name string `xml:"name,attr"`
		} `xml:"nvGraphicFramePr>cNvPr"`
		Chart struct {
			RID string `xml:"id,attr"`
		} `xml:"graphic>graphicData>chart"`
	} `xml:"graphicFrame"`
}

type drawingPackage struct {
	Anchors []drawingAnchor `xml:"twoCellAnchor"`
}

type chartReference struct {
	Formula string `xml:"f"`
	Value   string `xml:"v"`
}

type chartSeriesXML struct {
	Text struct {
		String chartReference `xml:"strRef"`
		Value  string         `xml:"v"`
	} `xml:"tx"`
	Category struct {
		String chartReference `xml:"strRef"`
		Number chartReference `xml:"numRef"`
	} `xml:"cat"`
	Values struct {
		Number chartReference `xml:"numRef"`
	} `xml:"val"`
}

type chartBlockXML struct {
	Series []chartSeriesXML `xml:"ser"`
}

type chartPackage struct {
	Chart struct {
		Title struct {
			Paragraphs []struct {
				Runs []struct {
					Text string `xml:"t"`
				} `xml:"r"`
			} `xml:"tx>rich>p"`
		} `xml:"title"`
		PlotArea struct {
			Bar      *chartBlockXML `xml:"barChart"`
			Line     *chartBlockXML `xml:"lineChart"`
			Pie      *chartBlockXML `xml:"pieChart"`
			Doughnut *chartBlockXML `xml:"doughnutChart"`
			Area     *chartBlockXML `xml:"areaChart"`
		} `xml:"plotArea"`
	} `xml:"chart"`
}

func (s *Session) readSheetChartsLocked(sheet string) ([]SheetChart, error) {
	reader, err := zip.OpenReader(s.path)
	if err != nil {
		return nil, fmt.Errorf("open workbook package for charts: %w", err)
	}
	defer reader.Close()
	parts := make(map[string]*zip.File, len(reader.File))
	for _, entry := range reader.File {
		parts[path.Clean(entry.Name)] = entry
	}
	workbookXML, err := readPackageXML[workbookPackage](parts, "xl/workbook.xml")
	if err != nil {
		return nil, err
	}
	workbookRels, err := readPackageXML[packageRelationships](parts, "xl/_rels/workbook.xml.rels")
	if err != nil {
		return nil, err
	}
	var worksheetPath string
	for _, candidate := range workbookXML.Sheets {
		if candidate.Name == sheet {
			worksheetPath = resolvePackageTarget("xl/workbook.xml", relationshipTarget(workbookRels, candidate.RID))
			break
		}
	}
	if worksheetPath == "" {
		return nil, nil
	}
	worksheetXML, err := readPackageXML[worksheetPackage](parts, worksheetPath)
	if err != nil || worksheetXML.Drawing.RID == "" {
		return nil, err
	}
	worksheetRelsPath := path.Join(path.Dir(worksheetPath), "_rels", path.Base(worksheetPath)+".rels")
	worksheetRels, err := readPackageXML[packageRelationships](parts, worksheetRelsPath)
	if err != nil {
		return nil, err
	}
	drawingPath := resolvePackageTarget(worksheetPath, relationshipTarget(worksheetRels, worksheetXML.Drawing.RID))
	if drawingPath == "" {
		return nil, nil
	}
	drawingXML, err := readPackageXML[drawingPackage](parts, drawingPath)
	if err != nil {
		return nil, err
	}
	drawingRelsPath := path.Join(path.Dir(drawingPath), "_rels", path.Base(drawingPath)+".rels")
	drawingRels, err := readPackageXML[packageRelationships](parts, drawingRelsPath)
	if err != nil {
		return nil, err
	}
	result := make([]SheetChart, 0)
	for index, anchor := range drawingXML.Anchors {
		if len(result) >= maxSheetObjects {
			break
		}
		if anchor.GraphicFrame.Chart.RID == "" {
			continue
		}
		chartPath := resolvePackageTarget(drawingPath, relationshipTarget(drawingRels, anchor.GraphicFrame.Chart.RID))
		chartXML, err := readPackageXML[chartPackage](parts, chartPath)
		if err != nil {
			return nil, err
		}
		chartType, block := chartBlock(chartXML)
		if block == nil {
			continue
		}
		if anchor.From.Row < 0 || anchor.From.Column < 0 {
			continue
		}
		width, height, sizeErr := s.drawingPixelSizeLocked(sheet, anchor.From, anchor.To)
		if sizeErr != nil {
			return nil, sizeErr
		}
		item := SheetChart{
			ID:      fmt.Sprintf("chart-%d", index+1),
			Title:   truncateText(chartTitle(chartXML), maxChartTitleBytes),
			Type:    chartType,
			Row:     anchor.From.Row,
			Column:  anchor.From.Column,
			OffsetX: anchor.From.ColumnOffset / 9525,
			OffsetY: anchor.From.RowOffset / 9525,
			Width:   min(maxSheetObjectDimension, max(1, width)),
			Height:  min(maxSheetObjectDimension, max(1, height)),
		}
		for _, series := range block.Series[:min(len(block.Series), maxChartSeries)] {
			name := series.Text.Value
			if name == "" {
				values := s.chartReferenceValues(series.Text.String.Formula)
				if len(values) > 0 {
					name = values[0]
				}
			}
			categoryFormula := series.Category.String.Formula
			if categoryFormula == "" {
				categoryFormula = series.Category.Number.Formula
			}
			categories := truncateLabels(s.chartReferenceValues(categoryFormula))
			valueStrings := s.chartReferenceValues(series.Values.Number.Formula)
			if len(valueStrings) > maxChartPoints {
				valueStrings = valueStrings[:maxChartPoints]
			}
			values := make([]float64, 0, len(valueStrings))
			for _, value := range valueStrings {
				parsed, parseErr := strconv.ParseFloat(value, 64)
				if parseErr == nil {
					values = append(values, parsed)
				}
			}
			if len(values) > 0 {
				item.Series = append(item.Series, ChartSeries{Name: truncateText(name, maxChartLabelBytes), Categories: categories, Values: values})
			}
		}
		if len(item.Series) > 0 {
			result = append(result, item)
		}
	}
	return result, nil
}

func readPackageXML[T any](parts map[string]*zip.File, name string) (T, error) {
	var result T
	entry := parts[path.Clean(name)]
	if entry == nil {
		return result, nil
	}
	file, err := entry.Open()
	if err != nil {
		return result, fmt.Errorf("open workbook package part %s: %w", name, err)
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, maxFeatureScanBytes))
	if err != nil {
		return result, fmt.Errorf("read workbook package part %s: %w", name, err)
	}
	if err := xml.Unmarshal(content, &result); err != nil {
		return result, fmt.Errorf("decode workbook package part %s: %w", name, err)
	}
	return result, nil
}

func relationshipTarget(relationships packageRelationships, id string) string {
	for _, relationship := range relationships.Items {
		if relationship.ID == id {
			return relationship.Target
		}
	}
	return ""
}

func resolvePackageTarget(source, target string) string {
	if target == "" {
		return ""
	}
	if strings.HasPrefix(target, "/") {
		return path.Clean(strings.TrimPrefix(target, "/"))
	}
	return path.Clean(path.Join(path.Dir(source), target))
}

func chartBlock(chart chartPackage) (string, *chartBlockXML) {
	switch {
	case chart.Chart.PlotArea.Bar != nil:
		return "bar", chart.Chart.PlotArea.Bar
	case chart.Chart.PlotArea.Line != nil:
		return "line", chart.Chart.PlotArea.Line
	case chart.Chart.PlotArea.Pie != nil:
		return "pie", chart.Chart.PlotArea.Pie
	case chart.Chart.PlotArea.Doughnut != nil:
		return "doughnut", chart.Chart.PlotArea.Doughnut
	case chart.Chart.PlotArea.Area != nil:
		return "area", chart.Chart.PlotArea.Area
	default:
		return "", nil
	}
}

func chartTitle(chart chartPackage) string {
	var title strings.Builder
	for _, paragraph := range chart.Chart.Title.Paragraphs {
		for _, run := range paragraph.Runs {
			title.WriteString(run.Text)
		}
	}
	return title.String()
}

func (s *Session) chartReferenceValues(formula string) []string {
	separator := strings.LastIndex(formula, "!")
	if separator < 1 {
		return nil
	}
	sheet := strings.Trim(formula[:separator], "'")
	sheet = strings.ReplaceAll(sheet, "''", "'")
	ref := strings.ReplaceAll(formula[separator+1:], "$", "")
	startColumn, startRow, width, height, err := parseRange(ref)
	if err != nil || int64(width)*int64(height) > maxRangeCells {
		return nil
	}
	values := make([]string, 0, min(width*height, maxChartPoints))
	for row := startRow; row < startRow+height; row++ {
		for column := startColumn; column < startColumn+width; column++ {
			if len(values) >= maxChartPoints {
				return values
			}
			cell, cellErr := excelize.CoordinatesToCellName(column, row)
			if cellErr != nil {
				continue
			}
			value, valueErr := s.file.GetCellValue(sheet, cell, excelize.Options{RawCellValue: true})
			if valueErr == nil {
				values = append(values, value)
			}
		}
	}
	return values
}

func truncateLabels(values []string) []string {
	if len(values) > maxChartPoints {
		values = values[:maxChartPoints]
	}
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = truncateText(value, maxChartLabelBytes)
	}
	return result
}

func truncateText(value string, limit int) string {
	value = strings.ToValidUTF8(value, "")
	if len(value) <= limit {
		return value
	}
	value = value[:limit]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value
}
