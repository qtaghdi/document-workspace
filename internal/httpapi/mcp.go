package httpapi

import (
	"context"
	"fmt"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/qtaghdi/xlsx-viewer/internal/workbook"
)

const (
	appResourceURI     = "ui://xlsx-viewer/workbook/v1.html"
	serverInstructions = "Call get_workbook before reads or writes. Use read_range for focused inspection. Pass the latest revision to apply_operations, and refresh after a conflict. Use open_workbook when an interactive spreadsheet helps, and update_presence before visible AI edits."
)

type readRangeInput struct {
	Sheet string `json:"sheet" jsonschema:"Worksheet name"`
	Range string `json:"range" jsonschema:"A1-style range, for example A1:H40"`
}

type eventPollInput struct {
	AfterSequence uint64 `json:"afterSequence,omitempty" jsonschema:"Last processed event sequence, or zero for all retained events"`
}

type eventPollResponse struct {
	Events   []workbook.Event  `json:"events"`
	Workbook workbook.Snapshot `json:"workbook"`
}

func (s *Server) mcpHandler() http.Handler {
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return s.mcpServer() }, nil)
}

// RunStdio serves the same MCP tools over stdin and stdout for local desktop
// hosts. Nothing except protocol messages may be written to stdout while it runs.
func (s *Server) RunStdio(ctx context.Context) error {
	return s.mcpServer().Run(ctx, &mcp.StdioTransport{})
}

func (s *Server) mcpServer() *mcp.Server {
	server := mcp.NewServer(
		&mcp.Implementation{Name: "xlsx-viewer", Version: "0.1.0"},
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
		Name:        "xlsx-viewer-workbook",
		Title:       "Workbook Editor",
		Description: "Interactive spreadsheet editor for the open workbook.",
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
		Description: "Open the current workbook in an interactive spreadsheet surface when the host supports MCP Apps.",
		Meta:        appMeta,
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &closedWorld},
	}, func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, workbook.Snapshot, error) {
		return nil, s.session.Snapshot(), nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_workbook",
		Title:       "Get Workbook",
		Description: "Get the open workbook name, sheets, and current revision before reading or editing it.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &closedWorld},
	}, func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, workbook.Snapshot, error) {
		return nil, s.session.Snapshot(), nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "read_range",
		Title:       "Read Range",
		Description: "Read cell display values and formulas from an A1-style range in the open workbook.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &closedWorld},
	}, func(_ context.Context, _ *mcp.CallToolRequest, input readRangeInput) (*mcp.CallToolResult, workbook.Range, error) {
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
		Name:        "apply_operations",
		Title:       "Apply Workbook Operations",
		Description: "Apply validated cell, formula, or rectangular paste edits to the open workbook. Pass the latest baseRevision to prevent overwriting concurrent edits.",
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
	}, func(_ context.Context, _ *mcp.CallToolRequest, input workbook.Presence) (*mcp.CallToolResult, map[string]bool, error) {
		err := s.session.UpdatePresence("ai", input)
		return nil, map[string]bool{"updated": err == nil}, err
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_user_presence",
		Title:       "Update User Presence",
		Description: "Publish the MCP App user's current selection without changing workbook data.",
		Meta:        appOnlyMeta,
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &nonDestructive, IdempotentHint: true, OpenWorldHint: &closedWorld},
	}, func(_ context.Context, _ *mcp.CallToolRequest, input workbook.Presence) (*mcp.CallToolResult, map[string]bool, error) {
		err := s.session.UpdatePresence("human", input)
		return nil, map[string]bool{"updated": err == nil}, err
	})
	return server
}
