package xlsx

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"path"
	"strings"
)

const maxWorkbookParts = 10_000
const maxWorkbookPartBytes = 256 << 20
const maxExpandedWorkbookBytes = 512 << 20

var ErrUnsupportedWorkbook = errors.New("unsupported workbook package")

type workbookPackageLimits struct {
	parts         int
	partBytes     uint64
	expandedBytes uint64
}

func defaultPackageLimits() workbookPackageLimits {
	return workbookPackageLimits{
		parts:         maxWorkbookParts,
		partBytes:     maxWorkbookPartBytes,
		expandedBytes: maxExpandedWorkbookBytes,
	}
}

func validateWorkbookBytes(content []byte, limits workbookPackageLimits) error {
	if len(content) > maxWorkbookBytes {
		return fmt.Errorf("workbook exceeds %d bytes", maxWorkbookBytes)
	}
	return validateWorkbookPackage(content, limits)
}

// validateWorkbookPackage rejects archive structures that can make parsing
// ambiguous or consume unbounded memory before Excelize sees the package.
func validateWorkbookPackage(content []byte, limits workbookPackageLimits) error {
	archive, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return fmt.Errorf("%w: invalid ZIP container: %v", ErrUnsupportedWorkbook, err)
	}
	if len(archive.File) > limits.parts {
		return fmt.Errorf("%w: package contains %d parts, limit is %d", ErrUnsupportedWorkbook, len(archive.File), limits.parts)
	}

	seen := make(map[string]struct{}, len(archive.File))
	var expanded uint64
	var hasContentTypes bool
	var hasWorkbook bool
	for _, part := range archive.File {
		name := part.Name
		canonicalName := strings.TrimSuffix(name, "/")
		if canonicalName == "" || strings.Contains(name, "\\") || strings.HasPrefix(name, "/") || path.Clean(canonicalName) != canonicalName || strings.HasPrefix(canonicalName, "../") {
			return fmt.Errorf("%w: unsafe package part name %q", ErrUnsupportedWorkbook, name)
		}
		if _, exists := seen[canonicalName]; exists {
			return fmt.Errorf("%w: duplicate package part %q", ErrUnsupportedWorkbook, name)
		}
		seen[canonicalName] = struct{}{}
		if part.Flags&0x1 != 0 {
			return fmt.Errorf("%w: encrypted package part %q", ErrUnsupportedWorkbook, name)
		}
		if part.UncompressedSize64 > limits.partBytes {
			return fmt.Errorf("%w: package part %q expands to %d bytes, limit is %d", ErrUnsupportedWorkbook, name, part.UncompressedSize64, limits.partBytes)
		}
		if part.UncompressedSize64 > limits.expandedBytes-expanded {
			return fmt.Errorf("%w: expanded package exceeds %d bytes", ErrUnsupportedWorkbook, limits.expandedBytes)
		}
		expanded += part.UncompressedSize64
		hasContentTypes = hasContentTypes || name == "[Content_Types].xml"
		hasWorkbook = hasWorkbook || name == "xl/workbook.xml"
	}
	if !hasContentTypes || !hasWorkbook {
		return fmt.Errorf("%w: required OOXML workbook parts are missing", ErrUnsupportedWorkbook)
	}
	return nil
}
