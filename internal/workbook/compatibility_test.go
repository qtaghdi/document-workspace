package workbook

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

type compatibilityInventory struct {
	Formula               string
	ExternalFormula       string
	Style                 string
	Merges                []string
	Validations           []string
	ConditionalFormatting []string
	Pictures              []string
	Hyperlinks            []string
	DefinedNames          []string
	Comments              []string
	PackageParts          []string
}

func TestCompatibilityCorpusPreservesUnrelatedFeatures(t *testing.T) {
	fixtures := []string{"feature-rich.xlsx", "libreoffice.xlsx"}
	for _, fixture := range fixtures {
		t.Run(fixture, func(t *testing.T) {
			verifyCompatibilityFixture(t, fixture)
		})
	}
}

func verifyCompatibilityFixture(t *testing.T, fixture string) {
	t.Helper()
	source := compatibilityFixturePath(t, fixture)
	before := inspectCompatibilityWorkbook(t, source)
	if fixture == "feature-rich.xlsx" {
		assertCompatibilityBaseline(t, before)
	} else if before.Formula == "" || len(before.Merges) == 0 || !strings.Contains(before.Style, "bold=true") {
		t.Fatalf("producer fixture is missing core compatibility features: %#v", before)
	}

	target := filepath.Join(t.TempDir(), fixture)
	contents, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, contents, 0o600); err != nil {
		t.Fatal(err)
	}

	session, err := Open(target)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.Apply(1, "compatibility-test", []Operation{{
		Type: "set_cell", Sheet: "Compatibility", Cell: "Z50", Value: "round-trip marker",
	}}); err != nil {
		_ = session.Close()
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}

	after := inspectCompatibilityWorkbook(t, target)
	if !reflect.DeepEqual(after, before) {
		beforeJSON, _ := json.MarshalIndent(before, "", "  ")
		afterJSON, _ := json.MarshalIndent(after, "", "  ")
		t.Fatalf("compatibility inventory changed after unrelated edit\nbefore: %s\nafter: %s", beforeJSON, afterJSON)
	}

	reopened, err := excelize.OpenFile(target)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	marker, err := reopened.GetCellValue("Compatibility", "Z50")
	if err != nil {
		t.Fatal(err)
	}
	if marker != "round-trip marker" {
		t.Fatalf("round-trip marker = %q", marker)
	}
}

func compatibilityFixturePath(t *testing.T, name string) string {
	t.Helper()
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve compatibility test path")
	}
	return filepath.Join(filepath.Dir(current), "..", "..", "testdata", "compatibility", name)
}

func inspectCompatibilityWorkbook(t *testing.T, path string) compatibilityInventory {
	t.Helper()
	file, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	formula, err := file.GetCellFormula("Compatibility", "B5")
	if err != nil {
		t.Fatal(err)
	}
	externalFormula, err := file.GetCellFormula("Compatibility", "F2")
	if err != nil {
		t.Fatal(err)
	}
	inventory := compatibilityInventory{
		Formula:               formula,
		ExternalFormula:       externalFormula,
		Style:                 compatibilityStyle(t, file),
		Merges:                compatibilityMerges(t, file),
		Validations:           compatibilityValidations(t, file),
		ConditionalFormatting: compatibilityConditionalFormatting(t, file),
		Pictures:              compatibilityPictures(t, file),
		Hyperlinks:            compatibilityHyperlinks(t, file),
		DefinedNames:          compatibilityDefinedNames(file),
		Comments:              compatibilityComments(t, file),
		PackageParts:          compatibilityPackageParts(t, path),
	}
	return inventory
}

func compatibilityStyle(t *testing.T, file *excelize.File) string {
	t.Helper()
	styleID, err := file.GetCellStyle("Compatibility", "B2")
	if err != nil {
		t.Fatal(err)
	}
	style, err := file.GetStyle(styleID)
	if err != nil {
		t.Fatal(err)
	}
	custom := ""
	if style.CustomNumFmt != nil {
		custom = *style.CustomNumFmt
	}
	font := excelize.Font{}
	if style.Font != nil {
		font = *style.Font
	}
	return fmt.Sprintf("bold=%t;family=%s;size=%.2f;color=%s;fill=%v;custom=%s", font.Bold, font.Family, font.Size, font.Color, style.Fill.Color, custom)
}

func compatibilityMerges(t *testing.T, file *excelize.File) []string {
	t.Helper()
	merges, err := file.GetMergeCells("Compatibility")
	if err != nil {
		t.Fatal(err)
	}
	result := make([]string, 0, len(merges))
	for index := range merges {
		result = append(result, merges[index].GetStartAxis()+":"+merges[index].GetEndAxis())
	}
	sort.Strings(result)
	return result
}

func compatibilityValidations(t *testing.T, file *excelize.File) []string {
	t.Helper()
	validations, err := file.GetDataValidations("Compatibility")
	if err != nil {
		t.Fatal(err)
	}
	result := make([]string, 0, len(validations))
	for _, validation := range validations {
		result = append(result, fmt.Sprintf("%s;%s;%s", validation.Sqref, validation.Type, validation.Formula1))
	}
	sort.Strings(result)
	return result
}

func compatibilityConditionalFormatting(t *testing.T, file *excelize.File) []string {
	t.Helper()
	formats, err := file.GetConditionalFormats("Compatibility")
	if err != nil {
		t.Fatal(err)
	}
	result := make([]string, 0, len(formats))
	for ref, options := range formats {
		encoded, err := json.Marshal(options)
		if err != nil {
			t.Fatal(err)
		}
		result = append(result, ref+";"+string(encoded))
	}
	sort.Strings(result)
	return result
}

func compatibilityPictures(t *testing.T, file *excelize.File) []string {
	t.Helper()
	pictures, err := file.GetPictures("Compatibility", "H2")
	if err != nil {
		t.Fatal(err)
	}
	result := make([]string, 0, len(pictures))
	for _, picture := range pictures {
		hash := sha256.Sum256(picture.File)
		result = append(result, picture.Extension+";"+hex.EncodeToString(hash[:]))
	}
	sort.Strings(result)
	return result
}

func compatibilityHyperlinks(t *testing.T, file *excelize.File) []string {
	t.Helper()
	hasLink, target, err := file.GetCellHyperLink("Compatibility", "E2")
	if err != nil {
		t.Fatal(err)
	}
	if !hasLink {
		return nil
	}
	return []string{"E2;" + target}
}

func compatibilityDefinedNames(file *excelize.File) []string {
	result := make([]string, 0)
	for _, name := range file.GetDefinedName() {
		if name.Name == "CompatibilityAmounts" {
			result = append(result, fmt.Sprintf("%s;%s;%s", name.Name, name.Scope, name.RefersTo))
		}
	}
	sort.Strings(result)
	return result
}

func compatibilityComments(t *testing.T, file *excelize.File) []string {
	t.Helper()
	comments, err := file.GetComments("Compatibility")
	if err != nil {
		t.Fatal(err)
	}
	result := make([]string, 0, len(comments))
	for _, comment := range comments {
		result = append(result, fmt.Sprintf("%s;%s;%s", comment.Cell, comment.Author, comment.Text))
	}
	sort.Strings(result)
	return result
}

func compatibilityPackageParts(t *testing.T, path string) []string {
	t.Helper()
	archive, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	result := make([]string, 0)
	for _, entry := range archive.File {
		if !strings.HasPrefix(entry.Name, "xl/charts/") && !strings.HasPrefix(entry.Name, "xl/media/") {
			continue
		}
		reader, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		hash := sha256.New()
		if _, err := io.Copy(hash, reader); err != nil {
			_ = reader.Close()
			t.Fatal(err)
		}
		if err := reader.Close(); err != nil {
			t.Fatal(err)
		}
		result = append(result, entry.Name+";"+hex.EncodeToString(hash.Sum(nil)))
	}
	sort.Strings(result)
	return result
}

func assertCompatibilityBaseline(t *testing.T, inventory compatibilityInventory) {
	t.Helper()
	if inventory.Formula != "SUM(B2:B4)" {
		t.Fatalf("formula = %q", inventory.Formula)
	}
	if inventory.ExternalFormula != "'[external-source.xlsx]Sheet1'!$A$1" {
		t.Fatalf("external formula = %q", inventory.ExternalFormula)
	}
	checks := map[string]int{
		"merges":                 len(inventory.Merges),
		"validations":            len(inventory.Validations),
		"conditional formatting": len(inventory.ConditionalFormatting),
		"pictures":               len(inventory.Pictures),
		"hyperlinks":             len(inventory.Hyperlinks),
		"defined names":          len(inventory.DefinedNames),
		"comments":               len(inventory.Comments),
		"chart and media parts":  len(inventory.PackageParts),
	}
	for feature, count := range checks {
		if count == 0 {
			t.Fatalf("fixture has no %s", feature)
		}
	}
	if !strings.Contains(inventory.Style, "bold=true") || !strings.Contains(inventory.Style, "custom=$#,##0.00") {
		t.Fatalf("style = %q", inventory.Style)
	}
}
