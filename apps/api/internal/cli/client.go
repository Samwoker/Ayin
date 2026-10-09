package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client handles HTTP and SSE communication with the triage API server.
type Client struct {
	cfg        Config
	httpClient *http.Client
}

func NewClient(cfg Config, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	if cfg.APIURL == "" {
		cfg.APIURL = "http://localhost:8080"
	}
	cfg.APIURL = strings.TrimSuffix(cfg.APIURL, "/")
	return &Client{
		cfg:        cfg,
		httpClient: httpClient,
	}
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	fullURL := c.cfg.APIURL + path

	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.cfg.APIKey != "" {
		req.Header.Set("X-API-Key", c.cfg.APIKey)
	} else if c.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	}
	if c.cfg.OrgID != "" {
		req.Header.Set("X-Org-ID", c.cfg.OrgID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request to %s failed: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

// ListIncidents retrieves incidents matching filter criteria.
func (c *Client) ListIncidents(ctx context.Context, status, service, severity string, limit int) ([]IncidentDTO, error) {
	params := url.Values{}
	if status != "" {
		params.Set("status", status)
	}
	if service != "" {
		params.Set("service", service)
	}
	if severity != "" {
		params.Set("severity", severity)
	}
	if limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", limit))
	}

	path := "/api/v1/incidents"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	var incidents []IncidentDTO
	if err := c.do(ctx, http.MethodGet, path, nil, &incidents); err != nil {
		return nil, err
	}
	return incidents, nil
}

// GetIncident returns incident details.
func (c *Client) GetIncident(ctx context.Context, id string) (*IncidentDTO, error) {
	var inc IncidentDTO
	if err := c.do(ctx, http.MethodGet, "/api/v1/incidents/"+id, nil, &inc); err != nil {
		return nil, err
	}
	return &inc, nil
}

// CreateIncident creates a new incident.
func (c *Client) CreateIncident(ctx context.Context, title, service, severity, description string) (*IncidentDTO, error) {
	req := map[string]string{
		"title":       title,
		"service":     service,
		"severity":    severity,
		"description": description,
	}
	var inc IncidentDTO
	if err := c.do(ctx, http.MethodPost, "/api/v1/incidents", req, &inc); err != nil {
		return nil, err
	}
	return &inc, nil
}

// ResolveIncident marks an incident as resolved.
func (c *Client) ResolveIncident(ctx context.Context, id string) error {
	req := map[string]string{
		"status": "resolved",
	}
	return c.do(ctx, http.MethodPatch, "/api/v1/incidents/"+id, req, nil)
}

// ListRuns returns all investigation runs for an incident.
func (c *Client) ListRuns(ctx context.Context, incidentID string) ([]RunDTO, error) {
	var runs []RunDTO
	if err := c.do(ctx, http.MethodGet, "/api/v1/incidents/"+incidentID+"/runs", nil, &runs); err != nil {
		return nil, err
	}
	return runs, nil
}

// GetRun returns run details.
func (c *Client) GetRun(ctx context.Context, runID string) (*RunDTO, error) {
	var run RunDTO
	if err := c.do(ctx, http.MethodGet, "/api/v1/runs/"+runID, nil, &run); err != nil {
		return nil, err
	}
	return &run, nil
}

// ListRemediations lists remediations for an incident.
func (c *Client) ListRemediations(ctx context.Context, incidentID string) ([]RemediationDTO, error) {
	var rems []RemediationDTO
	if err := c.do(ctx, http.MethodGet, "/api/v1/incidents/"+incidentID+"/remediations", nil, &rems); err != nil {
		return nil, err
	}
	return rems, nil
}

// GetRemediation returns remediation details.
func (c *Client) GetRemediation(ctx context.Context, id string) (*RemediationDTO, error) {
	var rem RemediationDTO
	if err := c.do(ctx, http.MethodGet, "/api/v1/remediations/"+id, nil, &rem); err != nil {
		return nil, err
	}
	return &rem, nil
}

// ApproveRemediation approves and executes a proposed remediation.
func (c *Client) ApproveRemediation(ctx context.Context, id, reason string) error {
	req := map[string]string{"reason": reason}
	return c.do(ctx, http.MethodPost, "/api/v1/remediations/"+id+"/approve", req, nil)
}

// RejectRemediation rejects a proposed remediation.
func (c *Client) RejectRemediation(ctx context.Context, id, reason string) error {
	req := map[string]string{"reason": reason}
	return c.do(ctx, http.MethodPost, "/api/v1/remediations/"+id+"/reject", req, nil)
}

// RollbackRemediation rolls back an executed remediation.
func (c *Client) RollbackRemediation(ctx context.Context, id, reason string) error {
	req := map[string]string{"reason": reason}
	return c.do(ctx, http.MethodPost, "/api/v1/remediations/"+id+"/rollback", req, nil)
}

// ListTools lists available diagnostic tools.
func (c *Client) ListTools(ctx context.Context) ([]ToolDTO, error) {
	var tools []ToolDTO
	if err := c.do(ctx, http.MethodGet, "/api/v1/tools", nil, &tools); err != nil {
		return nil, err
	}
	return tools, nil
}

// ExecuteTool invokes a diagnostic tool directly.
func (c *Client) ExecuteTool(ctx context.Context, name string, inputJSON []byte) (map[string]any, error) {
	var raw map[string]any
	if len(inputJSON) > 0 {
		_ = json.Unmarshal(inputJSON, &raw)
	}
	var res map[string]any
	if err := c.do(ctx, http.MethodPost, "/api/v1/tools/"+name+"/execute", raw, &res); err != nil {
		return nil, err
	}
	return res, nil
}

// StreamIncident connects to SSE endpoint and calls onEvent for every incoming event.
func (c *Client) StreamIncident(ctx context.Context, incidentID string, onEvent func(eventType, data string)) error {
	streamURL := fmt.Sprintf("%s/api/v1/incidents/%s/stream", c.cfg.APIURL, incidentID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, streamURL, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Accept", "text/event-stream")
	if c.cfg.APIKey != "" {
		req.Header.Set("X-API-Key", c.cfg.APIKey)
	} else if c.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	}
	if c.cfg.OrgID != "" {
		req.Header.Set("X-Org-ID", c.cfg.OrgID)
	}

	// Use unbuffered client for streaming
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to stream incident (status %d): %s", resp.StatusCode, string(body))
	}

	reader := bufio.NewReader(resp.Body)
	var currentEvent, currentData string

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if currentEvent != "" || currentData != "" {
				onEvent(currentEvent, currentData)
				currentEvent = ""
				currentData = ""
			}
			continue
		}

		if strings.HasPrefix(line, "event:") {
			currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			currentData = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
	}
}
