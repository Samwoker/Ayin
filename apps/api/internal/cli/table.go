package cli

import (
	"fmt"
	"strings"
	"time"
)

func RenderIncidentsTable(incidents []IncidentDTO) string {
	if len(incidents) == 0 {
		return "No incidents found.\n"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%-36s | %-6s | %-10s | %-18s | %-28s | %s\n",
		"INCIDENT ID", "SEV", "STATUS", "SERVICE", "TITLE", "CREATED"))
	sb.WriteString(strings.Repeat("-", 120) + "\n")

	for _, inc := range incidents {
		sb.WriteString(fmt.Sprintf("%-36s | %-6s | %-10s | %-18s | %-28s | %s\n",
			inc.ID,
			inc.Severity,
			inc.Status,
			truncate(inc.Service, 18),
			truncate(inc.Title, 28),
			inc.CreatedAt.Format("2006-01-02 15:04"),
		))
	}
	return sb.String()
}

func RenderIncidentDetail(inc *IncidentDTO) string {
	var sb strings.Builder
	sb.WriteString("====================================================================================================\n")
	sb.WriteString(fmt.Sprintf("INCIDENT: %s\n", inc.Title))
	sb.WriteString("====================================================================================================\n")
	sb.WriteString(fmt.Sprintf("ID:          %s\n", inc.ID))
	sb.WriteString(fmt.Sprintf("Service:     %s\n", inc.Service))
	sb.WriteString(fmt.Sprintf("Severity:    %s\n", inc.Severity))
	sb.WriteString(fmt.Sprintf("Status:      %s\n", inc.Status))
	sb.WriteString(fmt.Sprintf("Created At:  %s\n", inc.CreatedAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("Updated At:  %s\n", inc.UpdatedAt.Format(time.RFC3339)))
	if inc.Description != "" {
		sb.WriteString(fmt.Sprintf("Description: %s\n", inc.Description))
	}
	sb.WriteString("====================================================================================================\n")
	return sb.String()
}

func RenderRunsTable(runs []RunDTO) string {
	if len(runs) == 0 {
		return "No investigation runs found for this incident.\n"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%-36s | %-12s | %-6s | %-8s | %-10s | %s\n",
		"RUN ID", "STATUS", "STEPS", "TOKENS", "COST", "CREATED"))
	sb.WriteString(strings.Repeat("-", 95) + "\n")

	for _, r := range runs {
		sb.WriteString(fmt.Sprintf("%-36s | %-12s | %-6d | %-8d | $%-9.4f | %s\n",
			r.ID,
			r.Status,
			r.StepsCount,
			r.TotalTokens,
			r.TotalCostUSD,
			r.CreatedAt.Format("15:04:05"),
		))
	}
	return sb.String()
}

func RenderRemediationsTable(rems []RemediationDTO) string {
	if len(rems) == 0 {
		return "No proposed or executed remediations found.\n"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%-36s | %-18s | %-10s | %-10s | %-20s | %s\n",
		"REMEDIATION ID", "ACTION", "RISK", "STATUS", "TARGET", "EXPIRES"))
	sb.WriteString(strings.Repeat("-", 115) + "\n")

	now := time.Now().UTC()
	for _, r := range rems {
		expiresStr := "Expired"
		if r.ExpiresAt.After(now) {
			diff := time.Until(r.ExpiresAt).Round(time.Second)
			expiresStr = fmt.Sprintf("in %v", diff)
		}

		sb.WriteString(fmt.Sprintf("%-36s | %-18s | %-10s | %-10s | %-20s | %s\n",
			r.ID,
			r.Action,
			r.RiskTier,
			r.Status,
			truncate(r.Target, 20),
			expiresStr,
		))
	}
	return sb.String()
}

func RenderRemediationDetail(rem *RemediationDTO) string {
	var sb strings.Builder
	sb.WriteString("====================================================================================================\n")
	sb.WriteString(fmt.Sprintf("REMEDIATION: %s on %s\n", rem.Action, rem.Target))
	sb.WriteString("====================================================================================================\n")
	sb.WriteString(fmt.Sprintf("ID:              %s\n", rem.ID))
	sb.WriteString(fmt.Sprintf("Incident ID:     %s\n", rem.IncidentID))
	sb.WriteString(fmt.Sprintf("Risk Tier:       %s\n", rem.RiskTier))
	sb.WriteString(fmt.Sprintf("Status:          %s\n", rem.Status))
	sb.WriteString(fmt.Sprintf("Rationale:       %s\n", rem.Rationale))
	sb.WriteString(fmt.Sprintf("Expected Effect: %s\n", rem.ExpectedEffect))
	sb.WriteString(fmt.Sprintf("Rollback Plan:   %s\n", rem.RollbackPlan))
	sb.WriteString(fmt.Sprintf("Expires At:      %s\n", rem.ExpiresAt.Format(time.RFC3339)))
	if rem.ExecutedAt != nil {
		sb.WriteString(fmt.Sprintf("Executed At:     %s\n", rem.ExecutedAt.Format(time.RFC3339)))
	}
	sb.WriteString("====================================================================================================\n")
	return sb.String()
}

func RenderToolsTable(tools []ToolDTO) string {
	if len(tools) == 0 {
		return "No tools registered.\n"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%-22s | %s\n", "TOOL NAME", "DESCRIPTION"))
	sb.WriteString(strings.Repeat("-", 90) + "\n")

	for _, t := range tools {
		sb.WriteString(fmt.Sprintf("%-22s | %s\n", t.Name, t.Description))
	}
	return sb.String()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
