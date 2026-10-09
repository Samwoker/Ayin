package ingest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

type AlertmanagerParser struct{}

func NewAlertmanagerParser() *AlertmanagerParser {
	return &AlertmanagerParser{}
}

func (p *AlertmanagerParser) Name() string {
	return "alertmanager"
}

type alertmanagerPayload struct {
	Version           string                       `json:"version"`
	GroupKey          string                       `json:"groupKey"`
	Status            string                       `json:"status"`
	Receiver          string                       `json:"receiver"`
	GroupLabels       map[string]string            `json:"groupLabels"`
	CommonLabels      map[string]string            `json:"commonLabels"`
	CommonAnnotations map[string]string            `json:"commonAnnotations"`
	Alerts            []alertmanagerAlert          `json:"alerts"`
}

type alertmanagerAlert struct {
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     time.Time         `json:"startsAt"`
	EndsAt       time.Time         `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
	Fingerprint  string            `json:"fingerprint"`
}

func (p *AlertmanagerParser) Parse(ctx context.Context, body []byte) ([]NormalizedAlert, error) {
	if len(body) == 0 {
		return nil, ErrEmptyPayload
	}

	var payload alertmanagerPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformedPayload, err)
	}

	var results []NormalizedAlert
	for _, a := range payload.Alerts {
		labels := make(map[string]string)
		for k, v := range payload.CommonLabels {
			labels[k] = v
		}
		for k, v := range a.Labels {
			labels[k] = v
		}

		annotations := make(map[string]string)
		for k, v := range payload.CommonAnnotations {
			annotations[k] = v
		}
		for k, v := range a.Annotations {
			annotations[k] = v
		}

		service := extractService(labels)
		title := extractTitle(labels, annotations)
		severity := extractSeverity(labels)
		fingerprint := a.Fingerprint
		if fingerprint == "" {
			fingerprint = computeLabelsFingerprint(labels)
		}

		status := a.Status
		if status == "" {
			status = payload.Status
		}
		if status == "" {
			status = "firing"
		}

		var endsAt *time.Time
		if !a.EndsAt.IsZero() && a.EndsAt.After(a.StartsAt) {
			endsAt = &a.EndsAt
		}

		startsAt := a.StartsAt
		if startsAt.IsZero() {
			startsAt = time.Now().UTC()
		}

		rawAlert, _ := json.Marshal(a)

		results = append(results, NormalizedAlert{
			Fingerprint: fingerprint,
			Source:      "alertmanager",
			Title:       title,
			Service:     service,
			Severity:    severity,
			Status:      status,
			Labels:      labels,
			Annotations: annotations,
			RawPayload:  rawAlert,
			StartsAt:    startsAt,
			EndsAt:      endsAt,
		})
	}

	return results, nil
}

func extractService(labels map[string]string) string {
	for _, key := range []string{"service", "app", "job", "component", "service_name"} {
		if val, exists := labels[key]; exists && val != "" {
			return val
		}
	}
	return "unknown-service"
}

func extractTitle(labels, annotations map[string]string) string {
	for _, key := range []string{"alertname", "summary", "title", "name"} {
		if val, exists := annotations[key]; exists && val != "" {
			return val
		}
		if val, exists := labels[key]; exists && val != "" {
			return val
		}
	}
	return "Alertmanager Alert"
}

func extractSeverity(labels map[string]string) string {
	raw := strings.ToLower(labels["severity"])
	switch raw {
	case "critical", "sev1", "p1", "page":
		return "sev1"
	case "high", "sev2", "p2", "error":
		return "sev2"
	case "medium", "sev3", "p3", "warning":
		return "sev3"
	case "low", "sev4", "p4", "info":
		return "sev4"
	case "sev5", "p5":
		return "sev5"
	default:
		return "sev2"
	}
}

func computeLabelsFingerprint(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(k)
		sb.WriteString("=")
		sb.WriteString(labels[k])
		sb.WriteString(";")
	}

	hash := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(hash[:16])
}
