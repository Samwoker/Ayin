package remediation

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/seraph/triage/apps/api/internal/server/sse"
	"github.com/seraph/triage/apps/api/internal/store"
	"github.com/seraph/triage/apps/api/internal/store/db"
	"github.com/seraph/triage/apps/api/internal/telemetry"
)

// RemediationService orchestrates proposing, approving, rejecting, executing, and auditing remediations.
type RemediationService struct {
	store    *store.Store
	executor RemediationExecutor
	broker   *sse.Broker
	logger   *slog.Logger
}

// NewRemediationService creates a new RemediationService.
func NewRemediationService(
	st *store.Store,
	executor RemediationExecutor,
	broker *sse.Broker,
	logger *slog.Logger,
) *RemediationService {
	if executor == nil {
		executor = NewDefaultRemediationExecutor()
	}
	if logger == nil {
		logger = telemetry.SetupLogger("info", "remediation")
	}

	return &RemediationService{
		store:    st,
		executor: executor,
		broker:   broker,
		logger:   logger,
	}
}

// Propose submits a new remediation for human review.
func (s *RemediationService) Propose(ctx context.Context, orgID pgtype.UUID, input CreateRemediationInput) (*db.Remediation, error) {
	if !IsValidAction(input.ActionType) {
		return nil, fmt.Errorf("%w: %s", ErrInvalidAction, input.ActionType)
	}
	if !IsValidRisk(input.RiskLevel) {
		return nil, fmt.Errorf("%w: %s", ErrInvalidRisk, input.RiskLevel)
	}
	if input.Target == "" {
		return nil, ErrMissingTarget
	}

	ttl := input.TTL
	if ttl == 0 {
		ttl = 15 * time.Minute
	}
	expiresAt := time.Now().UTC().Add(ttl)

	paramsJSON, err := json.Marshal(input.Params)
	if err != nil {
		paramsJSON = []byte("{}")
	}

	var rem db.Remediation
	err = s.store.WithOrgTx(ctx, orgID, func(q *db.Queries) error {
		created, err := q.CreateRemediation(ctx, db.CreateRemediationParams{
			OrgID:          orgID,
			IncidentID:     input.IncidentID,
			HypothesisID:   input.HypothesisID,
			ActionType:     input.ActionType,
			Target:         input.Target,
			Params:         paramsJSON,
			Rationale:      input.Rationale,
			RiskLevel:      input.RiskLevel,
			ExpectedEffect: input.ExpectedEffect,
			RollbackPlan:   input.RollbackPlan,
			Status:         StatusProposed,
			ExpiresAt:      pgtype.Timestamptz{Time: expiresAt, Valid: true},
		})
		if err != nil {
			return err
		}
		rem = created

		// Record initial creation audit event
		_, _ = q.CreateRemediationEvent(ctx, db.CreateRemediationEventParams{
			OrgID:          orgID,
			RemediationID:  rem.ID,
			TransitionFrom: "",
			TransitionTo:   StatusProposed,
			UserID:         pgtype.UUID{},
			Reason:         "Remediation proposed",
		})

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to propose remediation: %w", err)
	}

	// Broadcast SSE event
	if s.broker != nil {
		s.broker.Broadcast(uuidToStr(rem.IncidentID), "remediation_proposed", map[string]any{
			"remediation_id": uuidToStr(rem.ID),
			"incident_id":    uuidToStr(rem.IncidentID),
			"action_type":    rem.ActionType,
			"target":         rem.Target,
			"risk_level":     rem.RiskLevel,
			"status":         rem.Status,
			"expires_at":     expiresAt.Format(time.RFC3339),
		})
	}

	return &rem, nil
}

// Get retrieves a remediation and its full transition history, checking for expiration.
func (s *RemediationService) Get(ctx context.Context, orgID, remediationID pgtype.UUID) (*db.Remediation, []db.RemediationEvent, error) {
	var rem db.Remediation
	var events []db.RemediationEvent

	err := s.store.WithOrgTx(ctx, orgID, func(q *db.Queries) error {
		found, err := q.GetRemediationByID(ctx, db.GetRemediationByIDParams{
			OrgID: orgID,
			ID:    remediationID,
		})
		if err != nil {
			return ErrRemediationNotFound
		}

		// Check expiration if still proposed
		if found.Status == StatusProposed && time.Now().UTC().After(found.ExpiresAt.Time) {
			updated, err := q.UpdateRemediationStatus(ctx, db.UpdateRemediationStatusParams{
				OrgID:  orgID,
				ID:     remediationID,
				Status: StatusExpired,
			})
			if err == nil {
				found = updated
				_, _ = q.CreateRemediationEvent(ctx, db.CreateRemediationEventParams{
					OrgID:          orgID,
					RemediationID:  remediationID,
					TransitionFrom: StatusProposed,
					TransitionTo:   StatusExpired,
					Reason:         "TTL expired before responder action",
				})
			}
		}

		evs, err := q.ListRemediationEvents(ctx, db.ListRemediationEventsParams{
			OrgID:         orgID,
			RemediationID: remediationID,
		})
		if err != nil {
			return err
		}

		rem = found
		events = evs
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	return &rem, events, nil
}

// ListForIncident retrieves all remediations for an incident, expiring any stale proposals.
func (s *RemediationService) ListForIncident(ctx context.Context, orgID, incidentID pgtype.UUID) ([]db.Remediation, error) {
	var items []db.Remediation

	err := s.store.WithOrgTx(ctx, orgID, func(q *db.Queries) error {
		rems, err := q.ListRemediationsByIncident(ctx, db.ListRemediationsByIncidentParams{
			OrgID:      orgID,
			IncidentID: incidentID,
		})
		if err != nil {
			return err
		}

		now := time.Now().UTC()
		for i := range rems {
			if rems[i].Status == StatusProposed && now.After(rems[i].ExpiresAt.Time) {
				updated, err := q.UpdateRemediationStatus(ctx, db.UpdateRemediationStatusParams{
					OrgID:  orgID,
					ID:     rems[i].ID,
					Status: StatusExpired,
				})
				if err == nil {
					rems[i] = updated
					_, _ = q.CreateRemediationEvent(ctx, db.CreateRemediationEventParams{
						OrgID:          orgID,
						RemediationID:  rems[i].ID,
						TransitionFrom: StatusProposed,
						TransitionTo:   StatusExpired,
						Reason:         "TTL expired before responder action",
					})
				}
			}
		}

		items = rems
		return nil
	})
	if err != nil {
		return nil, err
	}

	return items, nil
}

// Approve approves a proposed remediation and executes it immediately.
func (s *RemediationService) Approve(
	ctx context.Context,
	orgID, remediationID, userID pgtype.UUID,
	reason string,
) (*db.Remediation, *ExecutionResult, error) {
	var rem db.Remediation
	var params map[string]any

	// 1. Validate & transition: proposed -> approved -> executing
	err := s.store.WithOrgTx(ctx, orgID, func(q *db.Queries) error {
		found, err := q.GetRemediationByID(ctx, db.GetRemediationByIDParams{
			OrgID: orgID,
			ID:    remediationID,
		})
		if err != nil {
			return ErrRemediationNotFound
		}

		// Check TTL
		if time.Now().UTC().After(found.ExpiresAt.Time) {
			_, _ = q.UpdateRemediationStatus(ctx, db.UpdateRemediationStatusParams{
				OrgID:  orgID,
				ID:     remediationID,
				Status: StatusExpired,
			})
			_, _ = q.CreateRemediationEvent(ctx, db.CreateRemediationEventParams{
				OrgID:          orgID,
				RemediationID:  remediationID,
				TransitionFrom: found.Status,
				TransitionTo:   StatusExpired,
				UserID:         userID,
				Reason:         "Attempted approval on expired remediation",
			})
			return ErrRemediationExpired
		}

		if found.Status != StatusProposed {
			return fmt.Errorf("%w: cannot approve remediation in status %q", ErrInvalidTransition, found.Status)
		}

		// Step A: Mark approved
		_, err = q.UpdateRemediationStatus(ctx, db.UpdateRemediationStatusParams{
			OrgID:  orgID,
			ID:     remediationID,
			Status: StatusApproved,
		})
		if err != nil {
			return err
		}

		appReason := "Approved by responder"
		if reason != "" {
			appReason = reason
		}
		_, _ = q.CreateRemediationEvent(ctx, db.CreateRemediationEventParams{
			OrgID:          orgID,
			RemediationID:  remediationID,
			TransitionFrom: StatusProposed,
			TransitionTo:   StatusApproved,
			UserID:         userID,
			Reason:         appReason,
		})

		// Step B: Mark executing
		executingRem, err := q.UpdateRemediationStatus(ctx, db.UpdateRemediationStatusParams{
			OrgID:  orgID,
			ID:     remediationID,
			Status: StatusExecuting,
		})
		if err != nil {
			return err
		}

		_, _ = q.CreateRemediationEvent(ctx, db.CreateRemediationEventParams{
			OrgID:          orgID,
			RemediationID:  remediationID,
			TransitionFrom: StatusApproved,
			TransitionTo:   StatusExecuting,
			UserID:         userID,
			Reason:         "Executing remediation action",
		})

		rem = executingRem
		_ = json.Unmarshal(rem.Params, &params)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	// 2. Execute via RemediationExecutor
	execRes, execErr := s.executor.Execute(ctx, rem.ActionType, rem.Target, params)

	// 3. Record outcome: executing -> succeeded / failed
	finalStatus := StatusSucceeded
	finalReason := "Action executed successfully"
	if execErr != nil || (execRes != nil && !execRes.Success) {
		finalStatus = StatusFailed
		if execErr != nil {
			finalReason = fmt.Sprintf("Execution error: %v", execErr)
		} else {
			finalReason = fmt.Sprintf("Execution failed: %s", execRes.Error)
		}
	}

	_ = s.store.WithOrgTx(ctx, orgID, func(q *db.Queries) error {
		finalRem, err := q.UpdateRemediationStatus(ctx, db.UpdateRemediationStatusParams{
			OrgID:  orgID,
			ID:     remediationID,
			Status: finalStatus,
		})
		if err == nil {
			rem = finalRem
		}

		_, _ = q.CreateRemediationEvent(ctx, db.CreateRemediationEventParams{
			OrgID:          orgID,
			RemediationID:  remediationID,
			TransitionFrom: StatusExecuting,
			TransitionTo:   finalStatus,
			UserID:         userID,
			Reason:         finalReason,
		})
		return nil
	})

	// Broadcast SSE update
	if s.broker != nil {
		s.broker.Broadcast(uuidToStr(rem.IncidentID), "remediation_updated", map[string]any{
			"remediation_id": uuidToStr(rem.ID),
			"incident_id":    uuidToStr(rem.IncidentID),
			"status":         rem.Status,
			"action_type":    rem.ActionType,
			"target":         rem.Target,
			"success":        finalStatus == StatusSucceeded,
		})
	}

	return &rem, execRes, execErr
}

// Reject rejects a proposed remediation.
func (s *RemediationService) Reject(
	ctx context.Context,
	orgID, remediationID, userID pgtype.UUID,
	reason string,
) (*db.Remediation, error) {
	var rem db.Remediation

	err := s.store.WithOrgTx(ctx, orgID, func(q *db.Queries) error {
		found, err := q.GetRemediationByID(ctx, db.GetRemediationByIDParams{
			OrgID: orgID,
			ID:    remediationID,
		})
		if err != nil {
			return ErrRemediationNotFound
		}

		// Check TTL
		if time.Now().UTC().After(found.ExpiresAt.Time) {
			_, _ = q.UpdateRemediationStatus(ctx, db.UpdateRemediationStatusParams{
				OrgID:  orgID,
				ID:     remediationID,
				Status: StatusExpired,
			})
			return ErrRemediationExpired
		}

		if found.Status != StatusProposed {
			return fmt.Errorf("%w: cannot reject remediation in status %q", ErrInvalidTransition, found.Status)
		}

		rejReason := "Rejected by responder"
		if reason != "" {
			rejReason = reason
		}

		updated, err := q.UpdateRemediationStatus(ctx, db.UpdateRemediationStatusParams{
			OrgID:  orgID,
			ID:     remediationID,
			Status: StatusRejected,
		})
		if err != nil {
			return err
		}

		_, _ = q.CreateRemediationEvent(ctx, db.CreateRemediationEventParams{
			OrgID:          orgID,
			RemediationID:  remediationID,
			TransitionFrom: StatusProposed,
			TransitionTo:   StatusRejected,
			UserID:         userID,
			Reason:         rejReason,
		})

		rem = updated
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Broadcast SSE update
	if s.broker != nil {
		s.broker.Broadcast(uuidToStr(rem.IncidentID), "remediation_updated", map[string]any{
			"remediation_id": uuidToStr(rem.ID),
			"incident_id":    uuidToStr(rem.IncidentID),
			"status":         rem.Status,
			"action_type":    rem.ActionType,
			"target":         rem.Target,
		})
	}

	return &rem, nil
}

// Rollback initiates rollback for a previously succeeded remediation.
func (s *RemediationService) Rollback(
	ctx context.Context,
	orgID, remediationID, userID pgtype.UUID,
	reason string,
) (*db.Remediation, *ExecutionResult, error) {
	var rem db.Remediation
	var params map[string]any

	err := s.store.WithOrgTx(ctx, orgID, func(q *db.Queries) error {
		found, err := q.GetRemediationByID(ctx, db.GetRemediationByIDParams{
			OrgID: orgID,
			ID:    remediationID,
		})
		if err != nil {
			return ErrRemediationNotFound
		}

		if found.Status != StatusSucceeded {
			return fmt.Errorf("%w: can only rollback succeeded remediations, current status is %q", ErrInvalidTransition, found.Status)
		}

		rem = found
		_ = json.Unmarshal(rem.Params, &params)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	// Execute rollback
	rollRes, rollErr := s.executor.Rollback(ctx, rem.ActionType, rem.Target, params)
	if rollErr != nil {
		return nil, rollRes, rollErr
	}

	rbReason := "Rollback triggered"
	if reason != "" {
		rbReason = reason
	}

	err = s.store.WithOrgTx(ctx, orgID, func(q *db.Queries) error {
		updated, err := q.UpdateRemediationStatus(ctx, db.UpdateRemediationStatusParams{
			OrgID:  orgID,
			ID:     remediationID,
			Status: StatusRolledBack,
		})
		if err != nil {
			return err
		}
		rem = updated

		_, _ = q.CreateRemediationEvent(ctx, db.CreateRemediationEventParams{
			OrgID:          orgID,
			RemediationID:  remediationID,
			TransitionFrom: StatusSucceeded,
			TransitionTo:   StatusRolledBack,
			UserID:         userID,
			Reason:         rbReason,
		})
		return nil
	})
	if err != nil {
		return nil, rollRes, err
	}

	if s.broker != nil {
		s.broker.Broadcast(uuidToStr(rem.IncidentID), "remediation_updated", map[string]any{
			"remediation_id": uuidToStr(rem.ID),
			"incident_id":    uuidToStr(rem.IncidentID),
			"status":         rem.Status,
			"action_type":    rem.ActionType,
			"target":         rem.Target,
		})
	}

	return &rem, rollRes, nil
}

func uuidToStr(id pgtype.UUID) string {
	return fmt.Sprintf("%x-%x-%x-%x-%x", id.Bytes[0:4], id.Bytes[4:6], id.Bytes[6:8], id.Bytes[8:10], id.Bytes[10:16])
}
