package ingest

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/seraph/triage/apps/api/internal/server/middleware"
	"github.com/seraph/triage/apps/api/internal/store"
	"github.com/seraph/triage/apps/api/internal/store/db"
)

// IngestHandler processes incoming webhooks from monitoring and alerting providers.
type IngestHandler struct {
	store    *store.Store
	registry *ParserRegistry
	grouper  *AlertGrouper
}

// NewIngestHandler creates an IngestHandler.
func NewIngestHandler(st *store.Store, registry *ParserRegistry, grouper *AlertGrouper) *IngestHandler {
	if registry == nil {
		registry = NewDefaultParserRegistry()
	}
	return &IngestHandler{
		store:    st,
		registry: registry,
		grouper:  grouper,
	}
}

// HandleWebhook handles webhook ingestion for a given source.
func (h *IngestHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	source := chi.URLParam(r, "source")
	if source == "" {
		respondError(w, http.StatusBadRequest, "Missing webhook source in URL path")
		return
	}

	parser, err := h.registry.Get(source)
	if err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("Unsupported webhook source: %s", source))
		return
	}

	// 1. Resolve Target Organization
	org, err := h.resolveOrganization(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// 2. Read Request Body
	body, err := io.ReadAll(io.LimitReader(r.Body, 5*1024*1024)) // 5MB limit
	if err != nil || len(body) == 0 {
		respondError(w, http.StatusBadRequest, "Empty or unreadable request body")
		return
	}

	// 3. Verify HMAC Signature
	sigHeader := extractSignatureHeader(r)
	tsHeader := extractTimestampHeader(r)

	// In test mode or when webhook_secret is configured, verify HMAC
	if org.WebhookSecret != "" {
		if err := VerifyWebhookSignature(org.WebhookSecret, body, sigHeader, tsHeader, DefaultReplayWindow); err != nil {
			respondError(w, http.StatusUnauthorized, fmt.Sprintf("Webhook authentication failed: %v", err))
			return
		}
	}

	// 4. Parse Alerts
	alerts, err := parser.Parse(r.Context(), body)
	if err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("Failed to parse alerts: %v", err))
		return
	}

	if len(alerts) == 0 {
		respondJSON(w, http.StatusOK, map[string]any{
			"status":           "ignored",
			"message":          "No actionable alerts found in payload",
			"alerts_processed": 0,
		})
		return
	}

	// 5. Deduplicate, Group & Enqueue Investigations
	var createdCount, groupedCount int
	var incidentIDs []string

	for _, alert := range alerts {
		incident, isNew, err := h.grouper.ProcessAlert(r.Context(), org.ID, alert)
		if err != nil {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to process alert: %v", err))
			return
		}

		incIDStr := uuidToString(incident.ID)
		incidentIDs = append(incidentIDs, incIDStr)

		if isNew {
			createdCount++
		} else {
			groupedCount++
		}
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"status":            "success",
		"source":            source,
		"alerts_processed":  len(alerts),
		"incidents_created": createdCount,
		"incidents_grouped": groupedCount,
		"incident_ids":      incidentIDs,
	})
}

func (h *IngestHandler) resolveOrganization(r *http.Request) (db.Org, error) {
	// A. Check if caller already has authenticated TenantContext
	if tc, ok := middleware.GetTenant(r.Context()); ok {
		if h.store != nil {
			return h.store.Queries().GetOrgByID(r.Context(), tc.OrgID)
		}
		return db.Org{ID: tc.OrgID, WebhookSecret: "dev_secret"}, nil
	}

	// B. Check URL path / query params
	orgIDStr := chi.URLParam(r, "org_id")
	if orgIDStr == "" {
		orgIDStr = r.URL.Query().Get("org_id")
	}
	if orgIDStr == "" {
		orgIDStr = r.Header.Get("X-Org-ID")
	}

	if orgIDStr != "" {
		var orgUUID pgtype.UUID
		if err := orgUUID.Scan(orgIDStr); err == nil {
			if h.store != nil {
				return h.store.Queries().GetOrgByID(r.Context(), orgUUID)
			}
			return db.Org{ID: orgUUID, WebhookSecret: "dev_secret"}, nil
		}
	}

	// C. Check Org Slug
	orgSlug := chi.URLParam(r, "org_slug")
	if orgSlug == "" {
		orgSlug = r.URL.Query().Get("org_slug")
	}
	if orgSlug == "" {
		orgSlug = r.Header.Get("X-Org-Slug")
	}

	if orgSlug != "" && h.store != nil {
		return h.store.Queries().GetOrgBySlug(r.Context(), orgSlug)
	}

	// D. Fallback default org for local dev/testing if only 1 exists
	if h.store != nil {
		orgs, err := h.store.Queries().ListOrgs(r.Context())
		if err == nil && len(orgs) > 0 {
			return orgs[0], nil
		}
	}

	return db.Org{}, ErrMissingOrg
}

func extractSignatureHeader(r *http.Request) string {
	for _, h := range []string{
		"X-Signature-256",
		"X-Hub-Signature-256",
		"X-Signature",
		"X-PagerDuty-Signature",
		"X-Grafana-Signature",
	} {
		if val := r.Header.Get(h); val != "" {
			return val
		}
	}
	return ""
}

func extractTimestampHeader(r *http.Request) string {
	for _, h := range []string{
		"X-Timestamp",
		"X-Webhook-Timestamp",
		"X-Request-Timestamp",
	} {
		if val := r.Header.Get(h); val != "" {
			return val
		}
	}
	return ""
}

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func uuidToString(u pgtype.UUID) string {
	return fmt.Sprintf("%x-%x-%x-%x-%x", u.Bytes[0:4], u.Bytes[4:6], u.Bytes[6:8], u.Bytes[8:10], u.Bytes[10:16])
}
