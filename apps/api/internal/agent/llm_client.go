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

// OpenAILLMClient implements LLMClient for OpenAI or any compatible completion gateway.
type OpenAILLMClient struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

func NewOpenAILLMClient(apiKey, baseURL, defaultModel string, httpClient *http.Client) *OpenAILLMClient {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")
	if defaultModel == "" {
		defaultModel = "gpt-4o-mini"
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}

	return &OpenAILLMClient{
		apiKey:     apiKey,
		baseURL:    baseURL,
		model:      defaultModel,
		httpClient: httpClient,
	}
}

type openAIChatRequest struct {
	Model       string           `json:"model"`
	Messages    []ChatMessage    `json:"messages"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
	Temperature float64          `json:"temperature"`
}

type openAIChatResponse struct {
	Choices []struct {
		Message      ChatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

func (c *OpenAILLMClient) CreateChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	model := req.Model
	if model == "" {
		model = c.model
	}

	payload := openAIChatRequest{
		Model:       model,
		Messages:    req.Messages,
		Tools:       req.Tools,
		Temperature: req.Temperature,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to encode chat request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/chat/completions", c.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("llm completion request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("llm provider returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var openAIResp openAIChatResponse
	if err := json.Unmarshal(respBody, &openAIResp); err != nil {
		return nil, fmt.Errorf("failed to decode llm response: %w", err)
	}

	if openAIResp.Error != nil {
		return nil, fmt.Errorf("llm api error: %s", openAIResp.Error.Message)
	}

	if len(openAIResp.Choices) == 0 {
		return nil, fmt.Errorf("no completion choices returned by model")
	}

	choice := openAIResp.Choices[0]
	costUSD := (float64(openAIResp.Usage.PromptTokens)*0.00015 + float64(openAIResp.Usage.CompletionTokens)*0.00060) / 1000.0

	return &ChatResponse{
		Message:      choice.Message,
		TokensIn:     openAIResp.Usage.PromptTokens,
		TokensOut:    openAIResp.Usage.CompletionTokens,
		CostUSD:      costUSD,
		FinishReason: choice.FinishReason,
	}, nil
}
