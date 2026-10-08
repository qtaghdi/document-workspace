package xlsx

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestOpenRejectsInvalidInputsWithoutMutation(t *testing.T) {
	t.Run("corrupt", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "corrupt.xlsx")
		content := []byte("not an OOXML package")
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Open(path); !errors.Is(err, ErrUnsupportedWorkbook) {
			t.Fatalf("error = %v, want unsupported workbook", err)
		}
		assertFileContent(t, path, content)
	})

	t.Run("non regular", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "directory.xlsx")
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := Open(path); err == nil || !strings.Contains(err.Error(), "regular file") {
			t.Fatalf("error = %v, want regular file rejection", err)
		}
	})

	t.Run("compressed size", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "oversized.xlsx")
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := file.Truncate(maxWorkbookBytes + 1); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err := Open(path); err == nil || !strings.Contains(err.Error(), "workbook exceeds") {
			t.Fatalf("error = %v, want compressed size rejection", err)
		}
		info, err := os.Stat(path)
		if err != nil || info.Size() != maxWorkbookBytes+1 {
			t.Fatalf("oversized source changed: info = %#v, error = %v", info, err)
		}
	})
}

func TestWorkbookPackageBoundaries(t *testing.T) {
	valid := createTestPackage(t, []packagePart{
		{name: "[Content_Types].xml", content: "types"},
		{name: "xl/workbook.xml", content: "workbook"},
	})
	if err := validateWorkbookPackage(valid, workbookPackageLimits{parts: 2, partBytes: 8, expandedBytes: 13}); err != nil {
		t.Fatalf("valid package rejected: %v", err)
	}

	tests := []struct {
		name    string
		parts   []packagePart
		limits  workbookPackageLimits
		message string
	}{
		{
			name:    "part count",
			parts:   []packagePart{{name: "[Content_Types].xml"}, {name: "xl/workbook.xml"}, {name: "xl/worksheets/sheet1.xml"}},
			limits:  workbookPackageLimits{parts: 2, partBytes: 100, expandedBytes: 100},
			message: "contains 3 parts",
		},
		{
			name:    "single expanded part",
			parts:   []packagePart{{name: "[Content_Types].xml"}, {name: "xl/workbook.xml", content: "12345"}},
			limits:  workbookPackageLimits{parts: 2, partBytes: 4, expandedBytes: 100},
			message: "expands to 5 bytes",
		},
		{
			name:    "total expanded bytes",
			parts:   []packagePart{{name: "[Content_Types].xml", content: "123"}, {name: "xl/workbook.xml", content: "456"}},
			limits:  workbookPackageLimits{parts: 2, partBytes: 3, expandedBytes: 5},
			message: "expanded package exceeds 5 bytes",
		},
		{
			name:    "duplicate part",
			parts:   []packagePart{{name: "[Content_Types].xml"}, {name: "xl/workbook.xml"}, {name: "xl/workbook.xml"}},
			limits:  workbookPackageLimits{parts: 3, partBytes: 100, expandedBytes: 100},
			message: "duplicate package part",
		},
		{
			name:    "unsafe path",
			parts:   []packagePart{{name: "[Content_Types].xml"}, {name: "xl/workbook.xml"}, {name: "../payload"}},
			limits:  workbookPackageLimits{parts: 3, partBytes: 100, expandedBytes: 100},
			message: "unsafe package part name",
		},
		{
			name:    "encrypted part",
			parts:   []packagePart{{name: "[Content_Types].xml"}, {name: "xl/workbook.xml", encrypted: true}},
			limits:  workbookPackageLimits{parts: 2, partBytes: 100, expandedBytes: 100},
			message: "encrypted package part",
		},
		{
			name:    "missing workbook",
			parts:   []packagePart{{name: "[Content_Types].xml"}},
			limits:  workbookPackageLimits{parts: 1, partBytes: 100, expandedBytes: 100},
			message: "required OOXML workbook parts are missing",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			content := createTestPackage(t, test.parts)
			err := validateWorkbookPackage(content, test.limits)
			if !errors.Is(err, ErrUnsupportedWorkbook) || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("error = %v, want %q", err, test.message)
			}
		})
	}
}

func TestOpenAcceptsGeneratedWorkbookAfterPackageValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "valid.xlsx")
	file := excelize.NewFile()
	if err := file.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	_ = file.Close()
	session, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	_ = session.Close()
}

func TestSaveRejectsCandidateOutsidePackageLimitsWithoutMutation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "book.xlsx")
	file := excelize.NewFile()
	if err := file.SetCellValue("Sheet1", "A1", "original"); err != nil {
		t.Fatal(err)
	}
	if err := file.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	_ = file.Close()
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	session, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	session.packageLimits = workbookPackageLimits{
		parts:         maxWorkbookParts,
		partBytes:     maxWorkbookPartBytes,
		expandedBytes: 1,
	}

	if _, err := session.Apply(1, "human", []Operation{{Type: "set_cell", Sheet: "Sheet1", Cell: "A1", Value: "rejected"}}); err == nil || !strings.Contains(err.Error(), "validate workbook candidate") {
		t.Fatalf("error = %v, want candidate validation failure", err)
	}
	if session.Snapshot().Revision != 1 {
		t.Fatalf("revision = %d, want 1", session.Snapshot().Revision)
	}
	cell, err := session.ReadRange("Sheet1", "A1")
	if err != nil || cell.Rows[0][0].Value != "original" {
		t.Fatalf("authoritative value = %#v, error = %v", cell, err)
	}
	assertFileContent(t, path, original)
}

type packagePart struct {
	name      string
	content   string
	encrypted bool
}

func createTestPackage(t *testing.T, parts []packagePart) []byte {
	t.Helper()
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	for _, part := range parts {
		header := &zip.FileHeader{Name: part.name, Method: zip.Deflate}
		if part.encrypted {
			header.Flags |= 0x1
		}
		writer, err := archive.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write([]byte(part.content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func assertFileContent(t *testing.T, path string, expected []byte) {
	t.Helper()
	actual, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, expected) {
		t.Fatal("source file changed after rejected open")
	}
}
