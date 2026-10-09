package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/seraph/triage/apps/api/internal/telemetry"
	"github.com/seraph/triage/apps/api/internal/tools"
)

func setupTestMCPServer(t *testing.T) *Server {
	reg := tools.NewToolRegistry(tools.NewAllowAllPolicyProvider(), 5*time.Second)
	_ = reg.Register(tools.NewRecentDeploysTool(nil))
	_ = reg.Register(tools.NewLogSearchTool(nil))

	var orgID pgtype.UUID
	_ = orgID.Scan("00000000-0000-0000-0000-000000000001")

	logger := telemetry.SetupLogger("warn", "mcp-test")
	return NewServer(reg, nil, orgID, logger)
}

func TestMCPInitializeAndPing(t *testing.T) {
	s := setupTestMCPServer(t)
	ctx := context.Background()

	// Initialize
	initReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "initialize",
	}
	resp := s.HandleRequest(ctx, initReq)
	if resp == nil || resp.Error != nil {
		t.Fatalf("expected successful initialize response, got: %v", resp)
	}

	initRes, ok := resp.Result.(InitializeResult)
	if !ok || initRes.ServerInfo.Name != "triage-mcp-server" {
		t.Fatalf("unexpected initialize result: %v", resp.Result)
	}

	// Notifications initialized
	notifReq := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}
	notifResp := s.HandleRequest(ctx, notifReq)
	if notifResp != nil {
		t.Errorf("notifications must return nil response")
	}

	// Ping
	pingReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`2`),
		Method:  "ping",
	}
	pingResp := s.HandleRequest(ctx, pingReq)
	if pingResp == nil || pingResp.Error != nil {
		t.Fatalf("expected successful ping response")
	}
}

func TestMCPToolsListAndCall(t *testing.T) {
	s := setupTestMCPServer(t)
	ctx := context.Background()

	// Tools list
	listReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`10`),
		Method:  "tools/list",
	}
	listResp := s.HandleRequest(ctx, listReq)
	if listResp == nil || listResp.Error != nil {
		t.Fatalf("tools/list failed: %v", listResp)
	}

	toolsList, ok := listResp.Result.(ListToolsResult)
	if !ok || len(toolsList.Tools) < 2 {
		t.Fatalf("expected at least 2 tools, got: %v", toolsList)
	}

	// Tools call - recent_deploys
	callReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`11`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"recent_deploys","arguments":{"service":"checkout-worker","limit":2}}`),
	}
	callResp := s.HandleRequest(ctx, callReq)
	if callResp == nil || callResp.Error != nil {
		t.Fatalf("tools/call failed: %v", callResp)
	}

	callRes, ok := callResp.Result.(CallToolResult)
	if !ok || len(callRes.Content) == 0 {
		t.Fatalf("expected tool content returned")
	}

	// Tools call - unknown tool
	unknownReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`12`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"non_existent_tool","arguments":{}}`),
	}
	unknownResp := s.HandleRequest(ctx, unknownReq)
	if unknownResp == nil {
		t.Fatalf("expected response for unknown tool")
	}
	resObj := unknownResp.Result.(CallToolResult)
	if !resObj.IsError {
		t.Errorf("expected isError=true for unknown tool")
	}
}

func TestMCPResourcesAndPrompts(t *testing.T) {
	s := setupTestMCPServer(t)
	ctx := context.Background()

	// Resources list
	resListReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`20`),
		Method:  "resources/list",
	}
	resListResp := s.HandleRequest(ctx, resListReq)
	if resListResp == nil || resListResp.Error != nil {
		t.Fatalf("resources/list failed: %v", resListResp)
	}

	// Resources read
	resReadReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`21`),
		Method:  "resources/read",
		Params:  json.RawMessage(`{"uri":"triage://system/overview"}`),
	}
	resReadResp := s.HandleRequest(ctx, resReadReq)
	if resReadResp == nil || resReadResp.Error != nil {
		t.Fatalf("resources/read failed: %v", resReadResp)
	}

	// Prompts list
	promptListReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`30`),
		Method:  "prompts/list",
	}
	promptListResp := s.HandleRequest(ctx, promptListReq)
	if promptListResp == nil || promptListResp.Error != nil {
		t.Fatalf("prompts/list failed: %v", promptListResp)
	}

	// Prompts get
	promptGetReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`31`),
		Method:  "prompts/get",
		Params:  json.RawMessage(`{"name":"investigate_incident","arguments":{"service":"payment-api","alert_title":"DB Pool Out"}}`),
	}
	promptGetResp := s.HandleRequest(ctx, promptGetReq)
	if promptGetResp == nil || promptGetResp.Error != nil {
		t.Fatalf("prompts/get failed: %v", promptGetResp)
	}
}

func TestMCPUnknownMethod(t *testing.T) {
	s := setupTestMCPServer(t)
	ctx := context.Background()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`99`),
		Method:  "invalid_method",
	}
	resp := s.HandleRequest(ctx, req)
	if resp == nil || resp.Error == nil || resp.Error.Code != CodeMethodNotFound {
		t.Fatalf("expected -32601 MethodNotFound error, got: %v", resp)
	}
}

func TestServeStdio(t *testing.T) {
	s := setupTestMCPServer(t)
	ctx := context.Background()

	input := `{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n" +
		`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n"

	in := strings.NewReader(input)
	var out bytes.Buffer

	err := ServeStdio(ctx, s, in, &out)
	if err != nil {
		t.Fatalf("ServeStdio failed: %v", err)
	}

	outStr := out.String()
	if !strings.Contains(outStr, `"jsonrpc":"2.0"`) || !strings.Contains(outStr, `"id":1`) {
		t.Fatalf("unexpected stdio output: %s", outStr)
	}
}

func TestServeSSEEndpoints(t *testing.T) {
	s := setupTestMCPServer(t)

	// Test HTTP Message endpoint
	reqBody := `{"jsonrpc":"2.0","id":42,"method":"ping"}`
	httpReq := httptest.NewRequest(http.MethodPost, "/message", strings.NewReader(reqBody))
	w := httptest.NewRecorder()

	handler := http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		var rpcReq JSONRPCRequest
		_ = json.NewDecoder(r.Body).Decode(&rpcReq)
		resp := s.HandleRequest(r.Context(), rpcReq)
		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(resp)
	})

	handler.ServeHTTP(w, httpReq)
	if w.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d", w.Code)
	}

	var rpcResp JSONRPCResponse
	if err := json.Unmarshal(w.Body.Bytes(), &rpcResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if string(rpcResp.ID) != "42" {
		t.Errorf("expected response ID 42, got %s", string(rpcResp.ID))
	}
}

func TestMCPErrorBranches(t *testing.T) {
	s := setupTestMCPServer(t)
	ctx := context.Background()

	// 1. tools/call with invalid JSON params
	r1 := s.HandleRequest(ctx, JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
		Params:  json.RawMessage(`invalid-json`),
	})
	if r1 == nil || r1.Error == nil || r1.Error.Code != CodeInvalidParams {
		t.Errorf("expected CodeInvalidParams, got %v", r1)
	}

	// 2. tools/call with missing name
	r2 := s.HandleRequest(ctx, JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`2`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"arguments":{}}`),
	})
	if r2 == nil || r2.Error == nil || r2.Error.Code != CodeInvalidParams {
		t.Errorf("expected CodeInvalidParams for missing name, got %v", r2)
	}

	// 3. resources/read with invalid JSON
	r3 := s.HandleRequest(ctx, JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`3`),
		Method:  "resources/read",
		Params:  json.RawMessage(`invalid-json`),
	})
	if r3 == nil || r3.Error == nil || r3.Error.Code != CodeInvalidParams {
		t.Errorf("expected CodeInvalidParams, got %v", r3)
	}

	// 4. resources/read unknown URI
	r4 := s.HandleRequest(ctx, JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`4`),
		Method:  "resources/read",
		Params:  json.RawMessage(`{"uri":"triage://unknown"}`),
	})
	if r4 == nil || r4.Error == nil || r4.Error.Code != CodeInvalidParams {
		t.Errorf("expected CodeInvalidParams for unknown URI, got %v", r4)
	}

	// 5. resources/read tools catalog
	rCatalog := s.HandleRequest(ctx, JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`5`),
		Method:  "resources/read",
		Params:  json.RawMessage(`{"uri":"triage://tools/catalog"}`),
	})
	if rCatalog == nil || rCatalog.Error != nil {
		t.Errorf("expected successful tools catalog read, got %v", rCatalog)
	}

	// 6. prompts/get invalid JSON
	r6 := s.HandleRequest(ctx, JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`6`),
		Method:  "prompts/get",
		Params:  json.RawMessage(`invalid-json`),
	})
	if r6 == nil || r6.Error == nil || r6.Error.Code != CodeInvalidParams {
		t.Errorf("expected CodeInvalidParams, got %v", r6)
	}

	// 7. prompts/get unknown prompt
	r7 := s.HandleRequest(ctx, JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`7`),
		Method:  "prompts/get",
		Params:  json.RawMessage(`{"name":"unknown_prompt"}`),
	})
	if r7 == nil || r7.Error == nil || r7.Error.Code != CodeInvalidParams {
		t.Errorf("expected CodeInvalidParams for unknown prompt, got %v", r7)
	}

	// 8. Notification with unknown method
	r8 := s.HandleRequest(ctx, JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "notifications/unknown",
	})
	if r8 != nil {
		t.Errorf("expected nil for notification with unknown method, got %v", r8)
	}
}

func TestServeStdioParseError(t *testing.T) {
	s := setupTestMCPServer(t)
	ctx := context.Background()

	input := `{"malformed-json` + "\n"
	in := strings.NewReader(input)
	var out bytes.Buffer

	_ = ServeStdio(ctx, s, in, &out)
	if !strings.Contains(out.String(), `"code":-32700`) {
		t.Errorf("expected parse error code -32700 in output, got: %s", out.String())
	}
}

func TestServeSSELiveLifecycle(t *testing.T) {
	s := setupTestMCPServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	addr := "127.0.0.1:49182"
	errCh := make(chan error, 1)

	go func() {
		errCh <- ServeSSE(ctx, s, addr)
	}()

	// Wait for server to listen
	time.Sleep(50 * time.Millisecond)

	// Test GET /healthz
	resp, err := http.Get("http://" + addr + "/healthz")
	if err == nil {
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected HTTP 200 from /healthz, got %d", resp.StatusCode)
		}
	}

	// Test POST /message
	postResp, err := http.Post("http://"+addr+"/message", "application/json", strings.NewReader(`{"jsonrpc":"2.0","id":100,"method":"ping"}`))
	if err == nil {
		postResp.Body.Close()
		if postResp.StatusCode != http.StatusOK {
			t.Errorf("expected HTTP 200 from /message, got %d", postResp.StatusCode)
		}
	}

	// Cancel context to trigger graceful shutdown
	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("ServeSSE returned unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Errorf("ServeSSE did not shutdown in time")
	}
}
