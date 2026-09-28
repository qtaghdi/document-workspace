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
