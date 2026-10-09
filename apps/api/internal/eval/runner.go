package eval

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/seraph/triage/apps/api/internal/agent"
	"github.com/seraph/triage/apps/api/internal/ingest"
	"github.com/seraph/triage/apps/api/internal/telemetry"
)

// RunEvaluation executes the full benchmark evaluation suite according to EvalConfig.
func RunEvaluation(ctx context.Context, cfg EvalConfig) (*BenchmarkReport, error) {
	start := time.Now()
	scenarios, err := LoadScenarios(cfg.ScenariosDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load evaluation scenarios: %w", err)
	}

	logger := telemetry.SetupLogger("warn", "eval")

	runID := fmt.Sprintf("eval-%s-%d", cfg.Provider, time.Now().Unix())
	report := &BenchmarkReport{
		RunID:          runID,
		Timestamp:      time.Now().UTC(),
		Provider:       cfg.Provider,
		Model:          cfg.Model,
		TotalScenarios: len(scenarios),
		Results:        make([]ScenarioResult, 0, len(scenarios)),
	}

	if report.Model == "" {
		if cfg.Provider == "mock" {
			report.Model = "mock-deterministic-v1"
		} else {
			report.Model = "gpt-4o-mini"
		}
	}

	for _, sc := range scenarios {
		tracker := NewToolCallTracker()
		toolReg, err := BuildScenarioToolRegistry(sc, tracker)
		if err != nil {
			return nil, fmt.Errorf("failed to build tools for scenario %s: %w", sc.ID, err)
		}

		var llm agent.LLMClient
		switch cfg.Provider {
		case "mock", "":
			llm = NewScenarioAwareMockLLM(sc)
		case "openai":
			apiKey := cfg.APIKey
			if apiKey == "" {
				apiKey = os.Getenv("OPENAI_API_KEY")
			}
			if apiKey == "" {
				return nil, fmt.Errorf("OPENAI_API_KEY is required for openai provider")
			}
			llm = agent.NewOpenAILLMClient(apiKey, "", cfg.Model, nil)
		default:
			return nil, fmt.Errorf("unsupported eval LLM provider %q (supported: mock, openai)", cfg.Provider)
		}

		agentCfg := agent.DefaultAgentConfig()
		if sc.GroundTruth.MaxSteps > 0 {
			agentCfg.MaxSteps = sc.GroundTruth.MaxSteps + 2
		}
		if sc.GroundTruth.MaxTokens > 0 {
			agentCfg.MaxTokens = sc.GroundTruth.MaxTokens + 5000
		}

		runner := agent.NewAgentRunner(nil, llm, toolReg, nil, agentCfg, logger)

		var incidentID pgtype.UUID
		_ = incidentID.Scan(fmt.Sprintf("10000000-0000-0000-0000-%012d", time.Now().UnixNano()%1000000000000))

		orgID := cfg.OrgID
		if !orgID.Valid {
			_ = orgID.Scan("00000000-0000-0000-0000-000000000001")
		}

		job := ingest.InvestigationJob{
			IncidentID: incidentID,
			OrgID:      orgID,
			Service:    sc.Service,
			Title:      sc.Alert.Title,
			Severity:   sc.Severity,
			Source:     sc.Source,
			EnqueuedAt: time.Now(),
		}

		res, err := runner.RunInvestigation(ctx, job)
		if err != nil {
			logger.Error("scenario run failed", slog.String("scenario", sc.ID), slog.Any("error", err))
		}

		scResult := ScoreScenario(sc, res, tracker)
		report.Results = append(report.Results, scResult)

		if scResult.Passed {
			report.PassedScenarios++
		}
		report.TotalTokens += scResult.TotalTokens
		report.TotalCostUSD += scResult.CostUSD
	}

	report.TotalDuration = time.Since(start)

	// Aggregate metrics
	if report.TotalScenarios > 0 {
		report.PassRate = (float64(report.PassedScenarios) / float64(report.TotalScenarios)) * 100.0

		var sumOverall, sumAcc, sumEvid, sumHalluc, sumSteps float64
		for _, r := range report.Results {
			sumOverall += r.OverallScore
			sumAcc += r.AccuracyScore
			sumEvid += r.EvidenceScore
			sumHalluc += r.HallucinationScore
			sumSteps += float64(r.StepsCount)
		}

		count := float64(report.TotalScenarios)
		report.MeanOverallScore = sumOverall / count
		report.MeanAccuracyScore = sumAcc / count
		report.MeanEvidenceScore = sumEvid / count
		report.MeanHallucination = sumHalluc / count
		report.MeanSteps = sumSteps / count
	}

	// Baseline comparison if requested
	if cfg.CompareFile != "" {
		baseline, err := LoadReportJSON(cfg.CompareFile)
		if err == nil {
			report.Comparison = CompareWithBaseline(report, baseline)
		} else {
			logger.Warn("unable to load comparison baseline", slog.String("file", cfg.CompareFile), slog.Any("error", err))
		}
	}

	// Save baseline if requested
	if cfg.SaveBaseline != "" {
		if err := SaveReportJSON(report, cfg.SaveBaseline); err != nil {
			logger.Warn("unable to save baseline report", slog.String("file", cfg.SaveBaseline), slog.Any("error", err))
		}
	}

	return report, nil
}
