package simulate

import (
	"fmt"
	"math/rand"
	"sort"
	"time"

	"github.com/seraph/triage/apps/api/internal/eval"
)

// GenerateSimulationEvents produces an ordered sequence of simulated events for a scenario.
func GenerateSimulationEvents(sc *eval.Scenario, chaos ChaosLevel, seed int64) []SimulatedEvent {
	r := rand.New(rand.NewSource(seed))
	baseTime := time.Now().UTC().Add(-15 * time.Minute)

	var events []SimulatedEvent

	// 1. Deployment Event (if present in environment)
	if len(sc.Environment.Deploys) > 0 {
		dep := sc.Environment.Deploys[0]
		events = append(events, SimulatedEvent{
			Timestamp: baseTime.Add(2 * time.Minute),
			Type:      "deploy",
			Service:   sc.Service,
			Level:     "INFO",
			Summary:   fmt.Sprintf("Service %s deployed version %s (%s)", sc.Service, dep.Version, dep.Commit),
			Payload:   dep,
		})
	}

	// 2. Metric Spike Event
	for metricName, metricData := range sc.Environment.Metrics {
		if len(metricData.Values) > 0 {
			lastVal := metricData.Values[len(metricData.Values)-1]
			events = append(events, SimulatedEvent{
				Timestamp: baseTime.Add(6 * time.Minute),
				Type:      "metric",
				Service:   sc.Service,
				Level:     "WARN",
				Summary:   fmt.Sprintf("Metric %s surged to %.1f %s", metricName, lastVal.Value, metricData.Unit),
				Payload: map[string]any{
					"metric": metricName,
					"value":  lastVal.Value,
					"unit":   metricData.Unit,
				},
			})
			break
		}
	}

	// 3. Error Log Bursts
	for i, log := range sc.Environment.Logs {
		events = append(events, SimulatedEvent{
			Timestamp: baseTime.Add(time.Duration(8+i) * time.Minute),
			Type:      "log",
			Service:   log.Service,
			Level:     log.Level,
			Summary:   log.Message,
			Payload:   log,
		})
	}

	// 4. Primary Firing Alert Event
	events = append(events, SimulatedEvent{
		Timestamp: baseTime.Add(12 * time.Minute),
		Type:      "alert",
		Service:   sc.Service,
		Level:     "CRITICAL",
		Summary:   fmt.Sprintf("ALERT [%s]: %s", sc.Severity, sc.Alert.Title),
		Payload: map[string]any{
			"title":       sc.Alert.Title,
			"description": sc.Alert.Description,
			"fingerprint": sc.Alert.Fingerprint,
			"service":     sc.Service,
			"severity":    sc.Severity,
			"labels":      sc.Alert.Labels,
			"annotations": sc.Alert.Annotations,
			"status":      "firing",
		},
	})

	// Chaos Level 1: Background Noise
	if chaos >= ChaosLevelNoise {
		noiseLogs := []string{
			"Healthcheck probe /healthz responded 200 OK (latency=4ms)",
			"Go runtime GC pause cycle completed in 0.8ms",
			"Prometheus metrics scraped by agent-collector: 420 metrics exported",
			"Redis connection pool idle reap sweep: 0 connections closed",
		}
		for i, n := range noiseLogs {
			jitterSec := r.Intn(120)
			events = append(events, SimulatedEvent{
				Timestamp: baseTime.Add(time.Duration(3+i*2)*time.Minute + time.Duration(jitterSec)*time.Second),
				Type:      "log",
				Service:   sc.Service,
				Level:     "INFO",
				Summary:   n,
				Payload: map[string]string{
					"service": sc.Service,
					"message": n,
					"level":   "INFO",
				},
				IsNoise: true,
			})
		}
	}

	// Chaos Level 2: Jitter & Transient Retries
	if chaos >= ChaosLevelJitter {
		events = append(events, SimulatedEvent{
			Timestamp: baseTime.Add(7 * time.Minute),
			Type:      "log",
			Service:   sc.Service,
			Level:     "WARN",
			Summary:   "Transient socket timeout; retrying upstream RPC (attempt 1/3) after 150ms backoff",
			Payload: map[string]string{
				"service": sc.Service,
				"message": "Transient socket timeout; retrying upstream RPC (attempt 1/3) after 150ms backoff",
			},
			IsNoise: true,
		})
		events = append(events, SimulatedEvent{
			Timestamp: baseTime.Add(9 * time.Minute),
			Type:      "metric",
			Service:   sc.Service,
			Level:     "INFO",
			Summary:   "TCP retransmit percentage experienced transient spike to 1.8%",
			Payload: map[string]any{
				"metric": "tcp_retransmits_pct",
				"value":  1.8,
			},
			IsNoise: true,
		})
	}

	// Chaos Level 3: Adversarial Distractors & Red Herrings
	if chaos >= ChaosLevelAdversarial {
		// Benign deploy on an unrelated service
		events = append(events, SimulatedEvent{
			Timestamp: baseTime.Add(4 * time.Minute),
			Type:      "deploy",
			Service:   "notification-service",
			Level:     "INFO",
			Summary:   "Unrelated deploy: notification-service released v1.0.4 (routine email template update)",
			Payload: map[string]any{
				"service": "notification-service",
				"version": "v1.0.4",
				"commit":  "77ba91c",
				"message": "routine email template update",
			},
			IsAdversarial: true,
		})

		// Misleading warning log on an adjacent service
		events = append(events, SimulatedEvent{
			Timestamp: baseTime.Add(10 * time.Minute),
			Type:      "log",
			Service:   "auth-gateway",
			Level:     "WARN",
			Summary:   "Unrelated warning: client TLS handshake timeout from non-routable crawler IP 192.0.2.1",
			Payload: map[string]string{
				"service": "auth-gateway",
				"message": "client TLS handshake timeout from non-routable crawler IP 192.0.2.1",
			},
			IsAdversarial: true,
		})
	}

	// Sort events chronologically
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.Before(events[j].Timestamp)
	})

	return events
}
