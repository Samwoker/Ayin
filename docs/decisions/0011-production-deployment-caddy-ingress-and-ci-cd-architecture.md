# ADR 0011: Production Deployment, Caddy Ingress, and CI/CD Architecture

## Status
Accepted

## Context
Deploying the autonomous incident triage platform into production requires a unified, secure ingress topology, lightweight container runtimes, and a continuous delivery pipeline:
1. **Unified Ingress & Unbuffered SSE Streaming**: The platform exposes both a high-throughput REST API with real-time Server-Sent Events (SSE) and a SvelteKit server-rendered frontend. Traditional reverse proxies buffer responses by default, which delays or stalls real-time streaming traces. A modern reverse proxy with explicit unbuffered flushing is mandatory.
2. **Container Security & Minimal Footprint**: In accordance with cloud-native security standards, production containers must not include package managers, compilers, or root privileges.
3. **Continuous Verification & Artifact Publishing**: Pull requests must pass automated linting, database-backed integration tests, benchmark evaluation gating (`cmd/eval`), and multi-binary builds before deploying container images to GitHub Container Registry (GHCR).

---

## Decision

We implement production containerization, Caddy ingress routing, and GitHub Actions CI/CD:

### 1. Caddy Reverse Proxy Ingress (`deploy/Caddyfile`)
Caddy serves as the unified ingress point on port 80/443:
- **Unbuffered SSE Streaming**: The API reverse proxy block configures `flush_interval -1` on `/api/*`, guaranteeing real-time delivery of diagnostic agent steps and remediation lifecycle updates without intermediate buffer delays.
- **Path Routing**:
  - `/api/*`, `/healthz`, `/readyz`, `/metrics` -> `api:8080` (Go backend).
  - All remaining traffic -> `web:3000` (SvelteKit SSR node frontend).
- **Security Hardening**: Enforces `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: strict-origin-when-cross-origin`, and `Permissions-Policy`.

### 2. Multi-Stage Container Architecture
- **Backend Image (`apps/api/Dockerfile`)**:
  - *Builder Stage*: Golang 1.23 Alpine compiles all 6 static binaries (`api`, `worker`, `mcp`, `triage`, `eval`, `simulate`) with `-trimpath -ldflags="-s -w"`.
  - *Runtime Stage*: Google Distroless Static Debian 12 (`gcr.io/distroless/static-debian12:nonroot`) with CA certificates, non-root user execution (`nonroot:nonroot`), and minimal attack surface.
- **Frontend Image (`apps/web/Dockerfile`)**:
  - *Builder Stage*: Node 22 Alpine runs `npm ci`, compiles SvelteKit Vite bundle (`npm run build`), and prunes development dependencies.
  - *Runtime Stage*: Lightweight Node 22 Alpine running under the unprivileged `node` user.

### 3. Production Docker Compose Stack (`docker-compose.yml`)
Orchestrates the complete cloud-native platform:
- `postgres`: PostgreSQL 16 with `pgvector` extension.
- `redis`: Redis 7 with Append-Only File (AOF) persistence for stream queues.
- `api`: Control plane HTTP server (Chi).
- `worker`: Autonomous agent execution runner.
- `mcp`: Model Context Protocol server exposing diagnostic tools on `:8081`.
- `web`: SvelteKit incident response console.
- `prometheus` & `grafana`: Platform observability and metric dashboards.
- `caddy`: Unified ingress reverse proxy on `:80`.

### 4. GitHub Actions CI/CD Pipeline (`.github/workflows/ci.yml`)
Automates complete validation and image publishing:
- `lint`: Runs `go vet` and `svelte-check` type audits.
- `test-backend`: Executes Go test suite with active PostgreSQL and Redis service containers.
- `test-frontend`: Runs Vitest component test suites.
- `eval-benchmark`: Executes `make eval` to prevent regression against `eval_baseline.json`.
- `build`: Compiles all 7 Go binaries and the SvelteKit production bundle.
- `publish-ghcr`: Upon merging to `main`, uses Docker Buildx to build and publish versioned container images to GitHub Container Registry (`ghcr.io/triage-agent/api`, `ghcr.io/triage-agent/web`).

---

## Consequences

### Positive
- Single-port deployment with zero-configuration reverse proxying and real-time SSE streaming.
- Distroless non-root containers minimize CVE exposure and compliance overhead.
- Fully automated quality gates guarantee that no code reaching `main` degrades benchmark triage accuracy.
