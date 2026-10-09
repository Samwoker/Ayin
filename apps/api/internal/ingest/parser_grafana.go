package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type GrafanaParser struct{}

func NewGrafanaParser() *GrafanaParser {
	return &GrafanaParser{}
}

func (p *GrafanaParser) Name() string {
	return "grafana"
}

type grafanaPayload struct {
	Title       string         `json:"title"`
	State       string         `json:"state"` // alerting, ok, paused
	Message     string         `json:"message"`
	RuleName    string         `json:"ruleName"`
	RuleURL     string         `json:"ruleUrl"`
	Alerts      []grafanaAlert `json:"alerts"`
}

type grafanaAlert struct {
	Status      string            `json:"status"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	StartsAt    time.Time         `json:"startsAt"`
	EndsAt      time.Time         `json:"endsAt"`
	Fingerprint string            `json:"fingerprint"`
}

func (p *GrafanaParser) Parse(ctx context.Context, body []byte) ([]NormalizedAlert, error) {
	if len(body) == 0 {
		return nil, ErrEmptyPayload
	}

	var payload grafanaPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformedPayload, err)
	}

	var results []NormalizedAlert

	// If payload contains multiple sub-alerts
	if len(payload.Alerts) > 0 {
		for _, a := range payload.Alerts {
			labels := a.Labels
			if labels == nil {
				labels = make(map[string]string)
			}
			annotations := a.Annotations
			if annotations == nil {
				annotations = make(map[string]string)
			}

			service := extractService(labels)
			title := a.Annotations["summary"]
			if title == "" {
				title = a.Labels["alertname"]
			}
			if title == "" {
				title = payload.Title
			}
			if title == "" {
				title = "Grafana Alert"
			}

			severity := extractSeverity(labels)
			fingerprint := a.Fingerprint
			if fingerprint == "" {
				fingerprint = computeLabelsFingerprint(labels)
			}

			status := a.Status
			if status == "" {
				if payload.State == "alerting" {
					status = "firing"
				} else if payload.State == "ok" {
					status = "resolved"
				} else {
					status = "firing"
				}
			}

			startsAt := a.StartsAt
			if startsAt.IsZero() {
				startsAt = time.Now().UTC()
			}

			var endsAt *time.Time
			if !a.EndsAt.IsZero() && a.EndsAt.After(startsAt) {
				endsAt = &a.EndsAt
			}

			raw, _ := json.Marshal(a)

			results = append(results, NormalizedAlert{
				Fingerprint: fingerprint,
				Source:      "grafana",
				Title:       title,
				Service:     service,
				Severity:    severity,
				Status:      status,
				Labels:      labels,
				Annotations: annotations,
				RawPayload:  raw,
				StartsAt:    startsAt,
				EndsAt:      endsAt,
			})
		}
	} else {
		// Single unified alert payload without nested alerts array
		title := payload.Title
		if title == "" {
			title = payload.RuleName
		}
		if title == "" {
			title = "Grafana Alert"
		}

		status := "firing"
		if payload.State == "ok" {
			status = "resolved"
		}

		labels := map[string]string{"rule_name": payload.RuleName}
		fingerprint := computeLabelsFingerprint(labels)

		results = append(results, NormalizedAlert{
			Fingerprint: fingerprint,
			Source:      "grafana",
			Title:       title,
			Service:     "grafana-service",
			Severity:    "sev2",
			Status:      status,
			Labels:      labels,
			Annotations: map[string]string{"message": payload.Message, "rule_url": payload.RuleURL},
			RawPayload:  body,
			StartsAt:    time.Now().UTC(),
		})
	}

	return results, nil
}
