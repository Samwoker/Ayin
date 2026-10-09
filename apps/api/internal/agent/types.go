package agent

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// Role denotes the speaker in a chat completion conversation.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// FunctionCall describes an invocation of a tool function requested by the LLM.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolCall represents a function call request emitted by the model.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // e.g. "function"
	Function FunctionCall `json:"function"`
}

// ChatMessage represents a single turn in an agent conversation.
type ChatMessage struct {
	Role       Role       `json:"role"`
	Content    string     `json:"content,omitempty"`
	Name       string     `json:"name,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

// FunctionDefinition describes an executable tool function schema for the LLM.
type FunctionDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ToolDefinition wraps a function definition adhering to OpenAI/JSON tool standards.
type ToolDefinition struct {
	Type     string             `json:"type"` // "function"
	Function FunctionDefinition `json:"function"`
}

// ChatRequest specifies input parameters for LLM completion.
type ChatRequest struct {
	Model       string           `json:"model"`
	Messages    []ChatMessage    `json:"messages"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
	Temperature float64          `json:"temperature"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
}

// ChatResponse contains the model's generated message and execution telemetry.
type ChatResponse struct {
	Message      ChatMessage `json:"message"`
	TokensIn     int         `json:"tokens_in"`
	TokensOut    int         `json:"tokens_out"`
	CostUSD      float64     `json:"cost_usd"`
	FinishReason string      `json:"finish_reason"`
}

// LLMClient abstract interface for LLM completions.
type LLMClient interface {
	CreateChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error)
}

// AgentConfig controls budget caps and operational parameters for an investigation run.
type AgentConfig struct {
	Model           string        `json:"model"`
	MaxSteps        int           `json:"max_steps"`
	MaxTokens       int           `json:"max_tokens"`
	MaxCostUSD      float64       `json:"max_cost_usd"`
	StepTimeout     time.Duration `json:"step_timeout"`
	RunTimeout      time.Duration `json:"run_timeout"`
	CostPer1kInUSD  float64       `json:"cost_per_1k_in_usd"`
	CostPer1kOutUSD float64       `json:"cost_per_1k_out_usd"`
}

// DefaultAgentConfig returns standard production guardrails.
func DefaultAgentConfig() AgentConfig {
	return AgentConfig{
		Model:           "gpt-4o-mini",
		MaxSteps:        8,
		MaxTokens:       25000,
		MaxCostUSD:      1.50,
		StepTimeout:     20 * time.Second,
		RunTimeout:      90 * time.Second,
		CostPer1kInUSD:  0.00015,
		CostPer1kOutUSD: 0.00060,
	}
}

// HypothesisDTO represents a structured root-cause finding.
type HypothesisDTO struct {
	Rank       int      `json:"rank"`
	RootCause  string   `json:"root_cause"`
	Confidence float64  `json:"confidence"` // 0.0 - 1.0
	Evidence   []string `json:"evidence"`
	Checklist  []string `json:"checklist"`
}

// InvestigationResult encapsulates the terminal state of an investigation run.
type InvestigationResult struct {
	RunID        pgtype.UUID     `json:"run_id"`
	IncidentID   pgtype.UUID     `json:"incident_id"`
	OrgID        pgtype.UUID     `json:"org_id"`
	Status       string          `json:"status"` // succeeded, failed, budget_exhausted
	TotalTokens  int             `json:"total_tokens"`
	TotalCostUSD float64         `json:"total_cost_usd"`
	StepsCount   int             `json:"steps_count"`
	Hypotheses   []HypothesisDTO `json:"hypotheses"`
	ErrorMessage string          `json:"error_message,omitempty"`
	Duration     time.Duration   `json:"duration"`
}
