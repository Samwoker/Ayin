package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/seraph/triage/apps/api/internal/store/db"
)

func TestLiveTenantIsolationWithRLS(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://triage:triage_secret_dev@localhost:5433/triage?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s, err := NewStore(ctx, dbURL)
	if err != nil {
		t.Skipf("skipping live database test: postgres not reachable (%v)", err)
		return
	}
	defer s.Close()

	// Parse UUIDs for Acme (Org 1) and Globex (Org 2)
	var acmeOrgID, globexOrgID pgtype.UUID
	if err := acmeOrgID.Scan("00000000-0000-0000-0000-000000000001"); err != nil {
		t.Fatalf("failed to parse acme UUID: %v", err)
	}
	if err := globexOrgID.Scan("00000000-0000-0000-0000-000000000002"); err != nil {
		t.Fatalf("failed to parse globex UUID: %v", err)
	}

	// 1. Query incidents as Acme Corp
	var acmeIncidents []db.Incident
	err = s.WithOrgTx(ctx, acmeOrgID, func(q *db.Queries) error {
		var err error
		acmeIncidents, err = q.ListIncidents(ctx, db.ListIncidentsParams{
			OrgID:  acmeOrgID,
			Limit:  10,
			Offset: 0,
		})
		return err
	})
	if err != nil {
		t.Fatalf("failed to query Acme incidents: %v", err)
	}

	if len(acmeIncidents) == 0 {
		t.Fatalf("expected Acme incidents from seed data, got 0")
	}

	// 2. Query incidents as Globex Logistics (should not see Acme incidents)
	var globexIncidents []db.Incident
	err = s.WithOrgTx(ctx, globexOrgID, func(q *db.Queries) error {
		var err error
		globexIncidents, err = q.ListIncidents(ctx, db.ListIncidentsParams{
			OrgID:  globexOrgID,
			Limit:  10,
			Offset: 0,
		})
		return err
	})
	if err != nil {
		t.Fatalf("failed to query Globex incidents: %v", err)
	}

	if len(globexIncidents) != 0 {
		t.Fatalf("expected Globex to have 0 incidents, but saw %d (leak!)", len(globexIncidents))
	}

	// 3. Negative cross-tenant attempt: try to query Acme incident ID with Globex OrgID
	err = s.WithOrgTx(ctx, globexOrgID, func(q *db.Queries) error {
		_, err := q.GetIncidentByID(ctx, db.GetIncidentByIDParams{
			OrgID: globexOrgID,
			ID:    acmeIncidents[0].ID,
		})
		return err
	})
	if err == nil {
		t.Fatalf("expected error when querying Acme incident using Globex credentials, but got nil")
	}
}
