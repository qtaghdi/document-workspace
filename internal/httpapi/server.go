package httpapi

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"

	"github.com/qtaghdi/xlsx-viewer/internal/workbook"
)

//go:embed static
var staticFS embed.FS

type Server struct {
	session      *workbook.Session
	browserToken string
	mcpToken     string
	mux          *http.ServeMux
}

func New(session *workbook.Session, token string) *Server {
	return NewWithTokens(session, token, token)
}

func NewWithTokens(session *workbook.Session, browserToken, mcpToken string) *Server {
	s := &Server{session: session, browserToken: browserToken, mcpToken: mcpToken, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return securityHeaders(s.mux)
}

func (s *Server) routes() {
	assets, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(fmt.Sprintf("prepare embedded UI: %v", err))
	}
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	s.mux.Handle("/mcp", s.requireBearer(s.mcpHandler()))
	s.mux.HandleFunc("GET /{$}", s.index)
	s.mux.Handle("GET /assets/", s.requireSession(http.FileServer(http.FS(assets))))
	s.mux.Handle("GET /api/workbook", s.requireSession(http.HandlerFunc(s.getWorkbook)))
	s.mux.Handle("GET /api/range", s.requireSession(http.HandlerFunc(s.getRange)))
	s.mux.Handle("POST /api/operations", s.requireSession(http.HandlerFunc(s.applyOperations)))
	s.mux.Handle("POST /api/presence", s.requireSession(http.HandlerFunc(s.updatePresence)))
	s.mux.Handle("GET /api/events", s.requireSession(http.HandlerFunc(s.events)))
}
