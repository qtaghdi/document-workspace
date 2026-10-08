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
		{"Category", "Amount", "Score", "Choice", "", "", "Whole", "Range choice", "Text", "Scale", "Bar", "Duplicate", "Date", "Allowed values"},
		{"Alpha", 1250.5, 10, "A", "", "", 25, "North", "Alpha item", 10, 15, "Repeat", 45292, "North"},
		{"Beta", 845.25, 20, "B", "", "", 50, "South", "Beta item", 50, 45, "Unique", 45474, "South"},
		{"Gamma", 420.75, 30, "C", "", "", 75, "West", "Alpha note", 90, 85, "Repeat", 45657, "West"},
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

	wholeValidation := excelize.NewDataValidation(true)
	wholeValidation.Sqref = "G2:G10"
	must(wholeValidation.SetRange(1, 100, excelize.DataValidationTypeWhole, excelize.DataValidationOperatorBetween))
	wholeValidation.SetInput("Whole number", "Enter a whole number from 1 through 100.")
	wholeValidation.SetError(excelize.DataValidationErrorStyleStop, "Invalid number", "The value must be a whole number from 1 through 100.")
	must(workbook.AddDataValidation(sheet, wholeValidation))

	rangeListValidation := excelize.NewDataValidation(true)
	rangeListValidation.Sqref = "H2:H10"
	rangeListValidation.SetSqrefDropList("$N$2:$N$4")
	must(workbook.AddDataValidation(sheet, rangeListValidation))

	customValidation := excelize.NewDataValidation(true)
	customValidation.Sqref = "I2:I10"
	customValidation.Type = "custom"
	customValidation.Formula1 = "LEN(I2)&lt;=12"
	customValidation.SetError(excelize.DataValidationErrorStyleWarning, "Long text", "Use no more than 12 characters.")
	must(workbook.AddDataValidation(sheet, customValidation))

	dateValidation := excelize.NewDataValidation(true)
	dateValidation.Sqref = "M2:M10"
	must(dateValidation.SetRange(45292, 45657, excelize.DataValidationTypeDate, excelize.DataValidationOperatorBetween))
	must(workbook.AddDataValidation(sheet, dateValidation))

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
	must(workbook.SetConditionalFormat(sheet, "I2:I4", []excelize.ConditionalFormatOptions{{
		Type:     "text",
		Criteria: "containing",
		Value:    "Alpha",
		Format:   &conditionalStyle,
	}}))
	must(workbook.SetConditionalFormat(sheet, "J2:J4", []excelize.ConditionalFormatOptions{{
		Type:     "3_color_scale",
		Criteria: "=",
		MinType:  "min",
		MidType:  "percentile",
		MaxType:  "max",
		MinColor: "#F8696B",
		MidColor: "#FFEB84",
		MaxColor: "#63BE7B",
	}}))
	must(workbook.SetConditionalFormat(sheet, "K2:K4", []excelize.ConditionalFormatOptions{{
		Type:     "data_bar",
		Criteria: "=",
		MinType:  "min",
		MaxType:  "max",
		BarColor: "#638EC6",
	}}))
	must(workbook.SetConditionalFormat(sheet, "L2:L4", []excelize.ConditionalFormatOptions{{
		Type:     "duplicate",
		Criteria: "=",
		Format:   &conditionalStyle,
	}}))

	must(workbook.AddChart(sheet, "P8", &excelize.Chart{
		Type: excelize.Col,
		Series: []excelize.ChartSeries{{
			Name:       sheet + "!$B$1",
			Categories: sheet + "!$A$2:$A$4",
			Values:     sheet + "!$B$2:$B$4",
		}},
		Title: excelize.ChartTitle{Paragraph: []excelize.RichTextRun{{Text: "Amounts"}}},
	}))
	must(workbook.AddPictureFromBytes(sheet, "P2", &excelize.Picture{
		Extension: ".png",
		File:      compatibilityImage(),
		Format:    &excelize.GraphicOptions{AltText: "Compatibility marker"},
	}))

	must(workbook.SetCellValue(sheet, "E2", "Project link"))
	must(workbook.SetCellHyperLink(sheet, "E2", "https://example.com/document-workspace", "External"))
	must(workbook.SetDefinedName(&excelize.DefinedName{
		Name:     "CompatibilityAmounts",
		RefersTo: sheet + "!$B$2:$B$4",
		Scope:    "Workbook",
	}))
	must(workbook.AddComment(sheet, excelize.Comment{
		Cell:   "A2",
		Author: "document-workspace",
		Text:   "Compatibility corpus comment",
	}))

	output := filepath.Join("testdata", "xlsx", "compatibility", "feature-rich.xlsx")
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
