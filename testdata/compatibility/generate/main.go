package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"path/filepath"

	"github.com/xuri/excelize/v2"
)

const sheet = "Compatibility"

func main() {
	workbook := excelize.NewFile()
	defer workbook.Close()
	must(workbook.SetSheetName("Sheet1", sheet))

	rows := [][]any{
		{"Category", "Amount", "Score", "Choice"},
		{"Alpha", 1250.5, 10, "A"},
		{"Beta", 845.25, 20, "B"},
		{"Gamma", 420.75, 30, "C"},
	}
	for index, row := range rows {
		cell, err := excelize.CoordinatesToCellName(1, index+1)
		must(err)
		must(workbook.SetSheetRow(sheet, cell, &row))
	}
	must(workbook.SetCellFormula(sheet, "B5", "SUM(B2:B4)"))
	must(workbook.SetCellFormula(sheet, "F2", "'[external-source.xlsx]Sheet1'!$A$1"))

	numberFormat := "$#,##0.00"
	styleID, err := workbook.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Bold: true, Family: "Arial", Size: 12, Color: "FF1F4E78"},
		Fill:         excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"FFDDEBF7"}},
		CustomNumFmt: &numberFormat,
	})
	must(err)
	must(workbook.SetCellStyle(sheet, "B2", "B4", styleID))
	must(workbook.MergeCell(sheet, "A7", "C7"))
	must(workbook.SetCellValue(sheet, "A7", "Merged heading"))

	validation := excelize.NewDataValidation(true)
	validation.Sqref = "D2:D10"
	must(validation.SetDropList([]string{"A", "B", "C"}))
	must(workbook.AddDataValidation(sheet, validation))

	conditionalStyle, err := workbook.NewConditionalStyle(&excelize.Style{
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"FFC6EFCE"}},
	})
	must(err)
	must(workbook.SetConditionalFormat(sheet, "C2:C4", []excelize.ConditionalFormatOptions{{
		Type:     "cell",
		Criteria: ">",
		Format:   &conditionalStyle,
		Value:    "15",
	}}))

	must(workbook.AddChart(sheet, "H8", &excelize.Chart{
		Type: excelize.Col,
		Series: []excelize.ChartSeries{{
			Name:       sheet + "!$B$1",
			Categories: sheet + "!$A$2:$A$4",
			Values:     sheet + "!$B$2:$B$4",
		}},
		Title: excelize.ChartTitle{Paragraph: []excelize.RichTextRun{{Text: "Amounts"}}},
	}))
	must(workbook.AddPictureFromBytes(sheet, "H2", &excelize.Picture{
		Extension: ".png",
		File:      compatibilityImage(),
		Format:    &excelize.GraphicOptions{AltText: "Compatibility marker"},
	}))

	must(workbook.SetCellValue(sheet, "E2", "Project link"))
	must(workbook.SetCellHyperLink(sheet, "E2", "https://example.com/xlsx-viewer", "External"))
	must(workbook.SetDefinedName(&excelize.DefinedName{
		Name:     "CompatibilityAmounts",
		RefersTo: sheet + "!$B$2:$B$4",
		Scope:    "Workbook",
	}))
	must(workbook.AddComment(sheet, excelize.Comment{
		Cell:   "A2",
		Author: "xlsx-viewer",
		Text:   "Compatibility corpus comment",
	}))

	output := filepath.Join("testdata", "compatibility", "feature-rich.xlsx")
	must(workbook.SaveAs(output))
	must(os.Chmod(output, 0o644))
}

func compatibilityImage() []byte {
	canvas := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			canvas.Set(x, y, color.RGBA{R: uint8(30 + x*8), G: uint8(90 + y*6), B: 180, A: 255})
		}
	}
	var encoded bytes.Buffer
	must(png.Encode(&encoded, canvas))
	return encoded.Bytes()
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
