package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/qtaghdi/document-workspace/internal/formats/xlsx"
)

type applyInput struct {
	BaseRevision uint64           `json:"baseRevision" jsonschema:"Revision returned by get_workbook or read_range"`
	Operations   []xlsx.Operation `json:"operations" jsonschema:"Ordered workbook edits to apply atomically"`
}

type applyResponse struct {
	Workbook xlsx.Snapshot `json:"workbook"`
	Applied  int           `json:"applied"`
}

type historyInput struct {
	BaseRevision uint64 `json:"baseRevision" jsonschema:"Current workbook revision"`
	Direction    string `json:"direction,omitempty" jsonschema:"History direction: undo or redo"`
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	if token := r.URL.Query().Get("token"); token != "" {
		if token != s.browserToken {
			http.Error(w, "invalid session token", http.StatusUnauthorized)
			return
		}
		s.setBrowserSessionCookie(w)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if !s.validSessionCookie(r) {
		http.Error(w, "open the tokenized URL printed by document-workspace", http.StatusUnauthorized)
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

func (s *Server) launchBrowserSession(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	s.launchMu.Lock()
	expiresAt, ok := s.launchTokens[token]
	if ok {
		delete(s.launchTokens, token)
	}
	s.launchMu.Unlock()
	if !ok || !expiresAt.After(time.Now()) {
		http.Error(w, "invalid or expired launch link", http.StatusUnauthorized)
		return
	}
	s.setBrowserSessionCookie(w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) setBrowserSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "xlsx_session",
		Value:    s.browserToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   12 * 60 * 60,
	})
}

func (s *Server) updatePresence(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var input xlsx.Presence
	decoder := json.NewDecoder(io.LimitReader(r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid request: %w", err))
		return
	}
	if err := s.session.UpdatePresence("human", input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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

func (s *Server) getSheetObjects(w http.ResponseWriter, r *http.Request) {
	result, err := s.session.ReadSheetObjects(r.URL.Query().Get("sheet"))
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
		if errors.Is(err, xlsx.ErrRevisionConflict) {
			status = http.StatusConflict
		}
		writeError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, applyResponse{Workbook: snapshot, Applied: len(input.Operations)})
}

func (s *Server) restoreHistory(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var input historyInput
	decoder := json.NewDecoder(io.LimitReader(r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid request: %w", err))
		return
	}
	direction := r.PathValue("direction")
	snapshot, err := s.session.RestoreHistory(input.BaseRevision, "human", direction)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, xlsx.ErrRevisionConflict) {
			status = http.StatusConflict
		}
		writeError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, applyResponse{Workbook: snapshot, Applied: 1})
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
	var after uint64
	if value := r.Header.Get("Last-Event-ID"); value != "" {
		parsed, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, errors.New("invalid Last-Event-ID"))
			return
		}
		after = parsed
	}
	events, cancel := s.session.Subscribe(after)
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
			_, _ = fmt.Fprintf(w, "id: %d\nevent: workbook\ndata: %s\n\n", event.Sequence, data)
			flusher.Flush()
		case <-heartbeat.C:
			_, _ = io.WriteString(w, ": keepalive\n\n")
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}
