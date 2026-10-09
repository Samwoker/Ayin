package eval

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/seraph/triage/apps/api/internal/agent"
)

// AlertDefinition defines an incoming alert fixture.
type AlertDefinition struct {
	Title       string            `yaml:"title" json:"title"`
	Description string            `yaml:"description" json:"description"`
	Fingerprint string            `yaml:"fingerprint" json:"fingerprint"`
	Labels      map[string]string `yaml:"labels" json:"labels"`
	Annotations map[string]string `yaml:"annotations" json:"annotations"`
}

// MockDeployRecord defines a mock deployment record.
type MockDeployRecord struct {
	Version   string `yaml:"version" json:"version"`
	Commit    string `yaml:"commit" json:"commit"`
	Author    string `yaml:"author" json:"author"`
	Message   string `yaml:"message" json:"message"`
	Timestamp string `yaml:"timestamp" json:"timestamp"`
	Status    string `yaml:"status" json:"status"`
}

// MetricSample represents a single point in time metric value.
type MetricSample struct {
	Timestamp string  `yaml:"timestamp" json:"timestamp"`
	Value     float64 `yaml:"value" json:"value"`
}

// MockMetricData defines simulated metric time series.
type MockMetricData struct {
	Unit   string         `yaml:"unit" json:"unit"`
	Values []MetricSample `yaml:"values" json:"values"`
}

// MockLogRecord defines a single diagnostic log line.
type MockLogRecord struct {
	Timestamp string `yaml:"timestamp" json:"timestamp"`
	Level     string `yaml:"level" json:"level"`
	Service   string `yaml:"service" json:"service"`
	Message   string `yaml:"message" json:"message"`
}

// MockTopologyData defines service dependency relationships.
type MockTopologyData struct {
	Service    string   `yaml:"service" json:"service"`
	Upstream   []string `yaml:"upstream" json:"upstream"`
	Downstream []string `yaml:"downstream" json:"downstream"`
}

// MockRunbookData defines standard operating procedure content.
type MockRunbookData struct {
	Title   string `yaml:"title" json:"title"`
	Service string `yaml:"service" json:"service"`
	Content string `yaml:"content" json:"content"`
}

// MockSimilarData defines a precedent incident record.
type MockSimilarData struct {
	Title      string  `yaml:"title" json:"title"`
	Resolution string  `yaml:"resolution" json:"resolution"`
	Similarity float64 `yaml:"similarity" json:"similarity"`
}

// MockEnvironment models diagnostic tool data fixtures for a scenario.
type MockEnvironment struct {
	Deploys          []MockDeployRecord        `yaml:"deploys" json:"deploys"`
	Metrics          map[string]MockMetricData `yaml:"metrics" json:"metrics"`
	Logs             []MockLogRecord           `yaml:"logs" json:"logs"`
	Topology         MockTopologyData          `yaml:"topology" json:"topology"`
	Runbooks         []MockRunbookData         `yaml:"runbooks" json:"runbooks"`
	SimilarIncidents []MockSimilarData         `yaml:"similar_incidents" json:"similar_incidents"`
}

// GroundTruthCriteria specifies evaluation targets for scoring.
type GroundTruthCriteria struct {
	ExpectedRootCauses  []string `yaml:"expected_root_causes" json:"expected_root_causes"`
	ForbiddenRootCauses []string `yaml:"forbidden_root_causes" json:"forbidden_root_causes"`
	MinConfidence       float64  `yaml:"min_confidence" json:"min_confidence"`
	RequiredEvidence    []string `yaml:"required_evidence" json:"required_evidence"`
	RequiredTools       []string `yaml:"required_tools" json:"required_tools"`
	ExpectedRemediation string   `yaml:"expected_remediation" json:"expected_remediation"`
	MaxSteps            int      `yaml:"max_steps" json:"max_steps"`
	MaxTokens           int      `yaml:"max_tokens" json:"max_tokens"`
}

// Scenario encapsulates a single benchmark incident evaluation case.
type Scenario struct {
	ID          string              `yaml:"id" json:"id"`
	Name        string              `yaml:"name" json:"name"`
	Description string              `yaml:"description" json:"description"`
	Service     string              `yaml:"service" json:"service"`
	Severity    string              `yaml:"severity" json:"severity"`
	Source      string              `yaml:"source" json:"source"`
	Environment MockEnvironment     `yaml:"environment" json:"environment"`
	Alert       AlertDefinition     `yaml:"alert" json:"alert"`
	GroundTruth GroundTruthCriteria `yaml:"ground_truth" json:"ground_truth"`
}

// ToolCallRecord captures execution history of a tool invocation during evaluation.
type ToolCallRecord struct {
	ToolName   string         `json:"tool_name"`
	Arguments  map[string]any `json:"arguments"`
	Success    bool           `json:"success"`
	LatencyMs  int64          `json:"latency_ms"`
	Timestamp  time.Time      `json:"timestamp"`
	OutputSize int            `json:"output_size"`
}

// ScenarioResult details the evaluation outcome for one scenario.
type ScenarioResult struct {
	ScenarioID         string                `json:"scenario_id"`
	ScenarioName       string                `json:"scenario_name"`
	Passed             bool                  `json:"passed"`
	AccuracyScore      float64               `json:"accuracy_score"`      // 0-100
	EvidenceScore      float64               `json:"evidence_score"`      // 0-100
	ConfidenceScore    float64               `json:"confidence_score"`    // 0-100
	ToolEconomyScore   float64               `json:"tool_economy_score"`   // 0-100
	RemediationScore   float64               `json:"remediation_score"`   // 0-100
	HallucinationScore float64               `json:"hallucination_score"` // 0-100 (0 is best)
	OverallScore       float64               `json:"overall_score"`       // 0-100
	StepsCount         int                   `json:"steps_count"`
	TotalTokens        int                   `json:"total_tokens"`
	CostUSD            float64               `json:"cost_usd"`
	Duration           time.Duration         `json:"duration"`
	TopHypothesis      string                `json:"top_hypothesis"`
	TopConfidence      float64               `json:"top_confidence"`
	FailureReasons     []string              `json:"failure_reasons,omitempty"`
	ToolsCalled        []string              `json:"tools_called"`
	Investigation      *agent.InvestigationResult `json:"-"`
}

// ComparisonDelta expresses difference between current and baseline benchmark runs.
type ComparisonDelta struct {
	ScoreDelta         float64 `json:"score_delta"`
	AccuracyDelta      float64 `json:"accuracy_delta"`
	PassRateDelta      float64 `json:"pass_rate_delta"`
	TokensDelta        int     `json:"tokens_delta"`
	CostUSDDelta       float64 `json:"cost_usd_delta"`
	RegressionsCount   int     `json:"regressions_count"`
	ImprovementsCount  int     `json:"improvements_count"`
	RegressionDetails  []string `json:"regression_details,omitempty"`
}

// ComparisonReport compares current benchmark against a prior baseline.
type ComparisonReport struct {
	BaselineRunID string          `json:"baseline_run_id"`
	Delta         ComparisonDelta `json:"delta"`
}

// BenchmarkReport encapsulates aggregate results across all evaluation scenarios.
type BenchmarkReport struct {
	RunID             string            `json:"run_id"`
	Timestamp         time.Time         `json:"timestamp"`
	Provider          string            `json:"provider"`
	Model             string            `json:"model"`
	TotalScenarios    int               `json:"total_scenarios"`
	PassedScenarios   int               `json:"passed_scenarios"`
	PassRate          float64           `json:"pass_rate"`
	MeanOverallScore  float64           `json:"mean_overall_score"`
	MeanAccuracyScore float64           `json:"mean_accuracy_score"`
	MeanEvidenceScore float64           `json:"mean_evidence_score"`
	MeanHallucination float64           `json:"mean_hallucination"`
	MeanSteps         float64           `json:"mean_steps"`
	TotalTokens       int               `json:"total_tokens"`
	TotalCostUSD      float64           `json:"total_cost_usd"`
	TotalDuration     time.Duration     `json:"total_duration"`
	Results           []ScenarioResult  `json:"results"`
	Comparison        *ComparisonReport `json:"comparison,omitempty"`
}

// EvalConfig specifies execution options for running evaluations.
type EvalConfig struct {
	ScenariosDir string
	Provider     string
	Model        string
	APIKey       string
	CompareFile  string
	SaveBaseline string
	PassRateGoal float64 // e.g. 80.0
	OrgID        pgtype.UUID
}
