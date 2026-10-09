package ingest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type GenericParser struct{}

func NewGenericParser() *GenericParser {
	return &GenericParser{}
}

func (p *GenericParser) Name() string {
	return "generic"
}

type genericAlertItem struct {
	Title       string            `json:"title"`
	Service     string            `json:"service"`
	Severity    string            `json:"severity"`
	Status      string            `json:"status"` // firing, resolved
	Fingerprint string            `json:"fingerprint"`
	Message     string            `json:"message"`
	Description string            `json:"description"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	Timestamp   string            `json:"timestamp"`
}

func (p *GenericParser) Parse(ctx context.Context, body []byte) ([]NormalizedAlert, error) {
	if len(body) == 0 {
		return nil, ErrEmptyPayload
	}

	trimmed := strings.TrimSpace(string(body))
	var items []genericAlertItem

	if strings.HasPrefix(trimmed, "[") {
		// JSON Array of alerts
		if err := json.Unmarshal(body, &items); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrMalformedPayload, err)
		}
	} else if strings.HasPrefix(trimmed, "{") {
		// Single JSON alert or wrapped object
		var single genericAlertItem
		if err := json.Unmarshal(body, &single); err == nil && single.Title != "" {
			items = append(items, single)
		} else {
			// Try checking if wrapped under "alerts" key
			var wrapped struct {
				Alerts []genericAlertItem `json:"alerts"`
			}
			if errWrap := json.Unmarshal(body, &wrapped); errWrap == nil && len(wrapped.Alerts) > 0 {
				items = wrapped.Alerts
			} else if single.Service != "" || single.Message != "" {
				items = append(items, single)
			} else {
				return nil, fmt.Errorf("%w: unable to extract alert fields", ErrMalformedPayload)
			}
		}
	} else {
		return nil, ErrMalformedPayload
	}

	var results []NormalizedAlert
	for _, item := range items {
		title := item.Title
		if title == "" {
			title = item.Message
		}
		if title == "" {
			title = "Incoming Generic Alert"
		}

		service := item.Service
		if service == "" && item.Labels != nil {
			service = extractService(item.Labels)
		}
		if service == "" {
			service = "default-service"
		}

		severity := extractSeverity(map[string]string{"severity": item.Severity})

		status := strings.ToLower(item.Status)
		if status == "" || (status != "firing" && status != "resolved") {
			status = "firing"
		}

		labels := item.Labels
		if labels == nil {
			labels = make(map[string]string)
		}
		labels["service"] = service

		annotations := item.Annotations
		if annotations == nil {
			annotations = make(map[string]string)
		}
		if item.Description != "" {
			annotations["description"] = item.Description
		}
		if item.Message != "" {
			annotations["message"] = item.Message
		}

		fingerprint := item.Fingerprint
		if fingerprint == "" {
			h := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s", service, title, severity)))
			fingerprint = hex.EncodeToString(h[:16])
		}

		startsAt := time.Now().UTC()
		if item.Timestamp != "" {
			if t, err := time.Parse(time.RFC3339, item.Timestamp); err == nil {
				startsAt = t
			}
		}

		raw, _ := json.Marshal(item)

		results = append(results, NormalizedAlert{
			Fingerprint: fingerprint,
			Source:      "generic",
			Title:       title,
			Service:     service,
			Severity:    severity,
			Status:      status,
			Labels:      labels,
			Annotations: annotations,
			RawPayload:  raw,
			StartsAt:    startsAt,
		})
	}

	return results, nil
}
