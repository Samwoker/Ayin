package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/seraph/triage/apps/api/internal/store"
	"github.com/seraph/triage/apps/api/internal/telemetry"
	"github.com/seraph/triage/apps/api/internal/tools"
)

// Server implements the Model Context Protocol (MCP) JSON-RPC 2.0 handler.
type Server struct {
	toolReg *tools.ToolRegistry
	store   *store.Store
	orgID   pgtype.UUID
	logger  *slog.Logger
}

func NewServer(toolReg *tools.ToolRegistry, st *store.Store, orgID pgtype.UUID, logger *slog.Logger) *Server {
	if logger == nil {
		logger = telemetry.SetupLogger("info", "mcp")
	}
	if !orgID.Valid {
		_ = orgID.Scan("00000000-0000-0000-0000-000000000001")
	}
	if toolReg == nil {
		toolReg = tools.NewToolRegistry(tools.NewAllowAllPolicyProvider(), 15*time.Second)
	}

	return &Server{
		toolReg: toolReg,
		store:   st,
		orgID:   orgID,
		logger:  logger,
	}
}

// HandleRequest processes an incoming JSON-RPC 2.0 request and returns an appropriate response.
// If the request was a notification (no ID), it returns nil.
func (s *Server) HandleRequest(ctx context.Context, req JSONRPCRequest) *JSONRPCResponse {
	// If ID is empty, this is a notification which requires no response.
	isNotification := len(req.ID) == 0 || string(req.ID) == "null"

	switch req.Method {
	case "initialize":
		res := InitializeResult{
			ProtocolVersion: ProtocolVersion,
			ServerInfo: ServerInfo{
				Name:    "triage-mcp-server",
				Version: "1.0.0",
			},
			Capabilities: ServerCapabilities{
				Tools:     map[string]any{},
				Resources: map[string]any{},
				Prompts:   map[string]any{},
			},
		}
		return s.success(req.ID, res)

	case "notifications/initialized":
		s.logger.Info("client initialized notification received")
		return nil

	case "ping":
		return s.success(req.ID, map[string]any{})

	case "tools/list":
		return s.handleListTools(ctx, req.ID)

	case "tools/call":
		return s.handleCallTool(ctx, req.ID, req.Params)

	case "resources/list":
		return s.handleListResources(ctx, req.ID)

	case "resources/read":
		return s.handleReadResource(ctx, req.ID, req.Params)

	case "prompts/list":
		return s.handleListPrompts(ctx, req.ID)

	case "prompts/get":
		return s.handleGetPrompt(ctx, req.ID, req.Params)

	default:
		if isNotification {
			return nil
		}
		return s.error(req.ID, CodeMethodNotFound, fmt.Sprintf("method %q not found", req.Method), nil)
	}
}

func (s *Server) handleListTools(ctx context.Context, id json.RawMessage) *JSONRPCResponse {
	availTools := s.toolReg.ListAvailable(ctx, s.orgID)
	mcpTools := make([]MCPTool, 0, len(availTools))

	for _, t := range availTools {
		schema := t.Schema()
		mcpTools = append(mcpTools, MCPTool{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: map[string]any{
				"type":       schema.Type,
				"properties": schema.Properties,
				"required":   schema.Required,
			},
		})
	}

	return s.success(id, ListToolsResult{Tools: mcpTools})
}

func (s *Server) handleCallTool(ctx context.Context, id json.RawMessage, params json.RawMessage) *JSONRPCResponse {
	var callParams struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}

	if err := json.Unmarshal(params, &callParams); err != nil {
		return s.error(id, CodeInvalidParams, "invalid tool call parameters", err.Error())
	}

	if callParams.Name == "" {
		return s.error(id, CodeInvalidParams, "missing required 'name' field", nil)
	}

	toolResult := s.toolReg.Execute(ctx, s.orgID, callParams.Name, callParams.Arguments)

	outBytes, err := json.MarshalIndent(toolResult.Data, "", "  ")
	if err != nil {
		outBytes = []byte(fmt.Sprintf("%v", toolResult.Data))
	}

	if !toolResult.Success {
		return s.success(id, CallToolResult{
			Content: []ToolContent{
				{
					Type: "text",
					Text: fmt.Sprintf("Error executing tool %s: %s", callParams.Name, toolResult.Error),
				},
			},
			IsError: true,
		})
	}

	return s.success(id, CallToolResult{
		Content: []ToolContent{
			{
				Type: "text",
				Text: string(outBytes),
			},
		},
		IsError: false,
	})
}

func (s *Server) handleListResources(ctx context.Context, id json.RawMessage) *JSONRPCResponse {
	res := ListResourcesResult{
		Resources: []Resource{
			{
				URI:         "triage://system/overview",
				Name:        "System Overview",
				Description: "Triage platform operational topology and health metrics",
				MIMEType:    "application/json",
			},
			{
				URI:         "triage://tools/catalog",
				Name:        "Diagnostic Tools Catalog",
				Description: "Catalog of read-only diagnostic tools available for incident triage",
				MIMEType:    "application/json",
			},
		},
	}
	return s.success(id, res)
}

func (s *Server) handleReadResource(ctx context.Context, id json.RawMessage, params json.RawMessage) *JSONRPCResponse {
	var readParams struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(params, &readParams); err != nil {
		return s.error(id, CodeInvalidParams, "invalid read resource parameters", err.Error())
	}

	switch readParams.URI {
	case "triage://system/overview":
		overview := map[string]any{
			"platform":      "triage",
			"status":        "healthy",
			"version":       "v1.0.0",
			"active_tenant": s.orgID.String(),
			"timestamp":     time.Now().UTC().Format(time.RFC3339),
		}
		b, _ := json.MarshalIndent(overview, "", "  ")
		return s.success(id, ReadResourceResult{
			Contents: []ResourceContent{
				{
					URI:      readParams.URI,
					MIMEType: "application/json",
					Text:     string(b),
				},
			},
		})

	case "triage://tools/catalog":
		availTools := s.toolReg.ListAvailable(ctx, s.orgID)
		b, _ := json.MarshalIndent(availTools, "", "  ")
		return s.success(id, ReadResourceResult{
			Contents: []ResourceContent{
				{
					URI:      readParams.URI,
					MIMEType: "application/json",
					Text:     string(b),
				},
			},
		})

	default:
		return s.error(id, CodeInvalidParams, fmt.Sprintf("resource %q not found", readParams.URI), nil)
	}
}

func (s *Server) handleListPrompts(ctx context.Context, id json.RawMessage) *JSONRPCResponse {
	res := ListPromptsResult{
		Prompts: []Prompt{
			{
				Name:        "investigate_incident",
				Description: "Initiate autonomous diagnostic investigation and hypothesis formulation",
				Arguments: []PromptArgument{
					{Name: "service", Description: "Target affected service name", Required: true},
					{Name: "alert_title", Description: "Summary of the triggered alert", Required: true},
				},
			},
			{
				Name:        "review_remediation",
				Description: "Analyze proposed remediation against safety policies and rollback readiness",
				Arguments: []PromptArgument{
					{Name: "remediation_id", Description: "UUID of the proposed remediation", Required: true},
				},
			},
		},
	}
	return s.success(id, res)
}

func (s *Server) handleGetPrompt(ctx context.Context, id json.RawMessage, params json.RawMessage) *JSONRPCResponse {
	var getParams struct {
		Name      string            `json:"name"`
		Arguments map[string]string `json:"arguments"`
	}
	if err := json.Unmarshal(params, &getParams); err != nil {
		return s.error(id, CodeInvalidParams, "invalid get prompt parameters", err.Error())
	}

	switch getParams.Name {
	case "investigate_incident":
		svc := getParams.Arguments["service"]
		alert := getParams.Arguments["alert_title"]
		promptText := fmt.Sprintf(`You are an autonomous incident triage specialist investigating service %q.
Incoming Alert: %q.

Use available read-only diagnostic tools (log_search, recent_deploys, query_metrics, service_topology)
to gather concrete facts and formulate ranked root-cause hypotheses with verified evidence citations.`, svc, alert)

		return s.success(id, GetPromptResult{
			Description: "Incident triage prompt",
			Messages: []PromptMessage{
				{
					Role: "user",
					Content: ToolContent{
						Type: "text",
						Text: promptText,
					},
				},
			},
		})

	default:
		return s.error(id, CodeInvalidParams, fmt.Sprintf("prompt %q not found", getParams.Name), nil)
	}
}

func (s *Server) success(id json.RawMessage, result any) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Result:  result,
	}
}

func (s *Server) error(id json.RawMessage, code int, message string, data any) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
}
