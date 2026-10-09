package remediation

import (
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// Standard Action Types matching database CHECK constraint
const (
	ActionRestartService    = "restart_service"
	ActionScaleService      = "scale_service"
	ActionRollbackDeploy    = "rollback_deploy"
	ActionClearCache        = "clear_cache"
	ActionPauseConsumer     = "pause_consumer"
	ActionToggleFeatureFlag = "toggle_feature_flag"
)

// Standard Risk Levels matching database CHECK constraint
const (
	RiskLow      = "low"
	RiskMedium   = "medium"
	RiskHigh     = "high"
	RiskCritical = "critical"
)

// Standard Statuses matching database CHECK constraint
const (
	StatusProposed   = "proposed"
	StatusApproved   = "approved"
	StatusRejected   = "rejected"
	StatusExpired    = "expired"
	StatusExecuting  = "executing"
	StatusSucceeded  = "succeeded"
	StatusFailed     = "failed"
	StatusRolledBack = "rolled_back"
)

var (
	ErrRemediationNotFound = errors.New("remediation not found")
	ErrRemediationExpired  = errors.New("remediation has expired")
	ErrInvalidTransition   = errors.New("invalid remediation status transition")
	ErrInvalidAction       = errors.New("invalid remediation action type")
	ErrInvalidRisk         = errors.New("invalid remediation risk level")
	ErrMissingTarget       = errors.New("remediation target is required")
)

var validActions = map[string]bool{
	ActionRestartService:    true,
	ActionScaleService:      true,
	ActionRollbackDeploy:    true,
	ActionClearCache:        true,
	ActionPauseConsumer:     true,
	ActionToggleFeatureFlag: true,
}

var validRisks = map[string]bool{
	RiskLow:      true,
	RiskMedium:   true,
	RiskHigh:     true,
	RiskCritical: true,
}

// IsValidAction checks if action is recognized
func IsValidAction(action string) bool {
	return validActions[action]
}

// IsValidRisk checks if risk level is recognized
func IsValidRisk(risk string) bool {
	return validRisks[risk]
}

// CreateRemediationInput holds input parameters to propose a new remediation
type CreateRemediationInput struct {
	IncidentID     pgtype.UUID    `json:"incident_id"`
	HypothesisID   pgtype.UUID    `json:"hypothesis_id,omitempty"`
	ActionType     string         `json:"action_type"`
	Target         string         `json:"target"`
	Params         map[string]any `json:"params"`
	Rationale      string         `json:"rationale"`
	RiskLevel      string         `json:"risk_level"`
	ExpectedEffect string         `json:"expected_effect"`
	RollbackPlan   string         `json:"rollback_plan"`
	TTL            time.Duration  `json:"ttl,omitempty"`
}

// ExecutionResult represents the outcome of executing a remediation action
type ExecutionResult struct {
	Success    bool           `json:"success"`
	Action     string         `json:"action"`
	Target     string         `json:"target"`
	Details    map[string]any `json:"details"`
	Error      string         `json:"error,omitempty"`
	ExecutedAt time.Time      `json:"executed_at"`
}
