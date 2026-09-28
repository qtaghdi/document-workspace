package httpapi

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/qtaghdi/xlsx-viewer/internal/workbook"
	"github.com/xuri/excelize/v2"
)

func TestEmbeddedUIRequiresSessionAndServesAssets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "book.xlsx")
	file := excelize.NewFile()
	if err := file.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	session, err := workbook.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	handler := New(session, "test-token").Handler()

	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("GET / without session = %d, want %d", unauthorized.Code, http.StatusUnauthorized)
	}

	index := httptest.NewRecorder()
	indexRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	indexRequest.AddCookie(&http.Cookie{Name: "xlsx_session", Value: "test-token"})
	handler.ServeHTTP(index, indexRequest)
	if index.Code != http.StatusOK {
		t.Fatalf("GET / with session = %d, want %d", index.Code, http.StatusOK)
	}
	if !strings.Contains(index.Body.String(), "Workbook editor") {
		t.Fatal("embedded UI is missing the workbook editor")
	}

	assetPath := regexp.MustCompile(`/assets/[^\"]+\.js`).FindString(index.Body.String())
	if assetPath == "" {
		t.Fatal("embedded UI does not reference a JavaScript asset")
	}

	asset := httptest.NewRecorder()
	assetRequest := httptest.NewRequest(http.MethodGet, assetPath, nil)
	assetRequest.AddCookie(&http.Cookie{Name: "xlsx_session", Value: "test-token"})
	handler.ServeHTTP(asset, assetRequest)
	if asset.Code != http.StatusOK {
		t.Fatalf("GET %s with session = %d, want %d", assetPath, asset.Code, http.StatusOK)
	}
	if !strings.Contains(asset.Header().Get("Content-Type"), "javascript") {
		t.Fatalf("asset content type = %q, want JavaScript", asset.Header().Get("Content-Type"))
	}
}
