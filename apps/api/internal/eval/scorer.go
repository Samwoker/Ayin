package eval

import (
	"fmt"
	"strings"

	"github.com/seraph/triage/apps/api/internal/agent"
)

// ScoreScenario evaluates an agent investigation run against scenario ground truth criteria.
func ScoreScenario(sc *Scenario, res *agent.InvestigationResult, tracker *ToolCallTracker) ScenarioResult {
	result := ScenarioResult{
		ScenarioID:   sc.ID,
		ScenarioName: sc.Name,
		Investigation: res,
	}

	if res == nil {
		result.FailureReasons = append(result.FailureReasons, "investigation returned nil result")
		return result
	}

	result.StepsCount = res.StepsCount
	result.TotalTokens = res.TotalTokens
	result.CostUSD = res.TotalCostUSD
	result.Duration = res.Duration

	if tracker != nil {
		result.ToolsCalled = tracker.CalledTools()
	}

	if res.Status != "succeeded" {
		result.FailureReasons = append(result.FailureReasons, fmt.Sprintf("investigation ended with status %q", res.Status))
	}

	if len(res.Hypotheses) == 0 {
		result.FailureReasons = append(result.FailureReasons, "agent produced 0 hypotheses")
		return result
	}

	topHyp := res.Hypotheses[0]
	result.TopHypothesis = topHyp.RootCause
	result.TopConfidence = topHyp.Confidence

	gt := sc.GroundTruth

	// 1. Accuracy Score (0 - 100)
	accScore := 0.0
	matchedExpected := 0
	for _, expected := range gt.ExpectedRootCauses {
		if containsIgnoreCase(topHyp.RootCause, expected) {
			matchedExpected++
		}
	}

	if len(gt.ExpectedRootCauses) > 0 {
		accScore = (float64(matchedExpected) / float64(len(gt.ExpectedRootCauses))) * 100.0
		if matchedExpected > 0 && accScore < 80.0 {
			// At least one core root cause matched
			accScore = 80.0 + (float64(matchedExpected)/float64(len(gt.ExpectedRootCauses)))*20.0
		}
	} else {
		accScore = 100.0
	}

	// Check forbidden anti-hallucination causes
	for _, forbidden := range gt.ForbiddenRootCauses {
		if containsIgnoreCase(topHyp.RootCause, forbidden) {
			accScore -= 40.0
			result.FailureReasons = append(result.FailureReasons, fmt.Sprintf("hallucinated forbidden root cause: %q", forbidden))
		}
	}
	result.AccuracyScore = clamp(accScore, 0, 100)
	if result.AccuracyScore < 70 {
		result.FailureReasons = append(result.FailureReasons, "top hypothesis failed to identify ground truth root cause")
	}

	// 2. Confidence Calibration Score (0 - 100)
	confScore := 100.0
	if topHyp.Confidence < gt.MinConfidence {
		confScore -= (gt.MinConfidence - topHyp.Confidence) * 100.0
		result.FailureReasons = append(result.FailureReasons, fmt.Sprintf("confidence %.2f below threshold %.2f", topHyp.Confidence, gt.MinConfidence))
	} else if topHyp.Confidence > 1.0 {
		confScore = 50.0
		result.FailureReasons = append(result.FailureReasons, "confidence exceeds 1.0")
	}
	result.ConfidenceScore = clamp(confScore, 0, 100)

	// 3. Evidence Score (0 - 100)
	evidScore := 0.0
	if len(topHyp.Evidence) == 0 {
		evidScore = 0.0
		result.FailureReasons = append(result.FailureReasons, "hypothesis contains zero evidence citations")
	} else {
		matchedEvid := 0
		allEvidenceStr := strings.Join(topHyp.Evidence, " ")
		for _, req := range gt.RequiredEvidence {
			if containsIgnoreCase(allEvidenceStr, req) {
				matchedEvid++
			}
		}

		if len(gt.RequiredEvidence) > 0 {
			evidScore = (float64(matchedEvid) / float64(len(gt.RequiredEvidence))) * 100.0
		} else {
			evidScore = 90.0
		}
	}
	result.EvidenceScore = clamp(evidScore, 0, 100)
	if result.EvidenceScore < 60 {
		result.FailureReasons = append(result.FailureReasons, "missing required evidence citations")
	}

	// 4. Hallucination Score (0 - 100, 0 is ideal)
	hallucScore := 0.0
	// Check if hypothesis references tools that were never called
	if tracker != nil {
		calledMap := make(map[string]bool)
		for _, c := range tracker.CalledTools() {
			calledMap[c] = true
		}
		for _, e := range topHyp.Evidence {
			for _, knownTool := range []string{"recent_deploys", "query_metrics", "log_search", "service_topology", "search_runbooks", "similar_incidents"} {
				if containsIgnoreCase(e, knownTool) && !calledMap[knownTool] {
					hallucScore += 30.0
					result.FailureReasons = append(result.FailureReasons, fmt.Sprintf("hallucinated citation to uncalled tool %s", knownTool))
				}
			}
		}
	}
	result.HallucinationScore = clamp(hallucScore, 0, 100)

	// 5. Tool Economy Score (0 - 100)
	econScore := 100.0
	if tracker != nil {
		calledMap := make(map[string]bool)
		for _, c := range tracker.CalledTools() {
			calledMap[c] = true
		}
		missingTools := 0
		for _, reqTool := range gt.RequiredTools {
			if !calledMap[reqTool] {
				missingTools++
			}
		}
		if len(gt.RequiredTools) > 0 && missingTools > 0 {
			penalty := (float64(missingTools) / float64(len(gt.RequiredTools))) * 50.0
			econScore -= penalty
			result.FailureReasons = append(result.FailureReasons, fmt.Sprintf("missed %d required diagnostic tools", missingTools))
		}
	}
	if gt.MaxSteps > 0 && res.StepsCount > gt.MaxSteps {
		econScore -= float64(res.StepsCount-gt.MaxSteps) * 10.0
		result.FailureReasons = append(result.FailureReasons, fmt.Sprintf("exceeded maximum budget of %d steps", gt.MaxSteps))
	}
	result.ToolEconomyScore = clamp(econScore, 0, 100)

	// 6. Remediation Score (0 - 100)
	remedScore := 0.0
	checklistStr := strings.Join(topHyp.Checklist, " ")
	if gt.ExpectedRemediation != "" {
		if containsIgnoreCase(checklistStr, gt.ExpectedRemediation) {
			remedScore = 100.0
		} else {
			remedScore = 40.0 // partial score for having a checklist
			result.FailureReasons = append(result.FailureReasons, fmt.Sprintf("checklist missing expected remediation %q", gt.ExpectedRemediation))
		}
	} else {
		if len(topHyp.Checklist) > 0 {
			remedScore = 100.0
		} else {
			remedScore = 0.0
		}
	}
	result.RemediationScore = clamp(remedScore, 0, 100)

	// 7. Overall Weighted Score (0 - 100)
	// Weights: 35% Accuracy + 25% Evidence + 15% Economy + 15% Remediation + 10% Confidence - (20% Hallucination)
	overall := (0.35 * result.AccuracyScore) +
		(0.25 * result.EvidenceScore) +
		(0.15 * result.ToolEconomyScore) +
		(0.15 * result.RemediationScore) +
		(0.10 * result.ConfidenceScore) -
		(0.20 * result.HallucinationScore)

	result.OverallScore = clamp(overall, 0, 100)

	// Pass conditions: Overall >= 80, Accuracy >= 80, Evidence >= 60, Hallucination < 20, No fatal failure
	result.Passed = result.OverallScore >= 80.0 &&
		result.AccuracyScore >= 80.0 &&
		result.EvidenceScore >= 60.0 &&
		result.HallucinationScore < 20.0 &&
		len(result.FailureReasons) == 0

	return result
}

func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
