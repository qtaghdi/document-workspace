package httpapi

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/qtaghdi/document-workspace/internal/formats/xlsx"
	"github.com/xuri/excelize/v2"
)

func recoveryAPI(t *testing.T) (*Server, *xlsx.Session, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "book.xlsx")
	f := excelize.NewFile()
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	s, err := xlsx.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return NewWithTokens(s, "browser", "mcp"), s, path
}

func TestExternalReplacementReturnsHTTPConflict(t *testing.T) {
	api, _, path := recoveryAPI(t)
	if err := os.WriteFile(path, []byte("external"), 0o600); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "http://localhost/api/operations", strings.NewReader(`{"baseRevision":1,"operations":[{"type":"set_cell","sheet":"Sheet1","cell":"A1","value":"overwrite"}]}`))
	req.AddCookie(&http.Cookie{Name: "xlsx_session", Value: "browser"})
	req.Header.Set("Origin", "http://localhost")
	response := httptest.NewRecorder()
	api.Handler().ServeHTTP(response, req)
	if response.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	content, _ := os.ReadFile(path)
	if !bytes.Equal(content, []byte("external")) {
		t.Fatal("external file changed")
	}
}

func TestSSESessionMismatchRequiresResync(t *testing.T) {
	api, _, _ := recoveryAPI(t)
	server := httptest.NewServer(api.Handler())
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", server.URL+"/api/events?after=0&session=previous-process", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(&http.Cookie{Name: "xlsx_session", Value: "browser"})
	response, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	buffer := make([]byte, 2048)
	var result strings.Builder
	for !strings.Contains(result.String(), `"state":"resync"`) {
		n, err := response.Body.Read(buffer)
		result.Write(buffer[:n])
		if err != nil && err != io.EOF {
			t.Fatal(err)
		}
		if err == io.EOF {
			break
		}
	}
	if !strings.Contains(result.String(), `"state":"resync"`) {
		t.Fatalf("no resync: %s", result.String())
	}
}
