package remediation

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/seraph/triage/apps/api/internal/server/sse"
	"github.com/seraph/triage/apps/api/internal/store"
)

func parseUUID(s string) pgtype.UUID {
	var id pgtype.UUID
	_ = id.Scan(s)
	return id
}

func TestRemediationTypesAndValidation(t *testing.T) {
	validActions := []string{
		ActionRestartService,
		ActionScaleService,
		ActionRollbackDeploy,
		ActionClearCache,
		ActionPauseConsumer,
		ActionToggleFeatureFlag,
	}

	for _, a := range validActions {
		if !IsValidAction(a) {
			t.Errorf("expected %s to be valid action", a)
		}
	}

	if IsValidAction("drop_database") {
		t.Errorf("expected drop_database to be invalid action")
	}

	validRisks := []string{RiskLow, RiskMedium, RiskHigh, RiskCritical}
	for _, r := range validRisks {
		if !IsValidRisk(r) {
			t.Errorf("expected %s to be valid risk level", r)
		}
	}

	if IsValidRisk("extreme") {
		t.Errorf("expected extreme to be invalid risk level")
	}
}

func TestDefaultRemediationExecutorAllActions(t *testing.T) {
	exec := NewDefaultRemediationExecutor()
	ctx := context.Background()

	tests := []struct {
		name      string
		action    string
		target    string
		params    map[string]any
		expectKey string
	}{
		{
			name:      "restart_service",
			action:    ActionRestartService,
			target:    "checkout-worker",
			params:    map[string]any{"grace_period_seconds": 15.0},
			expectKey: "restart_mode",
		},
		{
			name:      "scale_service",
			action:    ActionScaleService,
			target:    "payment-api",
			params:    map[string]any{"replicas": 4.0, "previous_replicas": 1.0},
			expectKey: "desired_replicas",
		},
		{
			name:      "rollback_deploy",
			action:    ActionRollbackDeploy,
			target:    "auth-api",
			params:    map[string]any{"target_version": "v1.2.0", "previous_commit": "abc999"},
			expectKey: "rollback_version",
		},
		{
			name:      "clear_cache",
			action:    ActionClearCache,
			target:    "redis-sessions",
			params:    map[string]any{"pattern": "sess:*"},
			expectKey: "keys_evicted",
		},
		{
			name:      "pause_consumer",
			action:    ActionPauseConsumer,
			target:    "rabbitmq-orders",
			params:    map[string]any{"duration_seconds": 120.0},
			expectKey: "resume_at",
		},
		{
			name:      "toggle_feature_flag",
			action:    ActionToggleFeatureFlag,
			target:    "flag_new_checkout_flow",
			params:    map[string]any{"enabled": false},
			expectKey: "new_state",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, err := exec.Execute(ctx, tc.action, tc.target, tc.params)
			if err != nil {
				t.Fatalf("unexpected execute error: %v", err)
			}
			if !res.Success {
				t.Fatalf("expected success, got false")
			}
			if res.Action != tc.action || res.Target != tc.target {
				t.Errorf("mismatched action or target: %v", res)
			}
			if _, exists := res.Details[tc.expectKey]; !exists {
				t.Errorf("expected details key %q not found: %v", tc.expectKey, res.Details)
			}
		})
	}

	// Validation error cases
	t.Run("missing target", func(t *testing.T) {
		_, err := exec.Execute(ctx, ActionRestartService, "", nil)
		if err == nil {
			t.Errorf("expected error for empty target")
		}
	})

	t.Run("invalid action", func(t *testing.T) {
		_, err := exec.Execute(ctx, "unsupported_action", "svc", nil)
		if err == nil {
			t.Errorf("expected error for unsupported action")
		}
	})

	// Rollback cases
	t.Run("rollback scale_service", func(t *testing.T) {
		res, err := exec.Rollback(ctx, ActionScaleService, "payment-api", map[string]any{"previous_replicas": 2.0})
		if err != nil {
			t.Fatalf("rollback error: %v", err)
		}
		if res.Details["reverted_to"] != 2 {
			t.Errorf("expected reverted_to 2, got %v", res.Details["reverted_to"])
		}
	})

	t.Run("rollback toggle_feature_flag", func(t *testing.T) {
		res, err := exec.Rollback(ctx, ActionToggleFeatureFlag, "flag_checkout", map[string]any{"enabled": false})
		if err != nil {
			t.Fatalf("rollback error: %v", err)
		}
		if res.Details["reverted_state"] != true {
			t.Errorf("expected reverted_state true, got %v", res.Details["reverted_state"])
		}
	})

	t.Run("custom handlers", func(t *testing.T) {
		exec.SetCustomHandler(ActionRestartService, func(ctx context.Context, action, target string, params map[string]any) (*ExecutionResult, error) {
			return &ExecutionResult{
				Success: true,
				Action:  action,
				Target:  target,
				Details: map[string]any{"custom": true},
			}, nil
		})
		res, err := exec.Execute(ctx, ActionRestartService, "custom-target", nil)
		if err != nil || res.Details["custom"] != true {
			t.Errorf("failed to trigger custom handler")
		}
	})
}

func TestRemediationServiceLiveDatabase(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://triage:triage_secret_dev@localhost:5433/triage?sslmode=disable"
	}

	ctx := context.Background()
	st, err := store.NewStore(ctx, dbURL)
	if err != nil {
		t.Skipf("skipping live database test: postgres not reachable (%v)", err)
		return
	}
	defer st.Close()

	orgID := parseUUID("00000000-0000-0000-0000-000000000001")
	incidentID := parseUUID("40000000-0000-0000-0000-000000000001")
	userID := parseUUID("10000000-0000-0000-0000-000000000001")

	executor := NewDefaultRemediationExecutor()
	broker := sse.NewBroker()
	svc := NewRemediationService(st, executor, broker, nil)

	// 1. Propose remediation
	propInput := CreateRemediationInput{
		IncidentID:     incidentID,
		ActionType:     ActionRestartService,
		Target:         "checkout-worker",
		Params:         map[string]any{"grace_period_seconds": 30.0},
		Rationale:      "Queue backlog cleared by rolling restart to purge saturated threadpool",
		RiskLevel:      RiskLow,
		ExpectedEffect: "Zero unconsumed RabbitMQ messages within 3 minutes",
		RollbackPlan:   "Traffic redirected to secondary replica pool",
		TTL:            10 * time.Minute,
	}

	rem, err := svc.Propose(ctx, orgID, propInput)
	if err != nil {
		t.Fatalf("failed to propose remediation: %v", err)
	}

	if rem.Status != StatusProposed {
		t.Errorf("expected status proposed, got %s", rem.Status)
	}

	// 2. Get remediation and verify events
	fetched, events, err := svc.Get(ctx, orgID, rem.ID)
	if err != nil {
		t.Fatalf("failed to get remediation: %v", err)
	}
	if fetched.ID != rem.ID {
		t.Errorf("mismatched remediation ID")
	}
	if len(events) == 0 {
		t.Errorf("expected at least 1 remediation event")
	}

	// 3. List remediations for incident
	list, err := svc.ListForIncident(ctx, orgID, incidentID)
	if err != nil {
		t.Fatalf("failed to list remediations: %v", err)
	}
	if len(list) == 0 {
		t.Errorf("expected at least 1 remediation in list")
	}

	// 4. Approve and execute
	approved, execRes, err := svc.Approve(ctx, orgID, rem.ID, userID, "Looks safe, approving restart")
	if err != nil {
		t.Fatalf("failed to approve remediation: %v", err)
	}
	if approved.Status != StatusSucceeded {
		t.Errorf("expected status succeeded, got %s", approved.Status)
	}
	if execRes == nil || !execRes.Success {
		t.Errorf("expected execution result success")
	}

	// 5. Verify events after approval
	_, updatedEvents, err := svc.Get(ctx, orgID, rem.ID)
	if err != nil {
		t.Fatalf("failed to get events: %v", err)
	}
	// Events should have: proposed, approved, executing, succeeded
	if len(updatedEvents) < 4 {
		t.Errorf("expected at least 4 transition events, got %d", len(updatedEvents))
	}

	// 6. Rollback succeeded remediation
	rolledBack, rollRes, err := svc.Rollback(ctx, orgID, rem.ID, userID, "Incident resolved, test complete")
	if err != nil {
		t.Fatalf("failed to rollback remediation: %v", err)
	}
	if rolledBack.Status != StatusRolledBack {
		t.Errorf("expected status rolled_back, got %s", rolledBack.Status)
	}
	if rollRes == nil || !rollRes.Success {
		t.Errorf("expected rollback result success")
	}

	// 7. Test Rejection flow on a newly proposed remediation
	rejInput := CreateRemediationInput{
		IncidentID:     incidentID,
		ActionType:     ActionClearCache,
		Target:         "redis-cluster",
		Params:         map[string]any{"pattern": "*"},
		Rationale:      "Clear all keys",
		RiskLevel:      RiskCritical,
		ExpectedEffect: "Cache emptied",
		RollbackPlan:   "None",
		TTL:            5 * time.Minute,
	}
	remToReject, err := svc.Propose(ctx, orgID, rejInput)
	if err != nil {
		t.Fatalf("failed to propose for rejection: %v", err)
	}

	rejected, err := svc.Reject(ctx, orgID, remToReject.ID, userID, "Too risky, will drop user sessions")
	if err != nil {
		t.Fatalf("failed to reject remediation: %v", err)
	}
	if rejected.Status != StatusRejected {
		t.Errorf("expected status rejected, got %s", rejected.Status)
	}

	// 8. Test TTL Expiration
	expInput := CreateRemediationInput{
		IncidentID:     incidentID,
		ActionType:     ActionPauseConsumer,
		Target:         "worker-pool",
		Params:         map[string]any{"duration_seconds": 10.0},
		Rationale:      "Brief pause",
		RiskLevel:      RiskMedium,
		ExpectedEffect: "Paused",
		RollbackPlan:   "Resume",
		TTL:            -1 * time.Second, // Already expired
	}
	expRem, err := svc.Propose(ctx, orgID, expInput)
	if err != nil {
		t.Fatalf("failed to propose expired remediation: %v", err)
	}

	// Attempt approval on expired remediation
	_, _, err = svc.Approve(ctx, orgID, expRem.ID, userID, "Approving late")
	if err == nil {
		t.Errorf("expected error approving expired remediation")
	}

	// 9. Test Tenant Isolation: different org cannot view or approve
	otherOrgID := parseUUID("00000000-0000-0000-0000-000000000002")
	_, _, err = svc.Get(ctx, otherOrgID, rem.ID)
	if err == nil {
		t.Errorf("expected tenant isolation to block other org from reading remediation")
	}
}
