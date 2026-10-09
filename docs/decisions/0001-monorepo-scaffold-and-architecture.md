# ADR 0001: Monorepo Scaffold and Core Architecture

## Status
Accepted

## Context
Incident triage requires high concurrency, low latency, reliable telemetry ingest, real-time UI streaming, and strict isolation between tenants. We needed to choose the architecture and foundational tech stack for `triage` that allows rapid iteration, robust type safety, deterministic testing, and multi-tenant isolation.

## Decision
We adopt a monorepo structure with:
1. **Backend**: Go 1.23+ using `chi` for HTTP routing, `slog` for structured JSON logging, PostgreSQL with `pgvector` for storage and embeddings, and Redis for queues and rate-limiting.
2. **Frontend**: SvelteKit with Svelte 5 runes, TypeScript, and TailwindCSS for reactive SSE streaming and trace replay.
3. **API Contract**: OpenAPI 3.1 as the single source of truth (`api/openapi.yaml`).
4. **Infra/Deploy**: Docker Compose with Caddy for local development and edge routing; distroless container images for production.
5. **Architectural Isolation**: Multi-tenant Row-Level Security (RLS) built into the schema from day one (`org_id` on all tables, runtime verification via `SET LOCAL app.org_id`).

## Alternatives Considered
- **Python / LangChain Backend**: Rejected due to high memory overhead, runtime duck-typing risks, poor concurrency profile under high alert volume, and opaque framework abstractions.
- **Microservices across multiple repositories**: Rejected for early-stage development; monorepo enables atomic contract generation, unified CI, and simplified scenario simulations.
- **Schema-per-tenant PostgreSQL**: Rejected because maintaining hundreds of schema migrations at scale introduces operational overhead, whereas RLS with composite `(org_id, ...)` indexes provides rock-solid isolation with unified migrations.

## Consequences
- Single repository simplifies CI, end-to-end evaluation, and deployment.
- Strict compile-time checks in Go and TypeScript prevent common runtime regressions.
- All external dependencies and LLM calls must be behind interfaces to allow offline deterministic testing via mock providers.
