package httpapi

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/qtaghdi/xlsx-viewer/internal/workbook"
)

//go:embed static/index.html static/assets/*
var staticFS embed.FS

type Server struct {
	session *workbook.Session
	token   string
	mux     *http.ServeMux
}

type readRangeInput struct {
	Sheet string `json:"sheet" jsonschema:"Worksheet name"`
	Range string `json:"range" jsonschema:"A1-style range, for example A1:H40"`
}

type applyInput struct {
	BaseRevision uint64               `json:"baseRevision" jsonschema:"Revision returned by get_workbook or read_range"`
	Operations   []workbook.Operation `json:"operations" jsonschema:"Ordered workbook edits to apply atomically"`
}

type applyResponse struct {
	Workbook workbook.Snapshot `json:"workbook"`
	Applied  int               `json:"applied"`
}

func New(session *workbook.Session, token string) *Server {
	s := &Server{session: session, token: token, mux: http.NewServeMux()}
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
	s.mux.Handle("GET /api/events", s.requireSession(http.HandlerFunc(s.events)))
}

func (s *Server) mcpHandler() http.Handler {
	server := mcp.NewServer(&mcp.Implementation{Name: "xlsx-viewer", Version: "0.1.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_workbook",
		Description: "Get the open workbook name, sheets, and current revision before reading or editing it.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, workbook.Snapshot, error) {
		return nil, s.session.Snapshot(), nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "read_range",
		Description: "Read cell display values and formulas from an A1-style range in the open workbook.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(_ context.Context, _ *mcp.CallToolRequest, input readRangeInput) (*mcp.CallToolResult, workbook.Range, error) {
		returnValue, err := s.session.ReadRange(input.Sheet, input.Range)
		return nil, returnValue, err
	})
	destructive := true
	closedWorld := false
	mcp.AddTool(server, &mcp.Tool{
		Name:        "apply_operations",
		Description: "Apply validated cell value or formula edits to the open workbook. Pass the latest baseRevision to prevent overwriting concurrent edits.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive, OpenWorldHint: &closedWorld},
	}, func(_ context.Context, _ *mcp.CallToolRequest, input applyInput) (*mcp.CallToolResult, applyResponse, error) {
		snapshot, err := s.session.Apply(input.BaseRevision, "ai", input.Operations)
		return nil, applyResponse{Workbook: snapshot, Applied: len(input.Operations)}, err
	})
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	if token := r.URL.Query().Get("token"); token != "" {
		if token != s.token {
			http.Error(w, "invalid session token", http.StatusUnauthorized)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "xlsx_session", Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: 12 * 60 * 60})
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if !s.validSessionCookie(r) {
		http.Error(w, "open the tokenized URL printed by xlsx-viewer", http.StatusUnauthorized)
		return
	}
	body, err := staticFS.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "UI unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(body)
}

func (s *Server) getWorkbook(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.session.Snapshot())
}

func (s *Server) getRange(w http.ResponseWriter, r *http.Request) {
	result, err := s.session.ReadRange(r.URL.Query().Get("sheet"), r.URL.Query().Get("range"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) applyOperations(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var input applyInput
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid request: %w", err))
		return
	}
	snapshot, err := s.session.Apply(input.BaseRevision, "human", input.Operations)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, workbook.ErrRevisionConflict) {
			status = http.StatusConflict
		}
		writeError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, applyResponse{Workbook: snapshot, Applied: len(input.Operations)})
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	events, cancel := s.session.Subscribe()
	defer cancel()
	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case event, open := <-events:
			if !open {
				return
			}
			data, _ := json.Marshal(event)
			_, _ = fmt.Fprintf(w, "event: workbook\ndata: %s\n\n", data)
			flusher.Flush()
		case <-heartbeat.C:
			_, _ = io.WriteString(w, ": keepalive\n\n")
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (s *Server) requireBearer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+s.token {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.validSessionCookie(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodGet {
			origin := r.Header.Get("Origin")
			if origin != "" && !sameOrigin(origin, r.Host) {
				http.Error(w, "origin rejected", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) validSessionCookie(r *http.Request) bool {
	cookie, err := r.Cookie("xlsx_session")
	return err == nil && cookie.Value == s.token
}

func sameOrigin(origin, host string) bool {
	return origin == "http://"+host || origin == "https://"+host
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self'; img-src 'self' data: blob:; font-src 'self' data:; worker-src 'self' blob:")
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
