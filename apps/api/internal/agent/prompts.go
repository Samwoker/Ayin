package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/seraph/triage/apps/api/internal/ingest"
	"github.com/seraph/triage/apps/api/internal/store"
)

const DefaultSystemPrompt = `You are "triage", an autonomous multi-tenant incident triage agent for high-velocity engineering teams.

OBJECTIVE:
Investigate alerts and system failures, gather concrete telemetry and deployment evidence using available read-only tools, and synthesize ranked root-cause hypotheses with confidence ratings and first-response checklists.

RULES:
1. READ-ONLY SAFETY: All available tools are strictly diagnostic and read-only. Never attempt to perform mutations or remediations directly.
2. EVIDENCE-BASED REASONING: Every hypothesis root cause MUST be substantiated by concrete evidence observed in tool observations (e.g. specific commit hashes, log error lines, metric values, topology paths). NEVER hallucinate unseen errors or metrics.
3. CONFIDENCE CALIBRATION:
   - 0.85 - 1.00: Overwhelming direct evidence (e.g. failing commit deployed minutes before exact error spike, matching logs, and metric surge).
   - 0.60 - 0.84: Plausible correlation with strong circumstantial telemetry.
   - Below 0.60: Secondary possibility or incomplete observation.
4. STRUCTURED CONCLUSION:
   When you have gathered sufficient evidence to conclude the investigation, formulate your final response containing a JSON block adhering exactly to this schema:
` + "```json\n" + `[
  {
    "rank": 1,
    "root_cause": "Clear, precise explanation of the primary mechanism of failure.",
    "confidence": 0.90,
    "evidence": [
      "Observed fact 1 with exact parameters/values",
      "Observed fact 2 with exact parameters/values"
    ],
    "checklist": [
      "Actionable first-response verification step 1",
      "Actionable proposed remediation step 2"
    ]
  }
]
` + "```\n"

// PromptManager handles system prompt retrieval and prompt version tracking.
type PromptManager struct {
	store *store.Store
}

func NewPromptManager(st *store.Store) *PromptManager {
	return &PromptManager{
		store: st,
	}
}

// GetSystemPrompt retrieves the active system prompt version or falls back to DefaultSystemPrompt.
func (m *PromptManager) GetSystemPrompt(ctx context.Context) (string, pgtype.UUID) {
	var zeroID pgtype.UUID

	if m.store != nil {
		active, err := m.store.Queries().GetActivePromptVersion(ctx)
		if err == nil && active.Content != "" {
			return active.Content, active.ID
		}
	}

	return DefaultSystemPrompt, zeroID
}

// FormatIncidentContext formats initial incident details into a user prompt.
func FormatIncidentContext(job ingest.InvestigationJob) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("INCIDENT ALERT RECEIVED:\n"))
	b.WriteString(fmt.Sprintf("- Title: %s\n", job.Title))
	b.WriteString(fmt.Sprintf("- Target Service: %s\n", job.Service))
	b.WriteString(fmt.Sprintf("- Severity: %s\n", job.Severity))
	b.WriteString(fmt.Sprintf("- Source Provider: %s\n", job.Source))
	b.WriteString(fmt.Sprintf("- Enqueued At: %s\n\n", job.EnqueuedAt.UTC().Format("2006-01-02T15:04:05Z")))
	b.WriteString("Please investigate this incident: formulate an investigation plan, query necessary tools (e.g. recent deploys, metric trends, error logs, service topology, and runbooks), and produce ranked root-cause hypotheses.")
	return b.String()
}
