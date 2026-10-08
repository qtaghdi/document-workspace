package httpapi

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/qtaghdi/document-workspace/internal/formats/xlsx"
)

const (
	appResourceURI     = "ui://document-workspace/xlsx/v1.html"
	serverInstructions = "Call get_workbook before reads or writes. Use read_range for focused inspection. Pass the latest revision to apply_operations, and refresh after a conflict. Use open_workbook when an interactive spreadsheet helps. If it returns browserUrl because the host did not render the MCP App, open that short-lived URL in the host browser surface and do not repeat it in chat. Use update_presence before visible AI edits."
)

type openWorkbookResponse struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Sheets     []string `json:"sheets"`
	Revision   uint64   `json:"revision"`
	BrowserURL string   `json:"browserUrl,omitempty"`
}

type readRangeInput struct {
	Sheet string `json:"sheet" jsonschema:"Worksheet name"`
	Range string `json:"range" jsonschema:"A1-style range, for example A1:H40"`
}

type sheetInput struct {
	Sheet string `json:"sheet" jsonschema:"Worksheet name"`
}

type eventPollInput struct {
	AfterSequence uint64 `json:"afterSequence,omitempty" jsonschema:"Last processed event sequence, or zero for all retained events"`
}

type eventPollResponse struct {
	Events   []xlsx.Event  `json:"events"`
	Workbook xlsx.Snapshot `json:"workbook"`
}

func (s *Server) mcpHandler() http.Handler {
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return s.mcpServer() }, nil)
}

// RunStdio serves the same MCP tools over stdin and stdout for local desktop
// hosts. Nothing except protocol messages may be written to stdout while it runs.
func (s *Server) RunStdio(ctx context.Context) error {
	return s.mcpServer().Run(ctx, &mcp.StdioTransport{})
}

// RunStdioWithBrowser keeps stdout protocol-only while serving the authenticated
// browser fallback on a loopback listener owned by the same process.
func (s *Server) RunStdioWithBrowser(ctx context.Context, addr string) error {
	if err := requireLoopbackAddress(addr); err != nil {
		return err
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen for browser fallback: %w", err)
	}
	runContext, cancel := context.WithCancel(ctx)
	defer cancel()
	s.launchMu.Lock()
	s.browserBaseURL = "http://" + listener.Addr().String()
	s.launchMu.Unlock()

	httpServer := &http.Server{
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	httpErrors := make(chan error, 1)
	go func() {
		err := httpServer.Serve(listener)
		if err == http.ErrServerClosed {
			err = nil
		}
		httpErrors <- err
	}()
	stdioErrors := make(chan error, 1)
	go func() {
		stdioErrors <- s.RunStdio(runContext)
	}()

	var runErr error
	select {
	case runErr = <-stdioErrors:
	case err := <-httpErrors:
		if err != nil {
			runErr = fmt.Errorf("serve browser fallback: %w", err)
		}
		cancel()
	case <-ctx.Done():
		runErr = ctx.Err()
	}
	cancel()
	shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownContext); err != nil && runErr == nil {
		runErr = fmt.Errorf("stop browser fallback: %w", err)
	}
	return runErr
}

func requireLoopbackAddress(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("parse browser fallback address: %w", err)
	}
	if host == "localhost" {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("browser fallback must bind to a loopback address, got %q", host)
	}
	return nil
}

func (s *Server) mcpServer() *mcp.Server {
	server := mcp.NewServer(
		&mcp.Implementation{Name: "document-workspace", Version: "0.1.0"},
		&mcp.ServerOptions{Instructions: serverInstructions},
	)
	appMeta := mcp.Meta{
		"ui":                    map[string]any{"resourceUri": appResourceURI, "visibility": []string{"model", "app"}},
		"openai/outputTemplate": appResourceURI,
	}
	appOnlyMeta := mcp.Meta{"ui": map[string]any{"visibility": []string{"app"}}}
	closedWorld := false
	destructive := true
	nonDestructive := false
	server.AddResource(&mcp.Resource{
		URI:         appResourceURI,
		Name:        "document-workspace-xlsx",
		Title:       "Workbook Editor",
		Description: "Interactive spreadsheet editor for the open XLSX workbook.",
		MIMEType:    "text/html;profile=mcp-app",
	}, func(context.Context, *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		body, err := staticFS.ReadFile("static/app.html")
		if err != nil {
			return nil, fmt.Errorf("read embedded MCP App: %w", err)
		}
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{
			URI:      appResourceURI,
			MIMEType: "text/html;profile=mcp-app",
			Text:     string(body),
			Meta:     mcp.Meta{"ui": map[string]any{"prefersBorder": false}},
		}}}, nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "open_workbook",
		Title:       "Open Workbook",
		Description: "Open the current workbook in an interactive spreadsheet surface. When the host does not render MCP Apps, use the short-lived browserUrl fallback returned by this tool.",
		Meta:        appMeta,
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &closedWorld},
	}, func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, openWorkbookResponse, error) {
		snapshot := s.session.Snapshot()
		browserURL, err := s.issueBrowserLaunchURL()
		return nil, openWorkbookResponse{
			ID:         snapshot.ID,
			Name:       snapshot.Name,
			Sheets:     snapshot.Sheets,
			Revision:   snapshot.Revision,
			BrowserURL: browserURL,
		}, err
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "restore_history",
		Title:       "Undo or Redo Workbook Change",
		Description: "Undo or redo one committed workbook revision. Pass the latest baseRevision and direction.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive, OpenWorldHint: &closedWorld},
	}, func(_ context.Context, _ *mcp.CallToolRequest, input historyInput) (*mcp.CallToolResult, applyResponse, error) {
		snapshot, err := s.session.RestoreHistory(input.BaseRevision, "ai", input.Direction)
		return nil, applyResponse{Workbook: snapshot, Applied: 1}, err
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "restore_user_history",
		Title:       "Undo or Redo User Workbook Change",
		Description: "Undo or redo one committed workbook revision for the MCP App user.",
		Meta:        appOnlyMeta,
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive, OpenWorldHint: &closedWorld},
	}, func(_ context.Context, _ *mcp.CallToolRequest, input historyInput) (*mcp.CallToolResult, applyResponse, error) {
		snapshot, err := s.session.RestoreHistory(input.BaseRevision, "human", input.Direction)
		return nil, applyResponse{Workbook: snapshot, Applied: 1}, err
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_workbook",
		Title:       "Get Workbook",
		Description: "Get the open workbook name, sheets, and current revision before reading or editing it.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &closedWorld},
	}, func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, xlsx.Snapshot, error) {
		return nil, s.session.Snapshot(), nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "read_range",
		Title:       "Read Range",
		Description: "Read cell display values and formulas from an A1-style range in the open xlsx.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &closedWorld},
	}, func(_ context.Context, _ *mcp.CallToolRequest, input readRangeInput) (*mcp.CallToolResult, xlsx.Range, error) {
		returnValue, err := s.session.ReadRange(input.Sheet, input.Range)
		return nil, returnValue, err
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_events",
		Title:       "Get Workbook Events",
		Description: "Read retained workbook and presence events after a sequence number for MCP App synchronization.",
		Meta:        appOnlyMeta,
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &closedWorld},
	}, func(_ context.Context, _ *mcp.CallToolRequest, input eventPollInput) (*mcp.CallToolResult, eventPollResponse, error) {
		events, snapshot := s.session.EventsAfter(input.AfterSequence)
		return nil, eventPollResponse{Events: events, Workbook: snapshot}, nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_sheet_objects",
		Title:       "Get Sheet Objects",
		Description: "Read bounded image data and chart previews for MCP App rendering.",
		Meta:        appOnlyMeta,
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &closedWorld},
	}, func(_ context.Context, _ *mcp.CallToolRequest, input sheetInput) (*mcp.CallToolResult, xlsx.SheetObjects, error) {
		objects, err := s.session.ReadSheetObjects(input.Sheet)
		return nil, objects, err
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "apply_operations",
		Title:       "Apply Workbook Operations",
		Description: "Apply validated cell, formula, or rectangular paste edits to the open xlsx. Pass the latest baseRevision to prevent overwriting concurrent edits.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive, OpenWorldHint: &closedWorld},
	}, func(_ context.Context, _ *mcp.CallToolRequest, input applyInput) (*mcp.CallToolResult, applyResponse, error) {
		snapshot, err := s.session.Apply(input.BaseRevision, "ai", input.Operations)
		return nil, applyResponse{Workbook: snapshot, Applied: len(input.Operations)}, err
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "apply_user_operations",
		Title:       "Apply User Workbook Operations",
		Description: "Apply browser-originated workbook operations from the MCP App.",
		Meta:        appOnlyMeta,
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive, OpenWorldHint: &closedWorld},
	}, func(_ context.Context, _ *mcp.CallToolRequest, input applyInput) (*mcp.CallToolResult, applyResponse, error) {
		snapshot, err := s.session.Apply(input.BaseRevision, "human", input.Operations)
		return nil, applyResponse{Workbook: snapshot, Applied: len(input.Operations)}, err
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_presence",
		Title:       "Update AI Presence",
		Description: "Move the AI cursor or selection without changing workbook data or its revision.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &nonDestructive, IdempotentHint: true, OpenWorldHint: &closedWorld},
	}, func(_ context.Context, _ *mcp.CallToolRequest, input xlsx.Presence) (*mcp.CallToolResult, map[string]bool, error) {
		err := s.session.UpdatePresence("ai", input)
		return nil, map[string]bool{"updated": err == nil}, err
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_user_presence",
		Title:       "Update User Presence",
		Description: "Publish the MCP App user's current selection without changing workbook data.",
		Meta:        appOnlyMeta,
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &nonDestructive, IdempotentHint: true, OpenWorldHint: &closedWorld},
	}, func(_ context.Context, _ *mcp.CallToolRequest, input xlsx.Presence) (*mcp.CallToolResult, map[string]bool, error) {
		err := s.session.UpdatePresence("human", input)
		return nil, map[string]bool{"updated": err == nil}, err
	})
	return server
}
