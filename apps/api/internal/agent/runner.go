package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/seraph/triage/apps/api/internal/ingest"
	"github.com/seraph/triage/apps/api/internal/server/sse"
	"github.com/seraph/triage/apps/api/internal/store"
	"github.com/seraph/triage/apps/api/internal/store/db"
	"github.com/seraph/triage/apps/api/internal/telemetry"
	"github.com/seraph/triage/apps/api/internal/tools"
)

// AgentRunner coordinates the multi-turn autonomous investigation lifecycle.
type AgentRunner struct {
	store     *store.Store
	llm       LLMClient
	tools     *tools.ToolRegistry
	broker    *sse.Broker
	config    AgentConfig
	promptMgr *PromptManager
	logger    *slog.Logger
}

// NewAgentRunner creates a new AgentRunner.
func NewAgentRunner(
	st *store.Store,
	llm LLMClient,
	toolReg *tools.ToolRegistry,
	broker *sse.Broker,
	cfg AgentConfig,
	logger *slog.Logger,
) *AgentRunner {
	if llm == nil {
		llm = NewMockLLMClient()
	}
	if toolReg == nil {
		toolReg = tools.NewToolRegistry(tools.NewAllowAllPolicyProvider(), 15*time.Second)
	}
	if cfg.MaxSteps <= 0 {
		cfg.MaxSteps = 8
	}
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = 25000
	}
	if cfg.MaxCostUSD <= 0 {
		cfg.MaxCostUSD = 1.50
	}
	if cfg.RunTimeout <= 0 {
		cfg.RunTimeout = 90 * time.Second
	}
	if logger == nil {
		logger = telemetry.SetupLogger("info", "agent")
	}

	return &AgentRunner{
		store:     st,
		llm:       llm,
		tools:     toolReg,
		broker:    broker,
		config:    cfg,
		promptMgr: NewPromptManager(st),
		logger:    logger,
	}
}

// RunInvestigation conducts the full investigation loop for an incoming alert.
func (r *AgentRunner) RunInvestigation(ctx context.Context, job ingest.InvestigationJob) (*InvestigationResult, error) {
	start := time.Now()
	runCtx, cancel := context.WithTimeout(ctx, r.config.RunTimeout)
	defer cancel()

	r.logger.Info("starting autonomous investigation run",
		slog.String("service", job.Service),
		slog.String("title", job.Title),
		slog.String("source", job.Source),
	)

	// 1. Initialize or persist Run record
	var runID pgtype.UUID
	var promptVerID pgtype.UUID
	sysPrompt, promptVerID := r.promptMgr.GetSystemPrompt(runCtx)

	if r.store != nil {
		err := r.store.WithOrgTx(runCtx, job.OrgID, func(q *db.Queries) error {
			createdRun, err := q.CreateRun(runCtx, db.CreateRunParams{
				OrgID:           job.OrgID,
				IncidentID:      job.IncidentID,
				PromptVersionID: promptVerID,
				Status:          "running",
			})
			if err != nil {
				return err
			}
			runID = createdRun.ID
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create run in store: %w", err)
		}
	} else {
		_ = runID.Scan(fmt.Sprintf("50000000-0000-0000-0000-%012d", time.Now().UnixNano()%1000000000000))
	}

	runIDStr := uuidToStr(runID)

	// Broadcast initial SSE event
	if r.broker != nil {
		r.broker.Broadcast(runIDStr, "run_started", map[string]any{
			"run_id":      runIDStr,
			"incident_id": uuidToStr(job.IncidentID),
			"service":     job.Service,
			"status":      "running",
			"timestamp":   time.Now().UTC().Format(time.RFC3339),
		})
	}

	// 2. Discover permitted tools and convert to LLM schemas
	availTools := r.tools.ListAvailable(runCtx, job.OrgID)
	toolDefs := make([]ToolDefinition, 0, len(availTools))
	for _, t := range availTools {
		schema := t.Schema()
		toolDefs = append(toolDefs, ToolDefinition{
			Type: "function",
			Function: FunctionDefinition{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters: map[string]any{
					"type":       schema.Type,
					"properties": schema.Properties,
					"required":   schema.Required,
				},
			},
		})
	}

	// 3. Prepare conversation history
	messages := []ChatMessage{
		{Role: RoleSystem, Content: sysPrompt},
		{Role: RoleUser, Content: FormatIncidentContext(job)},
	}

	var totalTokens int
	var totalCost float64
	var stepNum int
	var hypotheses []HypothesisDTO
	var runStatus = "running"
	var runErr error

	// 4. Execution Step Loop
	for stepNum < r.config.MaxSteps {
		stepNum++

		// Budget checks
		if totalTokens >= r.config.MaxTokens || totalCost >= r.config.MaxCostUSD {
			r.logger.Warn("investigation budget cap exceeded",
				slog.Int("tokens", totalTokens),
				slog.Float64("cost_usd", totalCost),
			)
			runStatus = "budget_exhausted"
			break
		}

		if runCtx.Err() != nil {
			runErr = runCtx.Err()
			runStatus = "failed"
			break
		}

		// LLM Completion Turn
		chatReq := ChatRequest{
			Model:       r.config.Model,
			Messages:    messages,
			Tools:       toolDefs,
			Temperature: 0.2,
		}

		resp, err := r.llm.CreateChatCompletion(runCtx, chatReq)
		if err != nil {
			runErr = fmt.Errorf("llm completion error at step %d: %w", stepNum, err)
			runStatus = "failed"
			break
		}

		totalTokens += resp.TokensIn + resp.TokensOut
		totalCost += resp.CostUSD

		assistantMsg := resp.Message
		messages = append(messages, assistantMsg)

		// Case A: Model issued Tool Calls
		if len(assistantMsg.ToolCalls) > 0 {
			// Record reasoning plan or thought step
			if assistantMsg.Content != "" {
				r.recordStep(runCtx, job.OrgID, runID, stepNum, "plan", "", nil, nil, resp.TokensIn, resp.TokensOut, resp.CostUSD, 0, nil)
			}

			for _, tc := range assistantMsg.ToolCalls {
				toolName := tc.Function.Name
				toolArgs := tc.Function.Arguments

				// Execute tool safely through registry
				toolRes := r.tools.Execute(runCtx, job.OrgID, toolName, []byte(toolArgs))

				// Persist tool observation step
				var inputJSON, outputJSON []byte
				inputJSON = []byte(toolArgs)
				if toolRes.Data != nil {
					outputJSON, _ = json.Marshal(toolRes.Data)
				} else if toolRes.Error != "" {
					outputJSON, _ = json.Marshal(map[string]string{"error": toolRes.Error})
				}

				r.recordStep(runCtx, job.OrgID, runID, stepNum, "tool_call", toolName, inputJSON, outputJSON, 0, 0, 0, int(toolRes.LatencyMs), nil)

				// Broadcast SSE step update
				if r.broker != nil {
					r.broker.Broadcast(runIDStr, "step_completed", map[string]any{
						"run_id":      runIDStr,
						"step_number": stepNum,
						"step_type":   "tool_call",
						"tool_name":   toolName,
						"success":     toolRes.Success,
						"latency_ms":  toolRes.LatencyMs,
					})
				}

				// Feed observation back to conversation history
				obsContent, _ := json.Marshal(toolRes)
				messages = append(messages, ChatMessage{
					Role:       RoleTool,
					ToolCallID: tc.ID,
					Name:       toolName,
					Content:    string(obsContent),
				})
			}
			continue
		}

		// Case B: Model emitted text completion (Evaluation / Conclusion)
		if assistantMsg.Content != "" {
			extracted, err := ExtractHypothesesFromText(assistantMsg.Content)
			if err == nil && len(extracted) > 0 {
				hypotheses = extracted
				runStatus = "succeeded"

				r.recordStep(runCtx, job.OrgID, runID, stepNum, "conclusion", "", nil, []byte(assistantMsg.Content), resp.TokensIn, resp.TokensOut, resp.CostUSD, 0, nil)

				// Persist hypotheses to PostgreSQL under tenant transaction
				if r.store != nil {
					err := r.store.WithOrgTx(runCtx, job.OrgID, func(q *db.Queries) error {
						for _, h := range hypotheses {
							evidenceJSON, _ := json.Marshal(h.Evidence)
							checklistJSON, _ := json.Marshal(h.Checklist)

							_, err := q.CreateHypothesis(runCtx, db.CreateHypothesisParams{
								OrgID:      job.OrgID,
								RunID:      runID,
								Rank:       int32(h.Rank),
								RootCause:  h.RootCause,
								Confidence: h.Confidence,
								Evidence:   evidenceJSON,
								Checklist:  checklistJSON,
							})
							if err != nil {
								return err
							}
						}
						return nil
					})
					if err != nil {
						r.logger.Error("failed to persist hypotheses", slog.Any("error", err))
					}
				}

				break
			} else {
				// Intermediate commentary or plan
				stepType := "plan"
				if stepNum > 1 {
					stepType = "llm_response"
				}
				r.recordStep(runCtx, job.OrgID, runID, stepNum, stepType, "", nil, []byte(assistantMsg.Content), resp.TokensIn, resp.TokensOut, resp.CostUSD, 0, nil)
			}
		}
	}

	// Set default terminal status if loop ended without explicit conclusion
	if runStatus == "running" {
		if len(hypotheses) > 0 {
			runStatus = "succeeded"
		} else {
			runStatus = "budget_exhausted"
		}
	}

	// 5. Finalize Run Status in PostgreSQL
	duration := time.Since(start)
	if r.store != nil {
		err := r.store.WithOrgTx(runCtx, job.OrgID, func(q *db.Queries) error {
			var errMsg pgtype.Text
			if runErr != nil {
				errMsg = pgtype.Text{String: runErr.Error(), Valid: true}
			}

			_, err := q.UpdateRunStatus(runCtx, db.UpdateRunStatusParams{
				OrgID:        job.OrgID,
				ID:           runID,
				Status:       runStatus,
				CompletedAt:  pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
				TotalTokens:  int32(totalTokens),
				TotalCostUsd: floatToNumeric(totalCost),
				ErrorMessage: errMsg,
			})
			return err
		})
		if err != nil {
			r.logger.Error("failed to update run status", slog.Any("error", err))
		}
	}

	// Broadcast SSE run completion
	if r.broker != nil {
		r.broker.Broadcast(runIDStr, "run_completed", map[string]any{
			"run_id":           runIDStr,
			"status":           runStatus,
			"steps_count":      stepNum,
			"total_tokens":     totalTokens,
			"total_cost_usd":   totalCost,
			"hypotheses_count": len(hypotheses),
			"duration_ms":      duration.Milliseconds(),
		})
	}

	var errMsgStr string
	if runErr != nil {
		errMsgStr = runErr.Error()
	}

	return &InvestigationResult{
		RunID:        runID,
		IncidentID:   job.IncidentID,
		OrgID:        job.OrgID,
		Status:       runStatus,
		TotalTokens:  totalTokens,
		TotalCostUSD: totalCost,
		StepsCount:   stepNum,
		Hypotheses:   hypotheses,
		ErrorMessage: errMsgStr,
		Duration:     duration,
	}, runErr
}

func (r *AgentRunner) recordStep(
	ctx context.Context,
	orgID, runID pgtype.UUID,
	stepNum int,
	stepType string,
	toolName string,
	toolInput, toolOutput []byte,
	tokensIn, tokensOut int,
	costUSD float64,
	latencyMs int,
	stepErr error,
) {
	if r.store == nil {
		return
	}

	if len(toolInput) > 0 && !json.Valid(toolInput) {
		toolInput, _ = json.Marshal(map[string]string{"input": string(toolInput)})
	}
	if len(toolOutput) > 0 && !json.Valid(toolOutput) {
		toolOutput, _ = json.Marshal(map[string]string{"content": string(toolOutput)})
	}

	err := r.store.WithOrgTx(ctx, orgID, func(q *db.Queries) error {
		var toolNameText pgtype.Text
		if toolName != "" {
			toolNameText = pgtype.Text{String: toolName, Valid: true}
		}

		var errText pgtype.Text
		if stepErr != nil {
			errText = pgtype.Text{String: stepErr.Error(), Valid: true}
		}

		_, err := q.CreateRunStep(ctx, db.CreateRunStepParams{
			OrgID:      orgID,
			RunID:      runID,
			StepNumber: int32(stepNum),
			StepType:   stepType,
			ToolName:   toolNameText,
			ToolInput:  toolInput,
			ToolOutput: toolOutput,
			TokensIn:   int32(tokensIn),
			TokensOut:  int32(tokensOut),
			CostUsd:    floatToNumeric(costUSD),
			LatencyMs:  int32(latencyMs),
			Error:      errText,
		})
		return err
	})
	if err != nil {
		r.logger.Error("failed to record run step", slog.Any("error", err))
	}
}

func floatToNumeric(val float64) pgtype.Numeric {
	var num pgtype.Numeric
	_ = num.Scan(fmt.Sprintf("%.6f", val))
	return num
}

func uuidToStr(id pgtype.UUID) string {
	return fmt.Sprintf("%x-%x-%x-%x-%x", id.Bytes[0:4], id.Bytes[4:6], id.Bytes[6:8], id.Bytes[8:10], id.Bytes[10:16])
}
