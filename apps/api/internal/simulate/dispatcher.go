package simulate

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/seraph/triage/apps/api/internal/eval"
)

// Dispatcher executes or outputs the synthetic fault injection events.
type Dispatcher struct {
	httpClient *http.Client
}

func NewDispatcher(client *http.Client) *Dispatcher {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &Dispatcher{httpClient: client}
}

// Run executes the simulation config against the specified scenario.
func (d *Dispatcher) Run(ctx context.Context, cfg SimulationConfig, sc *eval.Scenario) (*SimulationResult, error) {
	start := time.Now()
	events := GenerateSimulationEvents(sc, cfg.ChaosLevel, cfg.Seed)

	res := &SimulationResult{
		ScenarioID:   sc.ID,
		ScenarioName: sc.Name,
		ChaosLevel:   cfg.ChaosLevel,
		EventsCount:  len(events),
		Timeline:     events,
	}

	if cfg.DryRun || cfg.TargetURL == "" {
		res.OutputLogs = append(res.OutputLogs, fmt.Sprintf("DRY-RUN: Generated %d synthetic fault events for scenario %q (chaos=%d)", len(events), sc.ID, cfg.ChaosLevel))
		for _, e := range events {
			tag := "[NOMINAL]"
			if e.IsNoise {
				tag = "[NOISE]"
			} else if e.IsAdversarial {
				tag = "[ADVERSARIAL]"
			}
			res.OutputLogs = append(res.OutputLogs, fmt.Sprintf("%s %s %-6s [%-7s] %s",
				e.Timestamp.Format("15:04:05"), tag, e.Level, e.Type, e.Summary))
		}
		res.Duration = time.Since(start)
		return res, nil
	}

	// Live Webhook Ingestion Mode
	targetURL := strings.TrimSuffix(cfg.TargetURL, "/")
	endpoint := targetURL + "/api/v1/ingest/alerts/generic"

	var alertEvent *SimulatedEvent
	for i := range events {
		if events[i].Type == "alert" {
			alertEvent = &events[i]
			break
		}
	}

	if alertEvent == nil {
		return nil, fmt.Errorf("scenario %s has no alert event to dispatch", sc.ID)
	}

	payloadBytes, err := json.Marshal(alertEvent.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal alert payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to construct HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "triage-fault-simulator/1.0")

	// HMAC signature if configured
	if cfg.WebhookSecret != "" {
		mac := hmac.New(sha256.New, []byte(cfg.WebhookSecret))
		mac.Write(payloadBytes)
		sig := hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-Signature-256", sig)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to dispatch alert to %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	res.ResponseCode = resp.StatusCode
	res.AlertDispatched = resp.StatusCode >= 200 && resp.StatusCode < 300

	bodyBytes, _ := io.ReadAll(resp.Body)
	var respObj struct {
		IncidentID string `json:"incident_id"`
		Status     string `json:"status"`
	}
	_ = json.Unmarshal(bodyBytes, &respObj)
	res.IncidentID = respObj.IncidentID

	res.OutputLogs = append(res.OutputLogs, fmt.Sprintf("DISPATCHED alert to %s (HTTP %d, incident_id=%s)",
		endpoint, resp.StatusCode, res.IncidentID))
	res.Duration = time.Since(start)

	return res, nil
}
