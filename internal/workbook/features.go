package workbook

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"strings"
)

const maxFeatureScanBytes = 8 << 20

// inspectFeatureWarnings detects preserved XLSX features that the browser does
// not render completely. The warning prevents visual absence from being
// mistaken for data loss while Excelize continues to preserve the package.
func inspectFeatureWarnings(path string) ([]FeatureWarning, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("inspect workbook features: %w", err)
	}
	defer reader.Close()

	detected := map[string]bool{}
	for _, entry := range reader.File {
		name := strings.ToLower(entry.Name)
		switch {
		case strings.HasPrefix(name, "xl/charts/"):
			detected["charts"] = true
		case strings.HasPrefix(name, "xl/media/"):
			detected["images"] = true
		case strings.HasPrefix(name, "xl/externallinks/"):
			detected["external links"] = true
		case strings.HasSuffix(name, "vbaproject.bin"):
			detected["macros"] = true
		}
		if !strings.HasPrefix(name, "xl/worksheets/") || !strings.HasSuffix(name, ".xml") {
			continue
		}
		file, openErr := entry.Open()
		if openErr != nil {
			return nil, fmt.Errorf("inspect workbook feature part %s: %w", entry.Name, openErr)
		}
		content, readErr := io.ReadAll(io.LimitReader(file, maxFeatureScanBytes))
		closeErr := file.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read workbook feature part %s: %w", entry.Name, readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close workbook feature part %s: %w", entry.Name, closeErr)
		}
		if bytes.Contains(content, []byte("<conditionalFormatting")) {
			detected["conditional formatting"] = true
		}
		if bytes.Contains(content, []byte("<dataValidations")) {
			detected["data validation"] = true
		}
	}

	features := []string{"charts", "images", "conditional formatting", "data validation", "external links", "macros"}
	warnings := make([]FeatureWarning, 0, len(features))
	for _, feature := range features {
		if detected[feature] {
			warnings = append(warnings, FeatureWarning{
				Feature: feature,
				Message: feature + " is preserved when possible but is not fully rendered or editable in the browser",
			})
		}
	}
	return warnings, nil
}
