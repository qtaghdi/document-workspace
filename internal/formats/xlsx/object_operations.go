package xlsx

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	"io"
	"math"
	"path"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

const emuPerPixel = 9525

func (s *Session) validateObjectOperationLocked(op Operation) error {
	if op.ObjectID == "" {
		return errors.New("objectId is required")
	}
	if _, _, err := excelize.CellNameToCoordinates(op.Cell); err != nil {
		return fmt.Errorf("invalid object anchor cell %q", op.Cell)
	}
	if strings.HasPrefix(op.Type, "set_") {
		if _, _, err := excelize.CellNameToCoordinates(op.TargetCell); err != nil {
			return fmt.Errorf("invalid object target cell %q", op.TargetCell)
		}
		if op.OffsetX < 0 || op.OffsetY < 0 || op.OffsetX > maxSheetObjectDimension || op.OffsetY > maxSheetObjectDimension {
			return fmt.Errorf("object offsets must be between 0 and %d pixels", maxSheetObjectDimension)
		}
		if op.Width < 1 || op.Width > maxSheetObjectDimension || op.Height < 1 || op.Height > maxSheetObjectDimension {
			return fmt.Errorf("object dimensions must be between 1 and %d pixels", maxSheetObjectDimension)
		}
	}
	if op.Title != nil && len(*op.Title) > maxChartTitleBytes {
		return fmt.Errorf("chart title exceeds %d bytes", maxChartTitleBytes)
	}
	switch op.Type {
	case "set_image", "delete_image":
		if !strings.HasPrefix(op.ObjectID, "image-") {
			return fmt.Errorf("invalid image object id %q", op.ObjectID)
		}
		pictures, err := s.file.GetPictures(op.Sheet, strings.ToUpper(op.Cell))
		if err != nil {
			return fmt.Errorf("read image at %s!%s: %w", op.Sheet, op.Cell, err)
		}
		if len(pictures) != 1 {
			return fmt.Errorf("image operation requires exactly one image at %s!%s, found %d", op.Sheet, op.Cell, len(pictures))
		}
	case "set_chart", "delete_chart":
		_, _, _, err := s.chartObjectLocked(op.Sheet, op.ObjectID, op.Cell)
		return err
	}
	return nil
}

func (s *Session) applyObjectOperationLocked(op Operation) error {
	switch op.Type {
	case "set_image":
		return s.setImageLocked(op)
	case "delete_image":
		if err := s.file.DeletePicture(op.Sheet, strings.ToUpper(op.Cell)); err != nil {
			return fmt.Errorf("delete image %s from %s!%s: %w", op.ObjectID, op.Sheet, op.Cell, err)
		}
		return nil
	case "set_chart":
		return s.setChartLocked(op)
	case "delete_chart":
		if err := s.file.DeleteChart(op.Sheet, strings.ToUpper(op.Cell)); err != nil {
			return fmt.Errorf("delete chart %s from %s!%s: %w", op.ObjectID, op.Sheet, op.Cell, err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported object operation %q", op.Type)
	}
}

func (s *Session) setImageLocked(op Operation) error {
	pictures, err := s.file.GetPictures(op.Sheet, strings.ToUpper(op.Cell))
	if err != nil {
		return fmt.Errorf("read image %s from %s!%s: %w", op.ObjectID, op.Sheet, op.Cell, err)
	}
	if len(pictures) != 1 {
		return fmt.Errorf("image operation requires exactly one image at %s!%s, found %d", op.Sheet, op.Cell, len(pictures))
	}
	picture := pictures[0]
	config, _, err := image.DecodeConfig(bytes.NewReader(picture.File))
	if err != nil {
		return fmt.Errorf("decode image %s: %w", op.ObjectID, err)
	}
	format := excelize.GraphicOptions{}
	if picture.Format != nil {
		format = *picture.Format
	}
	format.OffsetX = op.OffsetX
	format.OffsetY = op.OffsetY
	format.ScaleX = float64(op.Width) / float64(config.Width)
	format.ScaleY = float64(op.Height) / float64(config.Height)
	if err := s.file.DeletePicture(op.Sheet, strings.ToUpper(op.Cell)); err != nil {
		return fmt.Errorf("remove prior image %s: %w", op.ObjectID, err)
	}
	if err := s.file.AddPictureFromBytes(op.Sheet, strings.ToUpper(op.TargetCell), &excelize.Picture{
		Extension: picture.Extension,
		File:      picture.File,
		Format:    &format,
	}); err != nil {
		return fmt.Errorf("write image %s at %s!%s: %w", op.ObjectID, op.Sheet, op.TargetCell, err)
	}
	return nil
}

type chartObjectLocation struct {
	drawingPath string
	chartPath   string
	anchorIndex int
}

func (s *Session) chartObjectLocked(sheet, objectID, cell string) (chartObjectLocation, drawingAnchor, []byte, error) {
	var empty chartObjectLocation
	drawingPath, drawing, drawingRels, drawingContent, err := s.sheetDrawingLocked(sheet)
	if err != nil {
		return empty, drawingAnchor{}, nil, err
	}
	requestedIndex, err := chartObjectIndex(objectID)
	if err != nil {
		return empty, drawingAnchor{}, nil, err
	}
	column, row, _ := excelize.CellNameToCoordinates(cell)
	if requestedIndex < 0 || requestedIndex >= len(drawing.Anchors) {
		return empty, drawingAnchor{}, nil, fmt.Errorf("unknown chart object %q", objectID)
	}
	anchor := drawing.Anchors[requestedIndex]
	if anchor.GraphicFrame.Chart.RID == "" || anchor.From.Column != column-1 || anchor.From.Row != row-1 {
		return empty, drawingAnchor{}, nil, fmt.Errorf("chart %q is not anchored at %s!%s", objectID, sheet, cell)
	}
	chartPath := resolvePackageTarget(drawingPath, relationshipTarget(drawingRels, anchor.GraphicFrame.Chart.RID))
	if chartPath == "" {
		return empty, drawingAnchor{}, nil, fmt.Errorf("chart %q has no package relationship", objectID)
	}
	return chartObjectLocation{drawingPath: drawingPath, chartPath: chartPath, anchorIndex: requestedIndex}, anchor, drawingContent, nil
}

func chartObjectIndex(objectID string) (int, error) {
	value := strings.TrimPrefix(objectID, "chart-")
	index, err := strconv.Atoi(value)
	if err != nil || index < 1 {
		return 0, fmt.Errorf("invalid chart object id %q", objectID)
	}
	return index - 1, nil
}

func (s *Session) setChartLocked(op Operation) error {
	serialized, err := s.file.WriteToBuffer()
	if err != nil {
		return fmt.Errorf("stage workbook before chart edit: %w", err)
	}
	staged, err := excelize.OpenReader(bytes.NewReader(serialized.Bytes()))
	if err != nil {
		return fmt.Errorf("reopen staged workbook before chart edit: %w", err)
	}
	previous := s.file
	s.file = staged
	succeeded := false
	defer func() {
		if !succeeded {
			s.file = previous
			_ = staged.Close()
		}
	}()

	location, _, drawingContent, err := s.chartObjectLocked(op.Sheet, op.ObjectID, op.Cell)
	if err != nil {
		return err
	}
	column, row, _ := excelize.CellNameToCoordinates(op.TargetCell)
	from := drawingMarker{Column: column - 1, Row: row - 1, ColumnOffset: op.OffsetX * emuPerPixel, RowOffset: op.OffsetY * emuPerPixel}
	to, err := s.objectEndMarkerLocked(op.Sheet, from, op.Width, op.Height)
	if err != nil {
		return err
	}
	updatedDrawing, err := rewriteDrawingAnchor(drawingContent, location.anchorIndex, from, to)
	if err != nil {
		return fmt.Errorf("move chart %s: %w", op.ObjectID, err)
	}
	s.file.Pkg.Store(location.drawingPath, updatedDrawing)
	if op.Title != nil {
		chartContent, err := packageBytes(s.file, location.chartPath)
		if err != nil {
			return err
		}
		updatedChart, err := rewriteChartTitle(chartContent, *op.Title)
		if err != nil {
			return fmt.Errorf("rename chart %s: %w", op.ObjectID, err)
		}
		s.file.Pkg.Store(location.chartPath, updatedChart)
	}
	s.file.Drawings.Delete(location.drawingPath)
	succeeded = true
	_ = previous.Close()
	return nil
}

func (s *Session) sheetDrawingLocked(sheet string) (string, drawingPackage, packageRelationships, []byte, error) {
	workbookContent, err := packageBytes(s.file, "xl/workbook.xml")
	if err != nil {
		return "", drawingPackage{}, packageRelationships{}, nil, err
	}
	var workbook workbookPackage
	if err := xml.Unmarshal(workbookContent, &workbook); err != nil {
		return "", drawingPackage{}, packageRelationships{}, nil, fmt.Errorf("decode workbook package: %w", err)
	}
	workbookRels, err := packageXML[packageRelationships](s.file, "xl/_rels/workbook.xml.rels")
	if err != nil {
		return "", drawingPackage{}, packageRelationships{}, nil, err
	}
	worksheetPath := ""
	for _, candidate := range workbook.Sheets {
		if candidate.Name == sheet {
			worksheetPath = resolvePackageTarget("xl/workbook.xml", relationshipTarget(workbookRels, candidate.RID))
			break
		}
	}
	if worksheetPath == "" {
		return "", drawingPackage{}, packageRelationships{}, nil, fmt.Errorf("worksheet package for %q not found", sheet)
	}
	worksheet, err := packageXML[worksheetPackage](s.file, worksheetPath)
	if err != nil {
		return "", drawingPackage{}, packageRelationships{}, nil, err
	}
	worksheetRelsPath := path.Join(path.Dir(worksheetPath), "_rels", path.Base(worksheetPath)+".rels")
	worksheetRels, err := packageXML[packageRelationships](s.file, worksheetRelsPath)
	if err != nil {
		return "", drawingPackage{}, packageRelationships{}, nil, err
	}
	drawingPath := resolvePackageTarget(worksheetPath, relationshipTarget(worksheetRels, worksheet.Drawing.RID))
	if drawingPath == "" {
		return "", drawingPackage{}, packageRelationships{}, nil, fmt.Errorf("worksheet %q has no drawing package", sheet)
	}
	drawingContent, err := packageBytes(s.file, drawingPath)
	if err != nil {
		return "", drawingPackage{}, packageRelationships{}, nil, err
	}
	var drawing drawingPackage
	if err := xml.Unmarshal(drawingContent, &drawing); err != nil {
		return "", drawingPackage{}, packageRelationships{}, nil, fmt.Errorf("decode drawing package %s: %w", drawingPath, err)
	}
	drawingRelsPath := path.Join(path.Dir(drawingPath), "_rels", path.Base(drawingPath)+".rels")
	drawingRels, err := packageXML[packageRelationships](s.file, drawingRelsPath)
	if err != nil {
		return "", drawingPackage{}, packageRelationships{}, nil, err
	}
	return drawingPath, drawing, drawingRels, drawingContent, nil
}

func packageBytes(file *excelize.File, name string) ([]byte, error) {
	value, ok := file.Pkg.Load(path.Clean(name))
	if !ok {
		return nil, fmt.Errorf("workbook package part %s not found", name)
	}
	content, ok := value.([]byte)
	if !ok {
		return nil, fmt.Errorf("workbook package part %s has an unsupported representation", name)
	}
	return append([]byte(nil), content...), nil
}

func packageXML[T any](file *excelize.File, name string) (T, error) {
	var result T
	content, err := packageBytes(file, name)
	if err != nil {
		return result, err
	}
	if err := xml.Unmarshal(content, &result); err != nil {
		return result, fmt.Errorf("decode workbook package part %s: %w", name, err)
	}
	return result, nil
}

func (s *Session) objectEndMarkerLocked(sheet string, from drawingMarker, width, height int) (drawingMarker, error) {
	to := from
	remainingWidth := width + from.ColumnOffset/emuPerPixel
	for remainingWidth > 0 {
		pixels, err := s.columnPixelsLocked(sheet, to.Column)
		if err != nil {
			return drawingMarker{}, err
		}
		if remainingWidth <= pixels {
			to.ColumnOffset = remainingWidth * emuPerPixel
			break
		}
		remainingWidth -= pixels
		to.Column++
		to.ColumnOffset = 0
	}
	remainingHeight := height + from.RowOffset/emuPerPixel
	for remainingHeight > 0 {
		pixels, err := s.rowPixelsLocked(sheet, to.Row)
		if err != nil {
			return drawingMarker{}, err
		}
		if remainingHeight <= pixels {
			to.RowOffset = remainingHeight * emuPerPixel
			break
		}
		remainingHeight -= pixels
		to.Row++
		to.RowOffset = 0
	}
	return to, nil
}

func (s *Session) drawingPixelSizeLocked(sheet string, from, to drawingMarker) (int, int, error) {
	width := to.ColumnOffset/emuPerPixel - from.ColumnOffset/emuPerPixel
	for column := from.Column; column < to.Column; column++ {
		pixels, err := s.columnPixelsLocked(sheet, column)
		if err != nil {
			return 0, 0, err
		}
		width += pixels
	}
	height := to.RowOffset/emuPerPixel - from.RowOffset/emuPerPixel
	for row := from.Row; row < to.Row; row++ {
		pixels, err := s.rowPixelsLocked(sheet, row)
		if err != nil {
			return 0, 0, err
		}
		height += pixels
	}
	return max(1, width), max(1, height), nil
}

func (s *Session) columnPixelsLocked(sheet string, zeroBasedColumn int) (int, error) {
	columnName, err := excelize.ColumnNumberToName(zeroBasedColumn + 1)
	if err != nil {
		return 0, fmt.Errorf("resolve chart column %d: %w", zeroBasedColumn+1, err)
	}
	columnWidth, err := s.file.GetColWidth(sheet, columnName)
	if err != nil {
		return 0, fmt.Errorf("read %s column %s width: %w", sheet, columnName, err)
	}
	return max(1, int(columnWidth*8+0.5)), nil
}

func (s *Session) rowPixelsLocked(sheet string, zeroBasedRow int) (int, error) {
	rowHeight, err := s.file.GetRowHeight(sheet, zeroBasedRow+1)
	if err != nil {
		return 0, fmt.Errorf("read %s row %d height: %w", sheet, zeroBasedRow+1, err)
	}
	return max(1, int(math.Ceil(4.0/3.4*rowHeight))), nil
}

func rewriteDrawingAnchor(content []byte, targetIndex int, from, to drawingMarker) ([]byte, error) {
	decoder := xml.NewDecoder(bytes.NewReader(content))
	var output bytes.Buffer
	encoder := xml.NewEncoder(&output)
	anchorIndex := -1
	anchorDepth := 0
	section := ""
	field := ""
	replaced := 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch value := token.(type) {
		case xml.StartElement:
			if value.Name.Local == "twoCellAnchor" {
				anchorIndex++
				if anchorIndex == targetIndex {
					anchorDepth = 1
				}
			} else if anchorDepth > 0 {
				anchorDepth++
				if anchorDepth == 2 && (value.Name.Local == "from" || value.Name.Local == "to") {
					section = value.Name.Local
				} else if section != "" && (value.Name.Local == "col" || value.Name.Local == "colOff" || value.Name.Local == "row" || value.Name.Local == "rowOff") {
					field = value.Name.Local
				}
			}
		case xml.CharData:
			if anchorDepth > 0 && section != "" && field != "" {
				marker := from
				if section == "to" {
					marker = to
				}
				number := markerValue(marker, field)
				token = xml.CharData(strconv.AppendInt(nil, int64(number), 10))
				replaced++
			}
		case xml.EndElement:
			if anchorDepth > 0 {
				if value.Name.Local == field {
					field = ""
				}
				if anchorDepth == 2 && value.Name.Local == section {
					section = ""
				}
				anchorDepth--
			}
		}
		if err := encoder.EncodeToken(token); err != nil {
			return nil, err
		}
	}
	if err := encoder.Flush(); err != nil {
		return nil, err
	}
	if replaced != 8 {
		return nil, fmt.Errorf("expected 8 anchor coordinates, replaced %d", replaced)
	}
	return output.Bytes(), nil
}

func markerValue(marker drawingMarker, field string) int {
	switch field {
	case "col":
		return marker.Column
	case "colOff":
		return marker.ColumnOffset
	case "row":
		return marker.Row
	case "rowOff":
		return marker.RowOffset
	default:
		return 0
	}
}

func rewriteChartTitle(content []byte, title string) ([]byte, error) {
	decoder := xml.NewDecoder(bytes.NewReader(content))
	var output bytes.Buffer
	encoder := xml.NewEncoder(&output)
	titleDepth := 0
	textDepth := 0
	foundTitle := false
	foundText := false
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch value := token.(type) {
		case xml.StartElement:
			if value.Name.Local == "title" && !foundTitle {
				foundTitle = true
				titleDepth = 1
			} else if titleDepth > 0 {
				titleDepth++
				if value.Name.Local == "t" {
					textDepth = titleDepth
				}
			}
		case xml.CharData:
			if textDepth > 0 {
				if !foundText {
					token = xml.CharData([]byte(title))
					foundText = true
				} else {
					token = xml.CharData(nil)
				}
			}
		case xml.EndElement:
			if titleDepth > 0 {
				if titleDepth == textDepth && value.Name.Local == "t" {
					textDepth = 0
				}
				titleDepth--
			}
		}
		if err := encoder.EncodeToken(token); err != nil {
			return nil, err
		}
	}
	if err := encoder.Flush(); err != nil {
		return nil, err
	}
	if !foundTitle || !foundText {
		return nil, errors.New("chart has no editable text title")
	}
	return output.Bytes(), nil
}
