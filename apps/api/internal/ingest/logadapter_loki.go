package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// LokiLogAdapter implements LogAdapter by querying the Grafana Loki HTTP API.
type LokiLogAdapter struct {
	baseURL    string
	httpClient *http.Client
	tenantID   string
}

// NewLokiLogAdapter creates a LokiLogAdapter instance.
func NewLokiLogAdapter(baseURL string, httpClient *http.Client, tenantID string) *LokiLogAdapter {
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 10 * time.Second,
		}
	}
	return &LokiLogAdapter{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		httpClient: httpClient,
		tenantID:   tenantID,
	}
}

func (a *LokiLogAdapter) Name() string {
	return "loki"
}

type lokiResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Stream map[string]string `json:"stream"`
			Values [][]string        `json:"values"` // [timestamp_ns, log_line]
		} `json:"result"`
	} `json:"data"`
}

func (a *LokiLogAdapter) QueryLogs(ctx context.Context, query LogQuery) ([]LogEntry, error) {
	logQL := a.buildLogQL(query)

	endpoint := fmt.Sprintf("%s/loki/api/v1/query_range", a.baseURL)
	params := url.Values{}
	params.Set("query", logQL)
	params.Set("direction", "BACKWARD")

	limit := query.Limit
	if limit <= 0 {
		limit = 100
	}
	params.Set("limit", strconv.Itoa(limit))

	if !query.StartTime.IsZero() {
		params.Set("start", strconv.FormatInt(query.StartTime.UnixNano(), 10))
	}
	if !query.EndTime.IsZero() {
		params.Set("end", strconv.FormatInt(query.EndTime.UnixNano(), 10))
	}

	fullURL := fmt.Sprintf("%s?%s", endpoint, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Loki request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	if a.tenantID != "" {
		req.Header.Set("X-Scope-OrgID", a.tenantID)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("loki request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("loki returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var lokiResp lokiResponse
	if err := json.NewDecoder(resp.Body).Decode(&lokiResp); err != nil {
		return nil, fmt.Errorf("failed to decode loki response: %w", err)
	}

	var entries []LogEntry
	for _, stream := range lokiResp.Data.Result {
		streamService := stream.Stream["service"]
		if streamService == "" {
			streamService = stream.Stream["app"]
		}
		if streamService == "" {
			streamService = query.Service
		}

		for _, val := range stream.Values {
			if len(val) < 2 {
				continue
			}

			// Parse nanosecond timestamp
			ts := time.Now().UTC()
			if ns, err := strconv.ParseInt(val[0], 10, 64); err == nil {
				ts = time.Unix(0, ns).UTC()
			}

			line := val[1]
			level := inferLevel(line)

			entries = append(entries, LogEntry{
				Timestamp: ts,
				Service:   streamService,
				Level:     level,
				Message:   line,
				Labels:    stream.Stream,
			})

			if len(entries) >= limit {
				return entries, nil
			}
		}
	}

	return entries, nil
}

func (a *LokiLogAdapter) buildLogQL(query LogQuery) string {
	pattern := strings.TrimSpace(query.Pattern)

	// If already a valid LogQL expression
	if strings.HasPrefix(pattern, "{") {
		return pattern
	}

	service := query.Service
	if service == "" {
		service = ".*"
	}

	streamSelector := fmt.Sprintf(`{service=~"%s"}`, service)
	if pattern != "" {
		return fmt.Sprintf(`%s |= "%s"`, streamSelector, pattern)
	}

	return streamSelector
}
