package workbook

import (
	"errors"
	"path/filepath"
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
