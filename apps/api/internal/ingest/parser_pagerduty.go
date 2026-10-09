package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type PagerDutyParser struct{}

func NewPagerDutyParser() *PagerDutyParser {
	return &PagerDutyParser{}
}

func (p *PagerDutyParser) Name() string {
	return "pagerduty"
}

type pagerDutyV2Payload struct {
	Event struct {
		ID           string    `json:"id"`
		EventType    string    `json:"event_type"` // incident.triggered, incident.resolved
		ResourceType string    `json:"resource_type"`
		OccurredAt   time.Time `json:"occurred_at"`
		Data         struct {
			ID      string `json:"id"`
			Title   string `json:"title"`
			Urgency string `json:"urgency"` // high, low
			Status  string `json:"status"`  // triggered, acknowledged, resolved
			Service struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"service"`
			HTMLURL string `json:"html_url"`
		} `json:"data"`
	} `json:"event"`

	// Webhook v3 format fallback
	Messages []struct {
		Event     string    `json:"event"`
		LogEntry  struct {
			CreatedAt time.Time `json:"created_at"`
			Incident  struct {
				ID      string `json:"id"`
				Title   string `json:"title"`
				Urgency string `json:"urgency"`
				Status  string `json:"status"`
				Service struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"service"`
			} `json:"incident"`
		} `json:"log_entry"`
	} `json:"messages"`
}

func (p *PagerDutyParser) Parse(ctx context.Context, body []byte) ([]NormalizedAlert, error) {
	if len(body) == 0 {
		return nil, ErrEmptyPayload
	}

	var payload pagerDutyV2Payload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformedPayload, err)
	}

	var results []NormalizedAlert

	// 1. Process v2 format (event)
	if payload.Event.Data.ID != "" {
		data := payload.Event.Data
		eventType := payload.Event.EventType

		status := "firing"
		if strings.Contains(eventType, "resolved") || data.Status == "resolved" {
			status = "resolved"
		}

		severity := "sev2"
		if strings.ToLower(data.Urgency) == "high" {
			severity = "sev1"
		} else if strings.ToLower(data.Urgency) == "low" {
			severity = "sev3"
		}

		service := data.Service.Name
		if service == "" {
			service = "pagerduty-service"
		}

		title := data.Title
		if title == "" {
			title = fmt.Sprintf("PagerDuty Incident %s", data.ID)
		}

		occurredAt := payload.Event.OccurredAt
		if occurredAt.IsZero() {
			occurredAt = time.Now().UTC()
		}

		labels := map[string]string{
			"pagerduty_id":      data.ID,
			"service":           service,
			"urgency":           data.Urgency,
			"pagerduty_service": data.Service.ID,
		}

		annotations := map[string]string{
			"url": data.HTMLURL,
		}

		results = append(results, NormalizedAlert{
			Fingerprint: fmt.Sprintf("pd-%s", data.ID),
			Source:      "pagerduty",
			Title:       title,
			Service:     service,
			Severity:    severity,
			Status:      status,
			Labels:      labels,
			Annotations: annotations,
			RawPayload:  body,
			StartsAt:    occurredAt,
		})
		return results, nil
	}

	// 2. Process v3 format (messages array)
	if len(payload.Messages) > 0 {
		for _, msg := range payload.Messages {
			inc := msg.LogEntry.Incident
			if inc.ID == "" {
				continue
			}

			status := "firing"
			if strings.Contains(msg.Event, "resolved") || inc.Status == "resolved" {
				status = "resolved"
			}

			severity := "sev2"
			if strings.ToLower(inc.Urgency) == "high" {
				severity = "sev1"
			}

			service := inc.Service.Name
			if service == "" {
				service = "pagerduty-service"
			}

			createdAt := msg.LogEntry.CreatedAt
			if createdAt.IsZero() {
				createdAt = time.Now().UTC()
			}

			results = append(results, NormalizedAlert{
				Fingerprint: fmt.Sprintf("pd-%s", inc.ID),
				Source:      "pagerduty",
				Title:       inc.Title,
				Service:     service,
				Severity:    severity,
				Status:      status,
				Labels: map[string]string{
					"pagerduty_id": inc.ID,
					"service":      service,
				},
				RawPayload: body,
				StartsAt:   createdAt,
			})
		}
		if len(results) > 0 {
			return results, nil
		}
	}

	return nil, fmt.Errorf("%w: unrecognized PagerDuty payload structure", ErrMalformedPayload)
}
