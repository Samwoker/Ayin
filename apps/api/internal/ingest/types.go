package ingest

import (
	"context"
	"errors"
	"time"
)

var (
	ErrUnknownSource       = errors.New("unsupported or unknown webhook source")
	ErrEmptyPayload        = errors.New("empty webhook payload")
	ErrMalformedPayload    = errors.New("malformed or unparseable webhook payload")
	ErrMissingOrg          = errors.New("missing organization identifier in request")
	ErrOrgNotFound         = errors.New("organization not found")
	ErrMissingSignature    = errors.New("missing webhook signature header")
	ErrInvalidSignature    = errors.New("invalid HMAC webhook signature")
	ErrTimestampExpired    = errors.New("webhook timestamp expired outside acceptable replay window")
	ErrTimestampInvalid    = errors.New("invalid webhook timestamp format")
)

// NormalizedAlert represents a standardized alert extracted from any provider.
type NormalizedAlert struct {
	Fingerprint string            `json:"fingerprint"`
	Source      string            `json:"source"` // alertmanager, grafana, pagerduty, generic
	Title       string            `json:"title"`
	Service     string            `json:"service"`
	Severity    string            `json:"severity"` // sev1, sev2, sev3, sev4, sev5
	Status      string            `json:"status"`   // firing, resolved
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	RawPayload  []byte            `json:"raw_payload"`
	StartsAt    time.Time         `json:"starts_at"`
	EndsAt      *time.Time        `json:"ends_at,omitempty"`
}

// Parser defines the contract for parsing vendor-specific alert payloads.
type Parser interface {
	Name() string
	Parse(ctx context.Context, body []byte) ([]NormalizedAlert, error)
}
