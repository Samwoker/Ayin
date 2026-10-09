package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// RenderTextReport formats the evaluation benchmark results as a clean, high-density terminal table.
func RenderTextReport(r *BenchmarkReport) string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString("====================================================================================================\n")
	sb.WriteString(fmt.Sprintf("                        TRIAGE BENCHMARK EVALUATION REPORT (%s)\n", r.Provider))
	sb.WriteString("====================================================================================================\n")
	sb.WriteString(fmt.Sprintf("Run ID:        %s\n", r.RunID))
	sb.WriteString(fmt.Sprintf("Timestamp:     %s\n", r.Timestamp.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("Model:         %s\n", r.Model))
	sb.WriteString(fmt.Sprintf("Duration:      %v\n", r.TotalDuration.Round(time.Millisecond)))
	sb.WriteString("----------------------------------------------------------------------------------------------------\n")
	sb.WriteString(fmt.Sprintf("%-28s | %-6s | %-6s | %-6s | %-6s | %-6s | %-6s | %-5s | %-7s\n",
		"SCENARIO", "STATUS", "ACCUR", "EVID", "ECON", "HALLUC", "SCORE", "STEPS", "TOKENS"))
	sb.WriteString("----------------------------------------------------------------------------------------------------\n")

	for _, sc := range r.Results {
		statusStr := "FAIL"
		if sc.Passed {
			statusStr = "PASS"
		}

		sb.WriteString(fmt.Sprintf("%-28s | %-6s | %5.1f%% | %5.1f%% | %5.1f%% | %5.1f%% | %5.1f%% | %5d | %7d\n",
			truncate(sc.ScenarioID, 28),
			statusStr,
			sc.AccuracyScore,
			sc.EvidenceScore,
			sc.ToolEconomyScore,
			sc.HallucinationScore,
			sc.OverallScore,
			sc.StepsCount,
			sc.TotalTokens,
		))
	}

	sb.WriteString("----------------------------------------------------------------------------------------------------\n")
	sb.WriteString(fmt.Sprintf("OVERALL PASS RATE:    %.1f%% (%d/%d passed)\n", r.PassRate, r.PassedScenarios, r.TotalScenarios))
	sb.WriteString(fmt.Sprintf("MEAN OVERALL SCORE:   %.1f%%\n", r.MeanOverallScore))
	sb.WriteString(fmt.Sprintf("MEAN ACCURACY:        %.1f%%\n", r.MeanAccuracyScore))
	sb.WriteString(fmt.Sprintf("MEAN EVIDENCE SCORE:  %.1f%%\n", r.MeanEvidenceScore))
	sb.WriteString(fmt.Sprintf("MEAN HALLUCINATION:   %.1f%%\n", r.MeanHallucination))
	sb.WriteString(fmt.Sprintf("MEAN STEPS PER RUN:   %.1f\n", r.MeanSteps))
	sb.WriteString(fmt.Sprintf("TOTAL TOKENS / COST:  %d tokens ($%.4f USD)\n", r.TotalTokens, r.TotalCostUSD))

	if r.Comparison != nil {
		sb.WriteString("----------------------------------------------------------------------------------------------------\n")
		sb.WriteString("COMPARISON WITH BASELINE:\n")
		cmp := r.Comparison
		sb.WriteString(fmt.Sprintf("  Baseline ID:        %s\n", cmp.BaselineRunID))
		sb.WriteString(fmt.Sprintf("  Score Delta:        %+5.1f%%\n", cmp.Delta.ScoreDelta))
		sb.WriteString(fmt.Sprintf("  Accuracy Delta:     %+5.1f%%\n", cmp.Delta.AccuracyDelta))
		sb.WriteString(fmt.Sprintf("  Pass Rate Delta:    %+5.1f%%\n", cmp.Delta.PassRateDelta))
		sb.WriteString(fmt.Sprintf("  Regressions:        %d\n", cmp.Delta.RegressionsCount))
		sb.WriteString(fmt.Sprintf("  Improvements:       %d\n", cmp.Delta.ImprovementsCount))
		for _, det := range cmp.Delta.RegressionDetails {
			sb.WriteString(fmt.Sprintf("    ! %s\n", det))
		}
	}

	// Print failure details if any
	hasFailures := false
	for _, sc := range r.Results {
		if !sc.Passed && len(sc.FailureReasons) > 0 {
			if !hasFailures {
				sb.WriteString("----------------------------------------------------------------------------------------------------\n")
				sb.WriteString("FAILURE DIAGNOSTICS:\n")
				hasFailures = true
			}
			sb.WriteString(fmt.Sprintf("  [%s]:\n", sc.ScenarioID))
			for _, reason := range sc.FailureReasons {
				sb.WriteString(fmt.Sprintf("    - %s\n", reason))
			}
		}
	}

	sb.WriteString("====================================================================================================\n\n")
	return sb.String()
}

// RenderMarkdownReport formats the report for GitHub Actions or pull request comments.
func RenderMarkdownReport(r *BenchmarkReport) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## 🔬 Triage Benchmark Evaluation Report (%s)\n\n", r.Provider))
	sb.WriteString(fmt.Sprintf("- **Run ID**: `%s`\n", r.RunID))
	sb.WriteString(fmt.Sprintf("- **Model**: `%s`\n", r.Model))
	sb.WriteString(fmt.Sprintf("- **Pass Rate**: **%.1f%%** (%d/%d scenarios passed)\n", r.PassRate, r.PassedScenarios, r.TotalScenarios))
	sb.WriteString(fmt.Sprintf("- **Mean Score**: **%.1f%%** | Accuracy: **%.1f%%** | Evidence: **%.1f%%**\n", r.MeanOverallScore, r.MeanAccuracyScore, r.MeanEvidenceScore))
	sb.WriteString(fmt.Sprintf("- **Total Tokens / Cost**: %d tokens (~$%.4f USD)\n\n", r.TotalTokens, r.TotalCostUSD))

	sb.WriteString("| Scenario | Status | Accuracy | Evidence | Economy | Hallucination | Score | Steps | Tokens |\n")
	sb.WriteString("| :--- | :---: | :---: | :---: | :---: | :---: | :---: | :---: | :---: |\n")

	for _, sc := range r.Results {
		statusPill := "❌ FAIL"
		if sc.Passed {
			statusPill = "✅ PASS"
		}
		sb.WriteString(fmt.Sprintf("| `%s` | %s | %.1f%% | %.1f%% | %.1f%% | %.1f%% | **%.1f%%** | %d | %d |\n",
			sc.ScenarioID,
			statusPill,
			sc.AccuracyScore,
			sc.EvidenceScore,
			sc.ToolEconomyScore,
			sc.HallucinationScore,
			sc.OverallScore,
			sc.StepsCount,
			sc.TotalTokens,
		))
	}

	if r.Comparison != nil {
		cmp := r.Comparison
		sb.WriteString("\n### 📊 Comparison Against Baseline\n\n")
		sb.WriteString(fmt.Sprintf("- Baseline: `%s`\n", cmp.BaselineRunID))
		sb.WriteString(fmt.Sprintf("- Score Delta: `%+5.1f%%` | Accuracy Delta: `%+5.1f%%`\n", cmp.Delta.ScoreDelta, cmp.Delta.AccuracyDelta))
		sb.WriteString(fmt.Sprintf("- Regressions: **%d** | Improvements: **%d**\n\n", cmp.Delta.RegressionsCount, cmp.Delta.ImprovementsCount))
	}

	return sb.String()
}

// SaveReportJSON writes the report to a JSON file.
func SaveReportJSON(report *BenchmarkReport, path string) error {
	bytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, bytes, 0644)
}

// LoadReportJSON reads a prior benchmark report from a JSON file.
func LoadReportJSON(path string) (*BenchmarkReport, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var report BenchmarkReport
	if err := json.Unmarshal(bytes, &report); err != nil {
		return nil, err
	}
	return &report, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
