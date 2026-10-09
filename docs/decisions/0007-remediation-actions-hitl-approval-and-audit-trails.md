# ADR 0007: Remediation Actions Catalog, Human-in-the-Loop Approval Workflow, and Audit Trails

## Status
Accepted

## Context
In Phase 7, the autonomous incident triage agent and responders need a safe, robust mechanism to act upon verified root causes. Automated remediation presents high operational risks if executed unchecked:
1. **Safety & Least Privilege**: Mutating production environments (restarting services, scaling workloads, rolling back deployments, flushing caches) must never happen unprompted by autonomous agent scripts. A strict Human-in-the-Loop (HITL) gate is mandatory before any mutation executes.
2. **Action Catalog & Risk Tiers**: Operations must be classified into well-defined, standardized action primitives (`restart_service`, `scale_service`, `rollback_deploy`, `clear_cache`, `pause_consumer`, `toggle_feature_flag`) accompanied by risk assessments (`low`, `medium`, `high`, `critical`).
3. **Time-To-Live (TTL) Expiration**: Incident conditions evolve rapidly. A remediation proposed at minute 0 may become hazardous or irrelevant at minute 20. Stale proposals must expire automatically to prevent responders from approving out-of-date actions.
4. **Comprehensive Auditability & Attribution**: Enterprise compliance requires an immutable audit trail of every transition (`proposed`, `approved`, `rejected`, `expired`, `executing`, `succeeded`, `failed`, `rolled_back`) with responder identity (`user_id`), rationale, and execution timestamps.
5. **Rollback & Compensation**: Every succeeded remediation must offer an executable rollback plan or compensation mechanism should the intervention fail to resolve the incident or cause collateral issues.

## Decision

We implement a Human-in-the-Loop remediation subsystem in `apps/api/internal/remediation`:

### 1. Six Core Remediation Actions & Domain Model
The remediation engine standardizes 6 production actions:
1. **`restart_service`**: Performs rolling container/pod restarts with configurable grace periods to purge deadlocked workers or memory leaks.
2. **`scale_service`**: Horizontally scales target replicas up or down with previous replica state preservation.
3. **`rollback_deploy`**: Reverts workload deployments to previous stable container image tags or git commits.
4. **`clear_cache`**: Selectively invalidates keys matching target patterns across cache clusters without causing thundering herds.
5. **`pause_consumer`**: Halts message queue subscriber groups for a bounded duration to relieve downstream database lock contention.
6. **`toggle_feature_flag`**: Flips targeted feature flags dynamically to deactivate suspect code paths without requiring a full release.

### 2. Status Lifecycle State Machine
Remediations transition through strict state phases enforced by `RemediationService`:
- **`proposed`**: Created by agent or responder with parameters, rationale, expected effect, rollback plan, and expiration timestamp (default: 15 minutes TTL).
- **`approved`**: Responder (`responder` or `admin` role) reviews and explicitly approves the action.
- **`rejected`**: Responder rejects the action with a documented reason.
- **`expired`**: Triggered automatically if the proposal TTL lapses before responder review.
- **`executing`**: Action is dispatched to the `RemediationExecutor`.
- **`succeeded`**: Action executed with verified positive outcome.
- **`failed`**: Execution failed; diagnostic error captured.
- **`rolled_back`**: Rollback executed to compensate or revert a previously succeeded remediation.

### 3. Immutable Audit Trail (`remediation_events`)
Every status transition writes an immutable event into PostgreSQL `remediation_events` under tenant Row-Level Security (`WithOrgTx`):
- `remediation_id`, `transition_from`, `transition_to`
- `user_id` (authenticated responder UUID)
- `reason` (responder rationale or executor error message)
- `created_at` timestamp

### 4. Pluggable Execution & Rollback Engine (`RemediationExecutor`)
The decoupled execution layer:
```go
type RemediationExecutor interface {
    Execute(ctx context.Context, action string, target string, params map[string]any) (*ExecutionResult, error)
    Rollback(ctx context.Context, action string, target string, params map[string]any) (*ExecutionResult, error)
}
```
- `DefaultRemediationExecutor` implements validation and simulation for all 6 actions, alongside state-reverting rollback logic (e.g. reverting replica counts, toggling flags back, resuming consumers).
- Custom handlers can be injected for production infrastructure adapters (Kubernetes client-go, AWS ECS SDK, Unleash/LaunchDarkly client, Redis client).

### 5. API Endpoints & Real-Time SSE Broadcasting
- `GET /api/v1/incidents/{id}/remediations`: Lists all remediations for an incident.
- `POST /api/v1/incidents/{id}/remediations`: Proposes a new remediation (role: `responder`).
- `GET /api/v1/remediations/{id}`: Retrieves remediation details and complete event trail.
- `POST /api/v1/remediations/{id}/approve`: Approves and triggers execution (role: `responder`).
- `POST /api/v1/remediations/{id}/reject`: Rejects the proposed remediation (role: `responder`).
- `POST /api/v1/remediations/{id}/rollback`: Rolls back a succeeded remediation (role: `admin`).
- Real-time updates broadcast over `sse.Broker` topics (`remediation_proposed`, `remediation_updated`).

## Alternatives Considered

- **Fully Autonomous Self-Healing Without Human Approval**:
  - *Rejected*: In high-stakes production incidents, automated self-healing can easily trigger cascading failures, thundering herds, or severe data loss when an agent misidentifies the root cause. Requiring human confirmation is a fundamental safety invariant.
- **Ephemeral State Tracking Without Event Tables**:
  - *Rejected*: Incident post-mortems and compliance audits require full traceability of who approved what action, when it ran, and whether it succeeded.
