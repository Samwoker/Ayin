package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// GeminiLLMClient implements LLMClient for Google Gemini (v1beta REST API).
type GeminiLLMClient struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

// NewGeminiLLMClient constructs a Gemini client.
func NewGeminiLLMClient(apiKey, baseURL, defaultModel string, httpClient *http.Client) *GeminiLLMClient {
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com/v1beta"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")
	if defaultModel == "" {
		defaultModel = "gemini-2.5-flash"
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}

	return &GeminiLLMClient{
		apiKey:     apiKey,
		baseURL:    baseURL,
		model:      defaultModel,
		httpClient: httpClient,
	}
}

// Gemini API data structures
type geminiPart struct {
	Text             string                  `json:"text,omitempty"`
	FunctionCall     *geminiFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *geminiFunctionResponse `json:"functionResponse,omitempty"`
}

type geminiFunctionCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args,omitempty"`
}

type geminiFunctionResponse struct {
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiFunctionDeclaration struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type geminiTool struct {
	FunctionDeclarations []geminiFunctionDeclaration `json:"functionDeclarations,omitempty"`
}

type geminiGenerationConfig struct {
	Temperature     float64 `json:"temperature,omitempty"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
}

type geminiGenerateContentRequest struct {
	SystemInstruction *geminiContent          `json:"systemInstruction,omitempty"`
	Contents          []geminiContent         `json:"contents"`
	Tools             []geminiTool            `json:"tools,omitempty"`
	GenerationConfig  *geminiGenerationConfig `json:"generationConfig,omitempty"`
}

type geminiCandidate struct {
	Content struct {
		Role  string       `json:"role"`
		Parts []geminiPart `json:"parts"`
	} `json:"content"`
	FinishReason string `json:"finishReason"`
}

type geminiUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

type geminiGenerateContentResponse struct {
	Candidates    []geminiCandidate    `json:"candidates"`
	UsageMetadata *geminiUsageMetadata `json:"usageMetadata,omitempty"`
	Error         *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error,omitempty"`
}

func (c *GeminiLLMClient) CreateChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	model := req.Model
	if model == "" {
		model = c.model
	}

	payload := geminiGenerateContentRequest{
		Contents: make([]geminiContent, 0),
		GenerationConfig: &geminiGenerationConfig{
			Temperature:     req.Temperature,
			MaxOutputTokens: req.MaxTokens,
		},
	}

	// 1. Process messages
	for _, msg := range req.Messages {
		switch msg.Role {
		case RoleSystem:
			payload.SystemInstruction = &geminiContent{
				Parts: []geminiPart{{Text: msg.Content}},
			}
		case RoleUser:
			payload.Contents = append(payload.Contents, geminiContent{
				Role:  "user",
				Parts: []geminiPart{{Text: msg.Content}},
			})
		case RoleAssistant:
			parts := make([]geminiPart, 0)
			if msg.Content != "" {
				parts = append(parts, geminiPart{Text: msg.Content})
			}
			for _, tc := range msg.ToolCalls {
				var args map[string]any
				if tc.Function.Arguments != "" {
					_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
				}
				if args == nil {
					args = make(map[string]any)
				}
				parts = append(parts, geminiPart{
					FunctionCall: &geminiFunctionCall{
						Name: tc.Function.Name,
						Args: args,
					},
				})
			}
			if len(parts) > 0 {
				payload.Contents = append(payload.Contents, geminiContent{
					Role:  "model",
					Parts: parts,
				})
			}
		case RoleTool:
			// Tool output returned to model
			var parsedOutput map[string]any
			if err := json.Unmarshal([]byte(msg.Content), &parsedOutput); err != nil {
				parsedOutput = map[string]any{"output": msg.Content}
			}
			fnName := msg.Name
			if fnName == "" {
				fnName = "tool_output"
			}
			payload.Contents = append(payload.Contents, geminiContent{
				Role: "user",
				Parts: []geminiPart{
					{
						FunctionResponse: &geminiFunctionResponse{
							Name:     fnName,
							Response: parsedOutput,
						},
					},
				},
			})
		}
	}

	// 2. Process Tools definitions
	if len(req.Tools) > 0 {
		declarations := make([]geminiFunctionDeclaration, 0, len(req.Tools))
		for _, t := range req.Tools {
			declarations = append(declarations, geminiFunctionDeclaration{
				Name:        t.Function.Name,
				Description: t.Function.Description,
				Parameters:  t.Function.Parameters,
			})
		}
		payload.Tools = []geminiTool{
			{FunctionDeclarations: declarations},
		}
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to encode gemini request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/models/%s:generateContent", c.baseURL, model)
	if c.apiKey != "" {
		endpoint = fmt.Sprintf("%s?key=%s", endpoint, c.apiKey)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create gemini http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("gemini api request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read gemini response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini api returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var geminiResp geminiGenerateContentResponse
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return nil, fmt.Errorf("failed to decode gemini response: %w", err)
	}

	if geminiResp.Error != nil {
		return nil, fmt.Errorf("gemini api error [%s]: %s", geminiResp.Error.Status, geminiResp.Error.Message)
	}

	if len(geminiResp.Candidates) == 0 {
		return nil, fmt.Errorf("gemini returned no candidates")
	}

	candidate := geminiResp.Candidates[0]
	var textBuilder strings.Builder
	var toolCalls []ToolCall

	for i, part := range candidate.Content.Parts {
		if part.Text != "" {
			textBuilder.WriteString(part.Text)
		}
		if part.FunctionCall != nil {
			argsBytes, _ := json.Marshal(part.FunctionCall.Args)
			toolCalls = append(toolCalls, ToolCall{
				ID:   fmt.Sprintf("call_%s_%d_%d", part.FunctionCall.Name, time.Now().UnixNano(), i),
				Type: "function",
				Function: FunctionCall{
					Name:      part.FunctionCall.Name,
					Arguments: string(argsBytes),
				},
			})
		}
	}

	var tokensIn, tokensOut int
	if geminiResp.UsageMetadata != nil {
		tokensIn = geminiResp.UsageMetadata.PromptTokenCount
		tokensOut = geminiResp.UsageMetadata.CandidatesTokenCount
	}

	// Cost model for Gemini (Flash: $0.075 / 1M prompt, $0.30 / 1M candidates; Pro: $1.25 / 1M prompt, $5.00 / 1M candidates)
	costPer1MIn := 0.075
	costPer1MOut := 0.30
	if strings.Contains(strings.ToLower(model), "pro") {
		costPer1MIn = 1.25
		costPer1MOut = 5.00
	}
	costUSD := (float64(tokensIn)*costPer1MIn + float64(tokensOut)*costPer1MOut) / 1000000.0

	return &ChatResponse{
		Message: ChatMessage{
			Role:      RoleAssistant,
			Content:   textBuilder.String(),
			ToolCalls: toolCalls,
		},
		TokensIn:     tokensIn,
		TokensOut:    tokensOut,
		CostUSD:      costUSD,
		FinishReason: candidate.FinishReason,
	}, nil
}
