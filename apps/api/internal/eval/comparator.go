package eval

import (
	"fmt"
)

// CompareWithBaseline calculates differences between the current benchmark run and a prior baseline.
func CompareWithBaseline(current *BenchmarkReport, baseline *BenchmarkReport) *ComparisonReport {
	if baseline == nil {
		return nil
	}

	delta := ComparisonDelta{
		ScoreDelta:    current.MeanOverallScore - baseline.MeanOverallScore,
		AccuracyDelta: current.MeanAccuracyScore - baseline.MeanAccuracyScore,
		PassRateDelta: current.PassRate - baseline.PassRate,
		TokensDelta:   current.TotalTokens - baseline.TotalTokens,
		CostUSDDelta:  current.TotalCostUSD - baseline.TotalCostUSD,
	}

	baselineMap := make(map[string]ScenarioResult)
	for _, b := range baseline.Results {
		baselineMap[b.ScenarioID] = b
	}

	for _, c := range current.Results {
		b, exists := baselineMap[c.ScenarioID]
		if !exists {
			continue
		}

		if b.Passed && !c.Passed {
			delta.RegressionsCount++
			delta.RegressionDetails = append(delta.RegressionDetails,
				fmt.Sprintf("REGRESSION: %s previously passed (score %.1f), now failed (score %.1f)", c.ScenarioID, b.OverallScore, c.OverallScore))
		} else if !b.Passed && c.Passed {
			delta.ImprovementsCount++
		} else if c.OverallScore < b.OverallScore-5.0 {
			delta.RegressionDetails = append(delta.RegressionDetails,
				fmt.Sprintf("SCORE DROP: %s score dropped from %.1f to %.1f (-%.1f)", c.ScenarioID, b.OverallScore, c.OverallScore, b.OverallScore-c.OverallScore))
		}
	}

	return &ComparisonReport{
		BaselineRunID: baseline.RunID,
		Delta:         delta,
	}
}
