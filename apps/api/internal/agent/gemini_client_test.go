package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGeminiLLMClient_TextCompletion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "generateContent") {
			http.Error(w, "invalid path", http.StatusNotFound)
			return
		}
		if r.URL.Query().Get("key") != "test-gemini-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var req geminiGenerateContentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if req.SystemInstruction == nil || len(req.SystemInstruction.Parts) == 0 {
			http.Error(w, "missing system instruction", http.StatusBadRequest)
			return
		}

		resp := geminiGenerateContentResponse{
			Candidates: []geminiCandidate{
				{
					Content: struct {
						Role  string       `json:"role"`
						Parts []geminiPart `json:"parts"`
					}{
						Role: "model",
						Parts: []geminiPart{
							{Text: "Incident diagnosed: memory leak in worker pool."},
						},
					},
					FinishReason: "STOP",
				},
			},
			UsageMetadata: &geminiUsageMetadata{
				PromptTokenCount:     150,
				CandidatesTokenCount: 25,
				TotalTokenCount:      175,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewGeminiLLMClient("test-gemini-key", server.URL, "gemini-2.5-flash", server.Client())

	req := ChatRequest{
		Model: "gemini-2.5-flash",
		Messages: []ChatMessage{
			{Role: RoleSystem, Content: "You are an expert SRE investigator."},
			{Role: RoleUser, Content: "Analyze pod crash in checkout-worker"},
		},
	}

	resp, err := client.CreateChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Message.Content != "Incident diagnosed: memory leak in worker pool." {
		t.Errorf("unexpected content: %s", resp.Message.Content)
	}
	if resp.TokensIn != 150 || resp.TokensOut != 25 {
		t.Errorf("unexpected tokens: in=%d, out=%d", resp.TokensIn, resp.TokensOut)
	}
	if resp.CostUSD <= 0 {
		t.Errorf("expected non-zero cost, got %f", resp.CostUSD)
	}
}

func TestGeminiLLMClient_FunctionCalling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req geminiGenerateContentRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		if len(req.Tools) == 0 || len(req.Tools[0].FunctionDeclarations) == 0 {
			http.Error(w, "expected tools declarations", http.StatusBadRequest)
			return
		}

		resp := geminiGenerateContentResponse{
			Candidates: []geminiCandidate{
				{
					Content: struct {
						Role  string       `json:"role"`
						Parts []geminiPart `json:"parts"`
					}{
						Role: "model",
						Parts: []geminiPart{
							{
								FunctionCall: &geminiFunctionCall{
									Name: "log_search",
									Args: map[string]any{
										"service": "checkout-worker",
										"query":   "OOMKilled",
									},
								},
							},
						},
					},
					FinishReason: "STOP",
				},
			},
			UsageMetadata: &geminiUsageMetadata{
				PromptTokenCount:     500,
				CandidatesTokenCount: 40,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewGeminiLLMClient("test-gemini-key", server.URL, "gemini-2.5-flash", server.Client())

	req := ChatRequest{
		Model: "gemini-2.5-flash",
		Messages: []ChatMessage{
			{Role: RoleUser, Content: "Inspect checkout worker crash logs"},
		},
		Tools: []ToolDefinition{
			{
				Type: "function",
				Function: FunctionDefinition{
					Name:        "log_search",
					Description: "Search recent logs",
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"service": map[string]any{"type": "string"},
							"query":   map[string]any{"type": "string"},
						},
					},
				},
			},
		},
	}

	resp, err := client.CreateChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Message.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.Message.ToolCalls))
	}
	tc := resp.Message.ToolCalls[0]
	if tc.Function.Name != "log_search" {
		t.Errorf("expected tool name log_search, got %s", tc.Function.Name)
	}
	if !strings.Contains(tc.Function.Arguments, "checkout-worker") {
		t.Errorf("expected service in arguments, got %s", tc.Function.Arguments)
	}
}

func TestGeminiLLMClient_MultiTurnWithToolOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req geminiGenerateContentRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		// Expect 3 contents: user prompt, assistant tool call, user function response
		if len(req.Contents) < 3 {
			http.Error(w, "expected at least 3 contents in conversation history", http.StatusBadRequest)
			return
		}

		resp := geminiGenerateContentResponse{
			Candidates: []geminiCandidate{
				{
					Content: struct {
						Role  string       `json:"role"`
						Parts []geminiPart `json:"parts"`
					}{
						Role: "model",
						Parts: []geminiPart{
							{Text: "Logs confirm Out Of Memory error due to cache explosion."},
						},
					},
					FinishReason: "STOP",
				},
			},
			UsageMetadata: &geminiUsageMetadata{
				PromptTokenCount:     1200,
				CandidatesTokenCount: 50,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewGeminiLLMClient("test-gemini-key", server.URL, "gemini-2.5-pro", server.Client())

	req := ChatRequest{
		Model: "gemini-2.5-pro",
		Messages: []ChatMessage{
			{Role: RoleUser, Content: "Diagnose checkout-worker"},
			{
				Role: RoleAssistant,
				ToolCalls: []ToolCall{
					{
						ID:   "call-1",
						Type: "function",
						Function: FunctionCall{
							Name:      "log_search",
							Arguments: `{"service":"checkout-worker"}`,
						},
					},
				},
			},
			{
				Role:    RoleTool,
				Name:    "log_search",
				Content: `{"lines":["fatal error: runtime: out of memory"]}`,
			},
		},
	}

	resp, err := client.CreateChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(resp.Message.Content, "Out Of Memory") {
		t.Errorf("unexpected content: %s", resp.Message.Content)
	}
	// Check Pro cost is higher than Flash cost
	expectedMinCost := (1200.0*1.25 + 50.0*5.00) / 1000000.0
	if resp.CostUSD < expectedMinCost-0.0001 {
		t.Errorf("expected pro cost >= %f, got %f", expectedMinCost, resp.CostUSD)
	}
}

func TestGeminiLLMClient_ErrorHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		resp := geminiGenerateContentResponse{
			Error: &struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
				Status  string `json:"status"`
			}{
				Code:    400,
				Message: "API key not valid",
				Status:  "INVALID_ARGUMENT",
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewGeminiLLMClient("invalid-key", server.URL, "gemini-2.5-flash", server.Client())

	_, err := client.CreateChatCompletion(context.Background(), ChatRequest{
		Messages: []ChatMessage{{Role: RoleUser, Content: "Hello"}},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "API key not valid") && !strings.Contains(err.Error(), "status 400") {
		t.Errorf("unexpected error message: %v", err)
	}
}
