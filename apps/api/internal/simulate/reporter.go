package simulate

import (
	"fmt"
	"strings"
)

// RenderSimulationReport formats the simulation result into a clear, detailed terminal log.
func RenderSimulationReport(res *SimulationResult) string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString("====================================================================================================\n")
	sb.WriteString(fmt.Sprintf("                    TRIAGE FAULT-INJECTION SIMULATOR (%s)\n", res.ScenarioID))
	sb.WriteString("====================================================================================================\n")
	sb.WriteString(fmt.Sprintf("Scenario:        %s\n", res.ScenarioName))
	sb.WriteString(fmt.Sprintf("Chaos Level:     %d (%s)\n", res.ChaosLevel, chaosLevelName(res.ChaosLevel)))
	sb.WriteString(fmt.Sprintf("Total Events:    %d\n", res.EventsCount))
	if res.AlertDispatched {
		sb.WriteString(fmt.Sprintf("Webhook Status:  HTTP %d (Incident ID: %s)\n", res.ResponseCode, res.IncidentID))
	} else {
		sb.WriteString("Execution Mode:  Dry-Run (Local synthetic timeline)\n")
	}
	sb.WriteString("----------------------------------------------------------------------------------------------------\n")
	sb.WriteString(fmt.Sprintf("%-10s | %-13s | %-8s | %-6s | %-50s\n",
		"TIME", "CLASS", "TYPE", "LEVEL", "EVENT SUMMARY"))
	sb.WriteString("----------------------------------------------------------------------------------------------------\n")

	for _, e := range res.Timeline {
		classTag := "NOMINAL"
		if e.IsNoise {
			classTag = "CHAOS-NOISE"
		} else if e.IsAdversarial {
			classTag = "ADVERSARIAL"
		}

		sb.WriteString(fmt.Sprintf("%-10s | %-13s | %-8s | %-6s | %-50s\n",
			e.Timestamp.Format("15:04:05"),
			classTag,
			e.Type,
			e.Level,
			truncate(e.Summary, 50),
		))
	}

	sb.WriteString("----------------------------------------------------------------------------------------------------\n")
	for _, l := range res.OutputLogs {
		sb.WriteString(fmt.Sprintf(">> %s\n", l))
	}
	sb.WriteString("====================================================================================================\n\n")

	return sb.String()
}

func chaosLevelName(c ChaosLevel) string {
	switch c {
	case ChaosLevelNone:
		return "Clean Trace"
	case ChaosLevelNoise:
		return "Background Noise"
	case ChaosLevelJitter:
		return "Transient Jitter & Retries"
	case ChaosLevelAdversarial:
		return "Adversarial Red Herrings"
	default:
		return "Custom"
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
