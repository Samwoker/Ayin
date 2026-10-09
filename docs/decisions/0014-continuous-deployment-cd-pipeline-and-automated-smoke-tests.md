# 14. Continuous Deployment (CD) Pipeline, Release Gating, and Automated Smoke Tests

Date: 2026-10-09

## Status
Accepted

## Context
ADR 0011 established the initial CI pipeline, Caddy ingress, and container publishing to GitHub Container Registry (GHCR). However, the delivery lifecycle ended at image publishing without an automated Continuous Deployment (CD) stage. 

The project required a robust Continuous Deployment pipeline to:
1. Promote published images automatically to Staging upon merges to `main`.
2. Apply database migrations before routing traffic.
3. Verify deployments with automated smoke test suites covering ingress, liveness, readiness, Stripe billing endpoints, metrics, and the web console.
4. Provide zero-downtime rolling updates with automatic rollback on healthcheck failure.
5. Provide a gated production deployment stage with environment URLs and deployment summaries.

## Decision

### 1. Extended GitHub Actions CD Pipeline (`.github/workflows/ci.yml`)
- Added two consecutive deployment stages following `publish-ghcr`:
  - **`deploy-staging`**:
    - Environment: `staging` (`https://staging.triage.internal`).
    - Deploys automatically on push to `main`.
    - Spins up staging containers via Docker Compose.
    - Applies database migrations with `/app/seed`.
    - Executes the automated smoke test suite against all core endpoints.
    - Emits a deployment verification table to `$GITHUB_STEP_SUMMARY`.
  - **`deploy-production`**:
    - Environment: `production` (`https://triage.internal`).
    - Depends on `deploy-staging`.
    - Can be protected with GitHub Environment approval rules.
    - Executes production release gates and records deployment status in `$GITHUB_STEP_SUMMARY`.

### 2. Automated Smoke Test Suite (`deploy/scripts/smoke-test.sh`)
- Validates end-to-end platform health with configurable retry and timeout loops:
  1. **Ingress & Routing**: Caddy ingress probe (`/caddy-healthz`).
  2. **Core Backend**:
     - Liveness (`/healthz` -> 200 OK).
     - Readiness (`/readyz` -> 200 OK with Postgres & Redis checks).
     - Prometheus telemetry (`/metrics` -> 200 OK).
     - System overview status API (`/api/v1/status` -> 200 OK).
  3. **Stripe Billing & Plans**: Public billing plans catalog (`/api/v1/billing/plans` -> 200 OK).
  4. **Frontend Console**: SvelteKit SSR root response (`/` -> 200 OK).

### 3. Production Deployment Script (`deploy/scripts/deploy.sh`)
- Implements rolling zero-downtime container updates:
  - Pulls latest versioned container images from GHCR.
  - Applies database schema migrations before updating running services.
  - Starts updated containers with `--no-deps --remove-orphans`.
  - Runs `smoke-test.sh` against the target.
  - **Automated Rollback Guard**: If smoke tests fail within the timeout window, automatically captures container logs, rolls back to the previous stable image tag, and terminates with an error code.

### 4. Production Compose Specification (`deploy/docker-compose.prod.yml`)
- Configures production-grade settings:
  - Uses tagged GHCR images (`${API_IMAGE}:${IMAGE_TAG}`).
  - Non-root distroless container execution for Go services (`api`, `worker`, `mcp`).
  - Non-root user execution (`node`) for the SvelteKit frontend (`web`).
  - JSON-file logging drivers with log rotation (`max-size: 50m`, `max-file: 5`).
  - Container healthchecks on all services with restart policy `always`.

### 5. Makefile Automation
- Added targets:
  - `make deploy`: Triggers `deploy/scripts/deploy.sh`.
  - `make smoke-test`: Runs `deploy/scripts/smoke-test.sh`.

## Consequences
- **True Continuous Deployment**: Eliminates manual intervention for staging updates while maintaining automated rollback safety.
- **Fast Feedback Loop**: Catches configuration drift, broken migrations, or broken endpoints immediately via smoke testing before traffic shifts.
- **Auditable Deployments**: GitHub Environment summaries track commit SHAs, image tags, and verification statuses for every release.
