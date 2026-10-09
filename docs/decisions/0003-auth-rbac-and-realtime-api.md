# ADR 0003: Multi-Tenant Authentication, RBAC, Rate Limiting, and Real-Time SSE API

## Status
Accepted

## Context
Phase 3 requires establishing secure authentication, role-based access control (RBAC), rate limiting, and real-time streaming for `triage`:
1. Users must authenticate across multiple tenant organizations with distinct roles per organization (`owner`, `admin`, `responder`, `viewer`).
2. Automated systems and webhooks (Alertmanager, Grafana, CI) require org-scoped API keys with safe storage and revocability.
3. API traffic must be protected against abuse using distributed rate limiting per organization and per API key.
4. Investigation progress, tool invocations, and hypothesis updates must be streamed live to responders with minimal protocol overhead.

## Decision
We implement a unified, multi-layered authentication and real-time architecture:
1. **JWT Access & Refresh Tokens**:
   - Short-lived HS256 access tokens (15 minutes) carrying cryptographically signed tenant claims (`user_id`, `org_id`, `role`, `email`).
   - Long-lived refresh tokens (7 days) for rotating credentials without forcing constant re-logins.
   - Passwords hashed using OWASP-compliant Argon2id (`m=65536, t=3, p=2`) with constant-time equality checks.
2. **Per-Org RBAC**:
   - Closed role hierarchy: `owner (4) > admin (3) > responder (2) > viewer (1)`.
   - Middleware `RequireRole(minRole)` enforces permissions at the router layer.
3. **Org-Scoped API Keys**:
   - Keys generated with identifiable prefix (`tr_live_<slug>_<entropy>`).
   - Only SHA-256 hash stored at rest in `api_keys.key_hash`. Full key is displayed only once upon generation.
4. **Token-Bucket Rate Limiting**:
   - Dual-layer token bucket evaluating both per-organization and per-key quotas.
   - Implemented via atomic Redis Lua script with automatic in-memory fallback for local development or disconnected tests. Unauthenticated requests fall back to client IP throttling.
5. **Real-Time Streaming via Server-Sent Events (SSE)**:
   - Built around an in-memory broadcast broker (`sse.Broker`) supporting typed events (`run_created`, `step_started`, `step_completed`, `hypothesis_generated`).
   - Endpoints require tenant authentication (supporting Bearer headers and browser `?token=` query parameters) and enforce tenant ownership validation before permitting subscription.

## Alternatives Considered
- **Stateful sessions in Redis**:
   - *Pros*: Instant invalidation capability.
   - *Cons*: Every API call incurs a synchronous Redis roundtrip before reaching handlers; introduces Redis as a hard single point of failure for authentication.
- **WebSockets**:
   - *Pros*: Bi-directional communication.
   - *Cons*: Agent investigation streaming is unidirectional (server-to-client); WebSockets complicate proxy buffering (Caddy, Cloudflare, load balancers), firewall traversal, reconnect logic, and auth headers on initial handshake. SSE works seamlessly over standard HTTP/2 and HTTP/1.1 with `text/event-stream`.
- **Client-trusted `X-Org-ID` header**:
   - *Rejected*: Violates zero-trust tenancy rules. The active `org_id` must come from cryptographically signed JWT claims or validated API key records.

## Consequences
- **Positive**: Stateless, horizontally scalable API servers with zero-trust tenant resolution.
- **Positive**: Responders receive real-time streaming investigation traces with native browser `EventSource` support.
- **Positive**: Rate limiting protects against noisy neighbors and compromised API keys.
- **Trade-off**: SSE connections remain open for the duration of an investigation. Connection pooling and timeouts must be budgeted in reverse proxies (e.g. Caddy `flush_interval`).
