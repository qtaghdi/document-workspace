package workbook

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestApplyPersistsAndRejectsStaleRevision(t *testing.T) {
	path := filepath.Join(t.TempDir(), "book.xlsx")
	f := excelize.NewFile()
	if err := f.SetCellValue("Sheet1", "A1", "before"); err != nil {
		t.Fatal(err)
	}
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	session, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	snapshot, err := session.Apply(1, "ai", []Operation{{Type: "set_cell", Sheet: "Sheet1", Cell: "A1", Value: "after"}})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Revision != 2 {
		t.Fatalf("revision = %d, want 2", snapshot.Revision)
	}
	if _, err := session.Apply(1, "human", []Operation{{Type: "set_cell", Sheet: "Sheet1", Cell: "A2", Value: "stale"}}); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("error = %v, want revision conflict", err)
	}

	reopened, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	value, err := reopened.GetCellValue("Sheet1", "A1")
	if err != nil {
		t.Fatal(err)
	}
	if value != "after" {
		t.Fatalf("A1 = %q, want after", value)
	}
}

func TestReadRangeIncludesFormula(t *testing.T) {
	path := filepath.Join(t.TempDir(), "book.xlsx")
	f := excelize.NewFile()
	if err := f.SetCellFormula("Sheet1", "B2", "1+2"); err != nil {
		t.Fatal(err)
	}
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	session, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	got, err := session.ReadRange("Sheet1", "B2:B2")
	if err != nil {
		t.Fatal(err)
	}
	if got.Rows[0][0].Formula != "1+2" {
		t.Fatalf("formula = %q, want 1+2", got.Rows[0][0].Formula)
	}
}

func TestSnapshotReportsAndGrowsSheetDimensions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "book.xlsx")
	file := excelize.NewFile()
	if err := file.SetCellValue("Sheet1", "C250", "edge"); err != nil {
		t.Fatal(err)
	}
	if err := file.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	_ = file.Close()

	session, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	initial := session.Snapshot()
	if len(initial.SheetDimensions) != 1 {
		t.Fatalf("sheet dimensions = %#v", initial.SheetDimensions)
	}
	if got := initial.SheetDimensions[0]; got.Name != "Sheet1" || got.Rows != 250 || got.Columns != 3 {
		t.Fatalf("initial dimensions = %#v", got)
	}

	updated, err := session.Apply(1, "ai", []Operation{{
		Type: "set_cell", Sheet: "Sheet1", Cell: "D8148", Value: "expanded",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if got := updated.SheetDimensions[0]; got.Rows != 8148 || got.Columns != 4 {
		t.Fatalf("updated dimensions = %#v", got)
	}
}

func TestApplyPasteRangePersistsAsOneRevision(t *testing.T) {
	path := filepath.Join(t.TempDir(), "book.xlsx")
	file := excelize.NewFile()
	if err := file.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	_ = file.Close()

	session, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	snapshot, err := session.Apply(1, "ai", []Operation{{
		Type:  "paste_range",
		Sheet: "Sheet1",
		Range: "B2:C3",
		Cells: [][]CellInput{
			{{Value: "name"}, {Value: 4}},
			{{Value: "total"}, {Formula: "=C2*2"}},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Revision != 2 {
		t.Fatalf("revision = %d, want 2", snapshot.Revision)
	}

	got, err := session.ReadRange("Sheet1", "B2:C3")
	if err != nil {
		t.Fatal(err)
	}
	if got.Rows[0][0].Value != "name" || got.Rows[0][1].Value != "4" {
		t.Fatalf("first row = %#v", got.Rows[0])
	}
	if got.Rows[1][1].Formula != "C2*2" {
		t.Fatalf("formula = %q, want C2*2", got.Rows[1][1].Formula)
	}

	events, cancel := session.Subscribe(0)
	defer cancel()
	first := <-events
	second := <-events
	if first.Type != "presence.update" || first.Range != "B2:C3" {
		t.Fatalf("first event = %#v", first)
	}
	if second.Type != "range.commit" || len(second.Cells) != 4 || second.Revision != 2 {
		t.Fatalf("second event = %#v", second)
	}
}

func TestPasteRangeRejectsShapeMismatchWithoutMutation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "book.xlsx")
	file := excelize.NewFile()
	if err := file.SetCellValue("Sheet1", "A1", "safe"); err != nil {
		t.Fatal(err)
	}
	if err := file.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	_ = file.Close()

	session, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	_, err = session.Apply(1, "ai", []Operation{{
		Type:  "paste_range",
		Sheet: "Sheet1",
		Range: "A1:B2",
		Cells: [][]CellInput{{{Value: "incomplete"}}},
	}})
	if err == nil {
		t.Fatal("shape mismatch unexpectedly succeeded")
	}
	if session.Snapshot().Revision != 1 {
		t.Fatalf("revision = %d, want 1", session.Snapshot().Revision)
	}
	got, readErr := session.ReadRange("Sheet1", "A1:A1")
	if readErr != nil {
		t.Fatal(readErr)
	}
	if got.Rows[0][0].Value != "safe" {
		t.Fatalf("A1 = %q, want safe", got.Rows[0][0].Value)
	}
}

func TestPresenceDoesNotAdvanceRevision(t *testing.T) {
	path := filepath.Join(t.TempDir(), "book.xlsx")
	file := excelize.NewFile()
	if err := file.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	_ = file.Close()

	session, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	if err := session.UpdatePresence("ai", Presence{Sheet: "Sheet1", Range: "A1:C3", State: "selecting"}); err != nil {
		t.Fatal(err)
	}
	if session.Snapshot().Revision != 1 {
		t.Fatalf("revision = %d, want 1", session.Snapshot().Revision)
	}
	events, cancel := session.Subscribe(0)
	defer cancel()
	event := <-events
	if event.Actor != "ai" || event.Type != "presence.update" || event.Range != "A1:C3" {
		t.Fatalf("event = %#v", event)
	}
}

func TestStylesAndMergesRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "book.xlsx")
	file := excelize.NewFile()
	formatCode := "#,##0.00"
	styleID, err := file.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Bold: true, Family: "Arial", Size: 12, Color: "FF112233"},
		Fill:         excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"FFFFCC"}},
		CustomNumFmt: &formatCode,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := file.SetCellStyle("Sheet1", "A1", "A1", styleID); err != nil {
		t.Fatal(err)
	}
	if err := file.MergeCell("Sheet1", "A1", "B1"); err != nil {
		t.Fatal(err)
	}
	if err := file.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	_ = file.Close()

	session, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	initial, err := session.ReadRange("Sheet1", "A1:D2")
	if err != nil {
		t.Fatal(err)
	}
	if len(initial.Merges) != 1 || initial.Merges[0] != "A1:B1" {
		t.Fatalf("merges = %#v, want A1:B1", initial.Merges)
	}
	style := initial.Rows[0][0].Style
	if style == nil || !style.Bold || style.FontColor != "#112233" || style.NumberFormat != formatCode {
		t.Fatalf("style = %#v", style)
	}

	bold := false
	fill := "#00FF00"
	_, err = session.Apply(1, "ai", []Operation{
		{Type: "set_format", Sheet: "Sheet1", Range: "A1:B1", Format: &CellFormat{Bold: &bold, FillColor: &fill}},
		{Type: "unmerge_cells", Sheet: "Sheet1", Range: "A1:B1"},
		{Type: "merge_cells", Sheet: "Sheet1", Range: "C1:D1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := session.ReadRange("Sheet1", "A1:D2")
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Merges) != 1 || updated.Merges[0] != "C1:D1" {
		t.Fatalf("merges = %#v, want C1:D1", updated.Merges)
	}
	updatedStyle := updated.Rows[0][0].Style
	if updatedStyle == nil || updatedStyle.Bold || updatedStyle.FillColor != "#00FF00" {
		t.Fatalf("updated style = %#v", updatedStyle)
	}
}

func TestStructuralOperationsPersistAndPublish(t *testing.T) {
	path := filepath.Join(t.TempDir(), "book.xlsx")
	file := excelize.NewFile()
	if err := file.SetCellValue("Sheet1", "A2", "row"); err != nil {
		t.Fatal(err)
	}
	if err := file.SetCellValue("Sheet1", "B1", "column"); err != nil {
		t.Fatal(err)
	}
	if err := file.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	_ = file.Close()

	session, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	updated, err := session.Apply(1, "ai", []Operation{
		{Type: "insert_rows", Sheet: "Sheet1", Index: 2, Count: 2},
		{Type: "insert_columns", Sheet: "Sheet1", Index: 2, Count: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Revision != 2 {
		t.Fatalf("revision = %d, want 2", updated.Revision)
	}
	row, err := session.ReadRange("Sheet1", "A4:A4")
	if err != nil || row.Rows[0][0].Value != "row" {
		t.Fatalf("shifted row = %#v, error = %v", row, err)
	}
	column, err := session.ReadRange("Sheet1", "C1:C1")
	if err != nil || column.Rows[0][0].Value != "column" {
		t.Fatalf("shifted column = %#v, error = %v", column, err)
	}

	updated, err = session.Apply(2, "human", []Operation{
		{Type: "delete_rows", Sheet: "Sheet1", Index: 2, Count: 2},
		{Type: "delete_columns", Sheet: "Sheet1", Index: 2, Count: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.SheetDimensions[0].Rows != 2 || updated.SheetDimensions[0].Columns != 2 {
		t.Fatalf("restored dimensions = %#v", updated.SheetDimensions[0])
	}
	row, err = session.ReadRange("Sheet1", "A2:A2")
	if err != nil || row.Rows[0][0].Value != "row" {
		t.Fatalf("restored row = %#v, error = %v", row, err)
	}

	events, _ := session.EventsAfter(0)
	var structureEvents int
	for _, event := range events {
		if strings.HasPrefix(event.Type, "sheet.") {
			structureEvents++
			if event.Index < 1 || event.Count < 1 {
				t.Fatalf("invalid structural event = %#v", event)
			}
		}
	}
	if structureEvents != 4 {
		t.Fatalf("structural events = %d, want 4", structureEvents)
	}
}

func TestFeatureWarningsDetectBrowserCompatibilityGaps(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "compatibility", "feature-rich.xlsx")
	session, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	warnings := session.Snapshot().Warnings
	wanted := map[string]bool{
		"charts": false, "images": false, "conditional formatting": false, "data validation": false,
	}
	for _, warning := range warnings {
		if _, ok := wanted[warning.Feature]; ok {
			wanted[warning.Feature] = true
		}
	}
	for feature, found := range wanted {
		if !found {
			t.Fatalf("missing %s warning in %#v", feature, warnings)
		}
	}
	rangeData, err := session.ReadRange("Compatibility", "C2:D10")
	if err != nil {
		t.Fatal(err)
	}
	if len(rangeData.Validations) == 0 || rangeData.Validations[0].Type != "list" {
		t.Fatalf("validations = %#v", rangeData.Validations)
	}
	if len(rangeData.ConditionalFormatting) == 0 || rangeData.ConditionalFormatting[0].Style == nil {
		t.Fatalf("conditional formatting = %#v", rangeData.ConditionalFormatting)
	}
	objects, err := session.ReadSheetObjects("Compatibility")
	if err != nil {
		t.Fatal(err)
	}
	if len(objects.Images) != 1 || objects.Images[0].Data == "" || objects.Images[0].Width < 1 || objects.Images[0].Height < 1 {
		t.Fatalf("images = %#v", objects.Images)
	}
	if len(objects.Charts) != 1 || objects.Charts[0].Type != "bar" || len(objects.Charts[0].Series) != 1 {
		t.Fatalf("charts = %#v", objects.Charts)
	}
	if got := objects.Charts[0].Series[0].Values; len(got) != 3 || got[0] != 1250.5 || got[2] != 420.75 {
		t.Fatalf("chart values = %#v", got)
	}
}

func TestUndoAndRedoRestoreCommittedWorkbookRevisions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "book.xlsx")
	file := excelize.NewFile()
	if err := file.SetCellValue("Sheet1", "A1", "before"); err != nil {
		t.Fatal(err)
	}
	if err := file.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	_ = file.Close()

	session, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	changed, err := session.Apply(1, "human", []Operation{{Type: "set_cell", Sheet: "Sheet1", Cell: "A1", Value: "after"}})
	if err != nil {
		t.Fatal(err)
	}
	if !changed.CanUndo || changed.CanRedo {
		t.Fatalf("history state after edit = %#v", changed)
	}
	undone, err := session.RestoreHistory(2, "human", "undo")
	if err != nil {
		t.Fatal(err)
	}
	if undone.Revision != 3 || undone.CanUndo || !undone.CanRedo {
		t.Fatalf("history state after undo = %#v", undone)
	}
	value, err := session.ReadRange("Sheet1", "A1")
	if err != nil || value.Rows[0][0].Value != "before" {
		t.Fatalf("undo value = %#v, error = %v", value, err)
	}
	redone, err := session.RestoreHistory(3, "human", "redo")
	if err != nil {
		t.Fatal(err)
	}
	if redone.Revision != 4 || !redone.CanUndo || redone.CanRedo {
		t.Fatalf("history state after redo = %#v", redone)
	}
	value, err = session.ReadRange("Sheet1", "A1")
	if err != nil || value.Rows[0][0].Value != "after" {
		t.Fatalf("redo value = %#v, error = %v", value, err)
	}
}

func TestUndoAndRedoHistorySurvivesSessionRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "book.xlsx")
	file := excelize.NewFile()
	if err := file.SetCellValue("Sheet1", "A1", "before"); err != nil {
		t.Fatal(err)
	}
	if err := file.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	_ = file.Close()

	first, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := first.Apply(1, "human", []Operation{{Type: "set_cell", Sheet: "Sheet1", Cell: "A1", Value: "after"}})
	if err != nil {
		t.Fatal(err)
	}
	if changed.Revision != 2 || !changed.CanUndo {
		t.Fatalf("history after edit = %#v", changed)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot := second.Snapshot(); snapshot.Revision != 2 || !snapshot.CanUndo {
		t.Fatalf("reopened history = %#v", snapshot)
	}
	undone, err := second.RestoreHistory(2, "human", "undo")
	if err != nil {
		t.Fatal(err)
	}
	if undone.Revision != 3 || !undone.CanRedo {
		t.Fatalf("history after durable undo = %#v", undone)
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}

	third, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer third.Close()
	if snapshot := third.Snapshot(); snapshot.Revision != 3 || !snapshot.CanRedo {
		t.Fatalf("reopened redo history = %#v", snapshot)
	}
	redone, err := third.RestoreHistory(3, "human", "redo")
	if err != nil {
		t.Fatal(err)
	}
	if redone.Revision != 4 || !redone.CanUndo {
		t.Fatalf("history after durable redo = %#v", redone)
	}
	value, err := third.ReadRange("Sheet1", "A1")
	if err != nil || value.Rows[0][0].Value != "after" {
		t.Fatalf("redo value = %#v, error = %v", value, err)
	}
}

func TestExternalWorkbookReplacementResetsDurableHistory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "book.xlsx")
	file := excelize.NewFile()
	if err := file.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	_ = file.Close()

	session, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.Apply(1, "human", []Operation{{Type: "set_cell", Sheet: "Sheet1", Cell: "A1", Value: "session"}}); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}

	replacement := excelize.NewFile()
	if err := replacement.SetCellValue("Sheet1", "A1", "external"); err != nil {
		t.Fatal(err)
	}
	if err := replacement.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	_ = replacement.Close()

	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	snapshot := reopened.Snapshot()
	if snapshot.Revision != 1 || snapshot.CanUndo || snapshot.CanRedo {
		t.Fatalf("history after external replacement = %#v", snapshot)
	}
}
