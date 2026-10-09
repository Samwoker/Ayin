package simulate

import (
	"time"

	"github.com/seraph/triage/apps/api/internal/eval"
)

// ChaosLevel indicates the degree of noise, jitter, and adversarial distractors injected.
type ChaosLevel int

const (
	ChaosLevelNone        ChaosLevel = 0 // Clean reproducible scenario trace
	ChaosLevelNoise       ChaosLevel = 1 // Background log noise and non-critical metric jitter
	ChaosLevelJitter      ChaosLevel = 2 // Connection blips, transient retry warnings, and timestamp skew
	ChaosLevelAdversarial ChaosLevel = 3 // Deceptive red-herring logs and concurrent benign deploys
)

// SimulationConfig specifies options for fault injection simulation.
type SimulationConfig struct {
	ScenarioName  string        `json:"scenario_name"`
	ScenariosDir  string        `json:"scenarios_dir"`
	ChaosLevel    ChaosLevel    `json:"chaos_level"`
	Seed          int64         `json:"seed"`
	TargetURL     string        `json:"target_url"`
	WebhookSecret string        `json:"webhook_secret"`
	DryRun        bool          `json:"dry_run"`
	Watch         bool          `json:"watch"`
	AutoApprove   bool          `json:"auto_approve"`
	Interval      time.Duration `json:"interval"`
}

// SimulatedEvent represents an event in the synthetic fault-injection timeline.
type SimulatedEvent struct {
	Timestamp     time.Time `json:"timestamp"`
	Type          string    `json:"type"` // alert, metric, log, deploy
	Service       string    `json:"service"`
	Level         string    `json:"level,omitempty"`
	Summary       string    `json:"summary"`
	Payload       any       `json:"payload"`
	IsNoise       bool      `json:"is_noise"`
	IsAdversarial bool      `json:"is_adversarial"`
}

// SimulationResult captures the outcome of a simulation execution.
type SimulationResult struct {
	ScenarioID       string           `json:"scenario_id"`
	ScenarioName     string           `json:"scenario_name"`
	ChaosLevel       ChaosLevel       `json:"chaos_level"`
	EventsCount      int              `json:"events_count"`
	AlertDispatched  bool             `json:"alert_dispatched"`
	ResponseCode     int              `json:"response_code,omitempty"`
	IncidentID       string           `json:"incident_id,omitempty"`
	Timeline         []SimulatedEvent `json:"timeline"`
	InvestigationRun *eval.ScenarioResult `json:"investigation_run,omitempty"`
	Duration         time.Duration    `json:"duration"`
	OutputLogs       []string         `json:"output_logs"`
}
