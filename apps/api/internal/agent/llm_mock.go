package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// MockLLMClient provides deterministic, multi-turn reasoning simulations for incident triage testing and benchmarks.
type MockLLMClient struct {
	mu           sync.Mutex
	turnCount    int
	customTurns  []ChatResponse
	simulateErr  error
	tokensInBase int
	tokensOutBase int
}

func NewMockLLMClient() *MockLLMClient {
	return &MockLLMClient{
		tokensInBase:  350,
		tokensOutBase: 180,
	}
}

// SetError forces subsequent LLM calls to return the specified error.
func (m *MockLLMClient) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.simulateErr = err
}

// SetCustomTurns provides explicit scripted responses.
func (m *MockLLMClient) SetCustomTurns(turns []ChatResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.customTurns = turns
	m.turnCount = 0
}

func (m *MockLLMClient) CreateChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.simulateErr != nil {
		return nil, m.simulateErr
	}

	// 1. Check custom scripted turns
	if len(m.customTurns) > 0 {
		if m.turnCount < len(m.customTurns) {
			resp := m.customTurns[m.turnCount]
			m.turnCount++
			return &resp, nil
		}
		// Return last turn if exhausted
		last := m.customTurns[len(m.customTurns)-1]
		return &last, nil
	}

	// 2. Default realistic multi-turn incident triage scenario
	m.turnCount++
	switch m.turnCount {
	case 1:
		// Turn 1: Formulate plan and call recent_deploys & query_metrics
		return &ChatResponse{
			Message: ChatMessage{
				Role:    RoleAssistant,
				Content: "I am beginning the incident triage investigation.\n\nPlan:\n1. Inspect recent deployments to check if a release coincides with the alert onset.\n2. Query queue depth and error rate metric timeseries to confirm threshold severity.",
				ToolCalls: []ToolCall{
					{
						ID:   "call_deploys_1",
						Type: "function",
						Function: FunctionCall{
							Name:      "recent_deploys",
							Arguments: `{"service":"checkout-worker","limit":3}`,
						},
					},
					{
						ID:   "call_metrics_1",
						Type: "function",
						Function: FunctionCall{
							Name:      "query_metrics",
							Arguments: `{"query":"rabbitmq_queue_messages_ready","service":"checkout-worker"}`,
						},
					},
				},
			},
			TokensIn:     420,
			TokensOut:    140,
			CostUSD:      0.00014,
			FinishReason: "tool_calls",
		}, nil

	case 2:
		// Turn 2: Analyze deploy & metrics, then call log_search and service_topology
		return &ChatResponse{
			Message: ChatMessage{
				Role:    RoleAssistant,
				Content: "Recent deploy v2.14.0 occurred 22 minutes ago with changelog modifying prefetch count. Metric telemetry confirms queue depth spiked to ~58,000 ready messages.\n\nNext step:\nSearch checkout-worker error logs for connection exceptions and check architecture topology.",
				ToolCalls: []ToolCall{
					{
						ID:   "call_logs_1",
						Type: "function",
						Function: FunctionCall{
							Name:      "log_search",
							Arguments: `{"service":"checkout-worker","pattern":"deadlock|refused|timeout|backlog","limit":10}`,
						},
					},
					{
						ID:   "call_topology_1",
						Type: "function",
						Function: FunctionCall{
							Name:      "service_topology",
							Arguments: `{"service":"checkout-worker","depth":2}`,
						},
					},
				},
			},
			TokensIn:     780,
			TokensOut:    190,
			CostUSD:      0.00023,
			FinishReason: "tool_calls",
		}, nil

	case 3:
		// Turn 3: Observe logs and topology, then search runbooks
		return &ChatResponse{
			Message: ChatMessage{
				Role:    RoleAssistant,
				Content: "Logs show database deadlocks and buffer saturation. Downstream dependencies include rabbitmq and payment-api.\n\nSearching runbooks for RabbitMQ queue backlog mitigation procedures.",
				ToolCalls: []ToolCall{
					{
						ID:   "call_runbooks_1",
						Type: "function",
						Function: FunctionCall{
							Name:      "search_runbooks",
							Arguments: `{"query":"RabbitMQ consumer queue backlog","limit":2}`,
						},
					},
				},
			},
			TokensIn:     1150,
			TokensOut:    160,
			CostUSD:      0.00028,
			FinishReason: "tool_calls",
		}, nil

	default:
		// Turn 4: Final synthesis with JSON hypotheses
		conclusion := `Based on comprehensive evidence across deployments, metrics, logs, and system topology, the investigation is complete.

` + "```json\n" + `[
  {
    "rank": 1,
    "root_cause": "Deploy v2.14.0 increased RabbitMQ prefetch count from 10 to 1000, causing worker buffer saturation, unacknowledged message buildup, and database deadlocks.",
    "confidence": 0.92,
    "evidence": [
      "Deploy v2.14.0 deployed 22 minutes ago with changelog: 'switch prefetch count from 10 to 1000'",
      "rabbitmq_queue_messages_ready surged from 120 to 58,000 messages immediately following deployment",
      "checkout-worker logs confirm repeated database deadlock errors and connection timeouts"
    ],
    "checklist": [
      "Execute rollback of checkout-worker to previous release v2.13.9",
      "Scale checkout-worker consumer pods from 2 to 6 to drain message backlog",
      "Monitor rabbitmq_queue_messages_ready drop below 500 threshold"
    ]
  },
  {
    "rank": 2,
    "root_cause": "PostgreSQL database connection pool exhaustion under sudden traffic burst.",
    "confidence": 0.58,
    "evidence": [
      "Deadlock log entries observed on orders table during message processing spike",
      "checkout-worker downstream dependency connects directly to postgres-primary"
    ],
    "checklist": [
      "Check active connections count on postgres-primary database",
      "Verify connection pool max connections limits"
    ]
  }
]
` + "```"

		return &ChatResponse{
			Message: ChatMessage{
				Role:    RoleAssistant,
				Content: conclusion,
			},
			TokensIn:     1540,
			TokensOut:    380,
			CostUSD:      0.00045,
			FinishReason: "stop",
		}, nil
	}
}

// ExtractHypothesesFromText extracts the structured JSON hypothesis array from assistant message text.
func ExtractHypothesesFromText(text string) ([]HypothesisDTO, error) {
	startIdx := strings.Index(text, "```json")
	if startIdx != -1 {
		startIdx += len("```json")
		endIdx := strings.Index(text[startIdx:], "```")
		if endIdx != -1 {
			jsonBlock := strings.TrimSpace(text[startIdx : startIdx+endIdx])
			var hypotheses []HypothesisDTO
			if err := json.Unmarshal([]byte(jsonBlock), &hypotheses); err == nil && len(hypotheses) > 0 {
				return hypotheses, nil
			}
		}
	}

	// Try extracting directly as JSON array if no markdown code fences
	startBracket := strings.Index(text, "[")
	endBracket := strings.LastIndex(text, "]")
	if startBracket != -1 && endBracket != -1 && endBracket > startBracket {
		jsonBlock := strings.TrimSpace(text[startBracket : endBracket+1])
		var hypotheses []HypothesisDTO
		if err := json.Unmarshal([]byte(jsonBlock), &hypotheses); err == nil && len(hypotheses) > 0 {
			return hypotheses, nil
		}
	}

	return nil, fmt.Errorf("no valid JSON hypotheses array found in assistant text")
}
