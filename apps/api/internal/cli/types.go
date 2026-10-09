package cli

import (
	"time"
)

// Config holds client credentials and endpoint URLs for the CLI.
type Config struct {
	APIURL string `json:"api_url"`
	APIKey string `json:"api_key"`
	Token  string `json:"token"`
	OrgID  string `json:"org_id"`
	JSON   bool   `json:"json"`
}

// IncidentDTO represents an incident returned from the API.
type IncidentDTO struct {
	ID          string    `json:"id"`
	OrgID       string    `json:"org_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Service     string    `json:"service"`
	Severity    string    `json:"severity"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// RunDTO represents an investigation run.
type RunDTO struct {
	ID           string    `json:"id"`
	IncidentID   string    `json:"incident_id"`
	Status       string    `json:"status"`
	TotalTokens  int       `json:"total_tokens"`
	TotalCostUSD float64   `json:"total_cost_usd"`
	StepsCount   int       `json:"steps_count"`
	CreatedAt    time.Time `json:"created_at"`
}

// RemediationDTO represents a proposed or executed remediation.
type RemediationDTO struct {
	ID             string    `json:"id"`
	IncidentID     string    `json:"incident_id"`
	Action         string    `json:"action"`
	Target         string    `json:"target"`
	RiskTier       string    `json:"risk_tier"`
	Status         string    `json:"status"`
	Rationale      string    `json:"rationale"`
	ExpectedEffect string    `json:"expected_effect"`
	RollbackPlan   string    `json:"rollback_plan"`
	ExpiresAt      time.Time `json:"expires_at"`
	CreatedAt      time.Time `json:"created_at"`
	ExecutedAt     *time.Time `json:"executed_at,omitempty"`
}

// RemediationEventDTO represents an audit event on a remediation.
type RemediationEventDTO struct {
	ID             string    `json:"id"`
	RemediationID  string    `json:"remediation_id"`
	TransitionFrom string    `json:"transition_from"`
	TransitionTo   string    `json:"transition_to"`
	UserID         string    `json:"user_id"`
	Reason         string    `json:"reason"`
	CreatedAt      time.Time `json:"created_at"`
}

// ToolDTO represents a registered diagnostic tool.
type ToolDTO struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}
