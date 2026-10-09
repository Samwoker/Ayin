package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// MetricPoint represents an individual sample timestamp and scalar value.
type MetricPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// MetricSeries represents a timeseries result with calculated summary statistics.
type MetricSeries struct {
	Metric map[string]string `json:"metric"`
	Points []MetricPoint     `json:"points"`
	Min    float64           `json:"min"`
	Max    float64           `json:"max"`
	Avg    float64           `json:"avg"`
	Latest float64           `json:"latest"`
}

// MetricQuery represents metric request parameters.
type MetricQuery struct {
	Query     string
	Service   string
	StartTime time.Time
	EndTime   time.Time
	Step      time.Duration
}

// MetricProvider queries metrics from Prometheus or an in-memory telemetry simulator.
type MetricProvider interface {
	QueryMetrics(ctx context.Context, orgID pgtype.UUID, q MetricQuery) ([]MetricSeries, error)
}

// SimulatedMetricProvider generates realistic timeseries trends and anomalies for incident triage.
type SimulatedMetricProvider struct{}

func NewSimulatedMetricProvider() *SimulatedMetricProvider {
	return &SimulatedMetricProvider{}
}

func (p *SimulatedMetricProvider) QueryMetrics(ctx context.Context, orgID pgtype.UUID, q MetricQuery) ([]MetricSeries, error) {
	now := time.Now().UTC()
	start := q.StartTime
	if start.IsZero() {
		start = now.Add(-30 * time.Minute)
	}
	end := q.EndTime
	if end.IsZero() {
		end = now
	}
	step := q.Step
	if step <= 0 {
		step = time.Minute
	}

	qLower := strings.ToLower(q.Query)
	serviceName := q.Service
	if serviceName == "" {
		serviceName = "service"
	}

	// Generate 15-30 realistic sample points across the requested range
	var points []MetricPoint
	var sum float64
	minVal := math.MaxFloat64
	maxVal := -math.MaxFloat64

	t := start
	idx := 0
	for !t.After(end) {
		var val float64

		if strings.Contains(qLower, "queue") || strings.Contains(qLower, "depth") || strings.Contains(qLower, "backlog") {
			// Simulates queue depth surging from normal 120 up to 58,000
			if idx < 10 {
				val = 120.0 + float64(idx*15)
			} else {
				val = 45000.0 + float64((idx-10)*1300)
			}
		} else if strings.Contains(qLower, "cpu") {
			// Simulates CPU surging from 22% up to 98%
			if idx < 12 {
				val = 22.0 + float64(idx%4)
			} else {
				val = 88.0 + float64(idx%10)
			}
		} else if strings.Contains(qLower, "memory") || strings.Contains(qLower, "mem") {
			// Simulates memory leak climbing continuously
			val = 40.0 + float64(idx)*2.5
		} else if strings.Contains(qLower, "5xx") || strings.Contains(qLower, "error") {
			// Simulates error rate spiking from 0.01% to 14.5%
			if idx < 15 {
				val = 0.02
			} else {
				val = 12.8 + float64(idx%3)
			}
		} else if strings.Contains(qLower, "latency") || strings.Contains(qLower, "duration") {
			// Simulates p95 latency spiking from 65ms to 2400ms
			if idx < 14 {
				val = 65.0 + float64(idx*2)
			} else {
				val = 2100.0 + float64(idx*40)
			}
		} else {
			// Default baseline metric with minor fluctuations
			val = 100.0 + float64(idx%10)
		}

		points = append(points, MetricPoint{
			Timestamp: t,
			Value:     math.Round(val*100) / 100,
		})

		if val < minVal {
			minVal = val
		}
		if val > maxVal {
			maxVal = val
		}
		sum += val

		idx++
		t = t.Add(step)
	}

	if len(points) == 0 {
		return []MetricSeries{}, nil
	}

	avg := sum / float64(len(points))
	latest := points[len(points)-1].Value

	series := MetricSeries{
		Metric: map[string]string{
			"__name__": strings.Split(q.Query, "{")[0],
			"service":  serviceName,
			"job":      serviceName,
		},
		Points: points,
		Min:    math.Round(minVal*100) / 100,
		Max:    math.Round(maxVal*100) / 100,
		Avg:    math.Round(avg*100) / 100,
		Latest: math.Round(latest*100) / 100,
	}

	return []MetricSeries{series}, nil
}

// QueryMetricsTool queries metric series for threshold breaches, anomaly trends, and SLO checks.
type QueryMetricsTool struct {
	provider MetricProvider
}

func NewQueryMetricsTool(provider MetricProvider) *QueryMetricsTool {
	if provider == nil {
		provider = NewSimulatedMetricProvider()
	}
	return &QueryMetricsTool{
		provider: provider,
	}
}

func (t *QueryMetricsTool) Name() string {
	return "query_metrics"
}

func (t *QueryMetricsTool) Description() string {
	return "Executes metric queries (e.g. PromQL expressions) over a time window to analyze CPU, memory, queue backlog, latency percentiles, and error rate spikes."
}

func (t *QueryMetricsTool) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Required: []string{"query"},
		Properties: map[string]PropertyDoc{
			"query": {
				Type:        "string",
				Description: "PromQL expression or metric name (e.g. 'rabbitmq_queue_messages_ready', 'container_cpu_usage_seconds_total', 'http_request_duration_seconds').",
			},
			"service": {
				Type:        "string",
				Description: "Target service name to filter metric timeseries.",
			},
			"start_time": {
				Type:        "string",
				Format:      "date-time",
				Description: "Start of metric query window (RFC3339). Defaults to 30 minutes ago.",
			},
			"end_time": {
				Type:        "string",
				Format:      "date-time",
				Description: "End of metric query window (RFC3339). Defaults to current time.",
			},
			"step": {
				Type:        "string",
				Description: "Resolution step for timeseries points (e.g. '15s', '1m', '5m'). Defaults to '1m'.",
				Default:     "1m",
			},
		},
	}
}

type queryMetricsInput struct {
	Query     string `json:"query"`
	Service   string `json:"service"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	Step      string `json:"step"`
}

func (t *QueryMetricsTool) Execute(ctx context.Context, orgID pgtype.UUID, rawInput json.RawMessage) (ToolResult, error) {
	var in queryMetricsInput
	if len(rawInput) > 0 {
		if err := json.Unmarshal(rawInput, &in); err != nil {
			return ToolResult{
				Success: false,
				Error:   fmt.Sprintf("invalid input parameters: %v", err),
			}, nil
		}
	}

	if in.Query == "" {
		return ToolResult{
			Success: false,
			Error:   "parameter 'query' is required",
		}, nil
	}

	var start, end time.Time
	if in.StartTime != "" {
		if parsed, err := time.Parse(time.RFC3339, in.StartTime); err == nil {
			start = parsed
		}
	}
	if in.EndTime != "" {
		if parsed, err := time.Parse(time.RFC3339, in.EndTime); err == nil {
			end = parsed
		}
	}

	step := time.Minute
	if in.Step != "" {
		if d, err := time.ParseDuration(in.Step); err == nil && d > 0 {
			step = d
		}
	}

	seriesList, err := t.provider.QueryMetrics(ctx, orgID, MetricQuery{
		Query:     in.Query,
		Service:   in.Service,
		StartTime: start,
		EndTime:   end,
		Step:      step,
	})
	if err != nil {
		return ToolResult{
			Success: false,
			Error:   fmt.Sprintf("metric query execution failed: %v", err),
		}, nil
	}

	return ToolResult{
		Success: true,
		Data: map[string]any{
			"query":        in.Query,
			"service":      in.Service,
			"series_count": len(seriesList),
			"series":       seriesList,
		},
		Metadata: map[string]any{
			"series_count": len(seriesList),
		},
	}, nil
}
