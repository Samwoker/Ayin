package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/seraph/triage/apps/api/internal/store"
	"github.com/seraph/triage/apps/api/internal/store/db"
)

// DefaultDedupeWindow is the default time window to group repeated alerts into the same incident.
const DefaultDedupeWindow = 30 * time.Minute

// AlertGrouper handles alert deduplication, grouping, and incident creation.
type AlertGrouper struct {
	store        *store.Store
	queue        Queue
	dedupeWindow time.Duration
}

// NewAlertGrouper creates an AlertGrouper.
func NewAlertGrouper(st *store.Store, queue Queue, dedupeWindow time.Duration) *AlertGrouper {
	if dedupeWindow <= 0 {
		dedupeWindow = DefaultDedupeWindow
	}
	return &AlertGrouper{
		store:        st,
		queue:        queue,
		dedupeWindow: dedupeWindow,
	}
}

// ProcessAlert processes a normalized alert: deduplicates or groups it into an existing incident,
// or opens a new incident and enqueues an investigation job.
// Returns the associated incident, a boolean indicating if a new incident was opened, and any error.
func (g *AlertGrouper) ProcessAlert(ctx context.Context, orgID pgtype.UUID, alert NormalizedAlert) (*db.Incident, bool, error) {
	if g.store == nil {
		// Mock/test fallback without database
		return &db.Incident{
			ID:       orgID,
			OrgID:    orgID,
			Title:    alert.Title,
			Service:  alert.Service,
			Severity: alert.Severity,
			Status:   "investigating",
		}, true, nil
	}

	cutoffTime := pgtype.Timestamptz{
		Time:  time.Now().Add(-g.dedupeWindow).UTC(),
		Valid: true,
	}

	var targetIncident db.Incident
	var isNewIncident bool
	var createdAlert db.Alert

	err := g.store.WithOrgTx(ctx, orgID, func(q *db.Queries) error {
		// 1. Check for recent alert with identical fingerprint
		existingAlert, err := q.GetAlertByFingerprint(ctx, db.GetAlertByFingerprintParams{
			OrgID:       orgID,
			Fingerprint: alert.Fingerprint,
			ReceivedAt:  cutoffTime,
		})

		if err == nil && existingAlert.IncidentID.Valid {
			// Check if linked incident is still open or investigating
			inc, errInc := q.GetIncidentByID(ctx, db.GetIncidentByIDParams{
				OrgID: orgID,
				ID:    existingAlert.IncidentID,
			})
			if errInc == nil && (inc.Status == "open" || inc.Status == "investigating") {
				targetIncident = inc
				isNewIncident = false
			}
		}

		// 2. If no fingerprint match, check for open incident on the same service within window
		if !targetIncident.ID.Valid {
			openInc, errService := q.GetOpenIncidentForService(ctx, db.GetOpenIncidentForServiceParams{
				OrgID:     orgID,
				Service:   alert.Service,
				OpenedAt:  cutoffTime,
			})
			if errService == nil && openInc.ID.Valid {
				targetIncident = openInc
				isNewIncident = false
			}
		}

		// 3. If still no active incident found, open a brand new incident
		if !targetIncident.ID.Valid {
			severity := alert.Severity
			if severity == "" {
				severity = "sev2"
			}
			newInc, errCreate := q.CreateIncident(ctx, db.CreateIncidentParams{
				OrgID:    orgID,
				Title:    alert.Title,
				Service:  alert.Service,
				Severity: severity,
				Status:   "investigating",
			})
			if errCreate != nil {
				return fmt.Errorf("failed to create incident: %w", errCreate)
			}
			targetIncident = newInc
			isNewIncident = true

			// Log incident created event
			eventPayload, _ := json.Marshal(map[string]any{
				"action":      "incident_opened",
				"source":      alert.Source,
				"fingerprint": alert.Fingerprint,
				"severity":    severity,
			})
			_, _ = q.CreateIncidentEvent(ctx, db.CreateIncidentEventParams{
				OrgID:      orgID,
				IncidentID: targetIncident.ID,
				EventType:  "created",
				Payload:    eventPayload,
			})
		} else {
			// Log alert grouped into existing incident event
			eventPayload, _ := json.Marshal(map[string]any{
				"action":      "alert_grouped",
				"source":      alert.Source,
				"fingerprint": alert.Fingerprint,
				"alert_title": alert.Title,
			})
			_, _ = q.CreateIncidentEvent(ctx, db.CreateIncidentEventParams{
				OrgID:      orgID,
				IncidentID: targetIncident.ID,
				EventType:  "alert_grouped",
				Payload:    eventPayload,
			})
		}

		// 4. Save alert record linked to target incident
		var rawPayload []byte = alert.RawPayload
		if len(rawPayload) == 0 {
			rawPayload = []byte("{}")
		}

		createdAlert, err = q.CreateAlert(ctx, db.CreateAlertParams{
			OrgID:       orgID,
			IncidentID:  targetIncident.ID,
			Fingerprint: alert.Fingerprint,
			Source:      alert.Source,
			Title:       alert.Title,
			RawPayload:  rawPayload,
			Status:      alert.Status,
		})
		if err != nil {
			return fmt.Errorf("failed to record alert: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, false, err
	}

	// 5. If a new incident was opened, enqueue an investigation job for the background worker
	if isNewIncident && g.queue != nil {
		job := InvestigationJob{
			OrgID:      orgID,
			IncidentID: targetIncident.ID,
			AlertID:    createdAlert.ID,
			Title:      targetIncident.Title,
			Service:    targetIncident.Service,
			Severity:   targetIncident.Severity,
			Source:     alert.Source,
			EnqueuedAt: time.Now().UTC(),
		}
		_ = g.queue.Enqueue(ctx, job)
	}

	return &targetIncident, isNewIncident, nil
}
