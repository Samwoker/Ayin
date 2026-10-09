package ingest

import (
	"context"
	"time"
)

// LogEntry represents a single log line returned by a log adapter.
type LogEntry struct {
	Timestamp time.Time         `json:"timestamp"`
	Service   string            `json:"service"`
	Level     string            `json:"level"`
	Message   string            `json:"message"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// LogQuery defines the parameters for querying logs across adapters.
type LogQuery struct {
	Service   string    `json:"service"`
	Pattern   string    `json:"pattern"` // Regex or substring
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Limit     int       `json:"limit"`
}

// LogAdapter is the contract implemented by log storage providers (File, Loki, etc.).
type LogAdapter interface {
	Name() string
	QueryLogs(ctx context.Context, query LogQuery) ([]LogEntry, error)
}
