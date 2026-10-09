package store

import (
	"io/fs"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/seraph/triage/apps/api/internal/store/db"
)

func TestEmbeddedMigrations(t *testing.T) {
	entries, err := fs.ReadDir(EmbedMigrations, "migrations")
	if err != nil {
		t.Fatalf("failed to read embedded migrations: %v", err)
	}

	if len(entries) == 0 {
		t.Fatalf("expected embedded migration files, got 0")
	}

	foundInitial := false
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".sql") {
			content, err := fs.ReadFile(EmbedMigrations, "migrations/"+entry.Name())
			if err != nil {
				t.Fatalf("failed to read migration file %s: %v", entry.Name(), err)
			}
			if len(content) == 0 {
				t.Fatalf("migration file %s is empty", entry.Name())
			}
			if strings.Contains(string(content), "CREATE TABLE orgs") {
				foundInitial = true
			}
		}
	}

	if !foundInitial {
		t.Fatalf("expected migration to contain CREATE TABLE orgs")
	}
}

func TestGeneratedModelsTenantAwareness(t *testing.T) {
	// Verify that tenant models have OrgID field
	tenantModelTypes := []any{
		db.Incident{},
		db.Alert{},
		db.Run{},
		db.RunStep{},
		db.Hypothesis{},
		db.Feedback{},
		db.Runbook{},
		db.RunbookChunk{},
		db.Remediation{},
		db.AuditLog{},
		db.UsageLedger{},
	}

	for _, model := range tenantModelTypes {
		val := reflect.ValueOf(model)
		typ := val.Type()
		field, found := typ.FieldByName("OrgID")
		if !found {
			t.Fatalf("model %s missing required OrgID tenant field", typ.Name())
		}
		if field.Type != reflect.TypeOf(pgtype.UUID{}) {
			t.Fatalf("model %s OrgID expected pgtype.UUID, got %s", typ.Name(), field.Type)
		}
	}
}

func TestRLSPolicyCoverageInMigrations(t *testing.T) {
	content, err := fs.ReadFile(EmbedMigrations, "migrations/00001_init_schema.sql")
	if err != nil {
		t.Fatalf("failed to read migration: %v", err)
	}

	sqlStr := string(content)

	// Verify RLS policy creation
	if !strings.Contains(sqlStr, "ENABLE ROW LEVEL SECURITY") {
		t.Fatalf("migration missing ENABLE ROW LEVEL SECURITY")
	}
	if !strings.Contains(sqlStr, "current_setting('app.org_id'") {
		t.Fatalf("migration missing current_setting('app.org_id') in RLS policies")
	}
	if !strings.Contains(sqlStr, "CREATE EXTENSION IF NOT EXISTS \"vector\"") {
		t.Fatalf("migration missing vector extension")
	}
}
