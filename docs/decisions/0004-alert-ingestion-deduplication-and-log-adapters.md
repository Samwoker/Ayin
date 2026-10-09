# ADR 0004: Alert Ingestion, Signature Verification, Incident Deduplication, and Log Adapters

## Status
Accepted

## Context
In Phase 4, `triage` requires an autonomous, resilient alert ingestion and log interrogation subsystem:
1. Inbound alerts arrive from diverse monitoring and paging systems (Prometheus Alertmanager, Grafana Alerting, PagerDuty, and custom JSON webhook senders).
2. Webhooks originate over the public Internet; payloads must be authenticated, verified for integrity, and protected against replay attacks without relying on client-supplied trust.
3. Alert storms (e.g. 50 pods failing simultaneously or repeated flapping alerts) must be deduplicated and correlated into unified incidents rather than triggering dozens of duplicate, expensive agent investigations.
4. Agent investigation workflows require querying historical and runtime logs across heterogeneous observability backends (local development file logs and centralized Grafana Loki clusters).
5. Webhook ingestion must be asynchronous: incoming HTTP webhooks must acknowledge receipt immediately (<50ms) while investigation workloads are enqueued for background worker execution.

## Decision

We implement a decoupled ingestion, deduplication, queueing, and log access architecture:

### 1. Webhook HMAC-SHA256 Verification & Replay Protection
- Every tenant organization is provisioned with a secure `webhook_secret` generated at creation time.
- The `VerifyWebhookSignature` engine validates incoming `X-Signature-256` (or `X-Hub-Signature-256`) headers using constant-time comparison (`subtle.ConstantTimeCompare`) against `HMAC-SHA256(secret, timestamp + body)`.
- Replay protection validates `X-Timestamp` against a strict clock-skew window (`DefaultReplayWindow = 5 * time.Minute`). Requests with expired or future timestamps are rejected before parsing.

### 2. Multi-Vendor Normalized Alert Parsers
- A pluggable `ParserRegistry` decouples HTTP request handling from vendor-specific payload structures.
- **Alertmanager**: Parses Prometheus Alertmanager v4 JSON payloads, unwrapping grouped `alerts`, extracting service names from labels, and mapping severities (`critical -> sev1`, `warning -> sev3`, `info -> sev4`). Generates deterministic fingerprint hashes when missing.
- **Grafana**: Supports both Grafana Unified Alerting and legacy webhook formats, extracting alert rule titles, labels, annotations, and metric evaluation summaries.
- **PagerDuty**: Handles PagerDuty v2/v3 incident events (`incident.triggered`, `incident.resolved`), extracting urgency levels and service metadata.
- **Generic JSON**: Accommodates raw single-alert or batch JSON arrays from CI/CD pipelines, Kubernetes controllers, and custom scripts.

### 3. Two-Tier Deduplication and Incident Grouping Engine (`AlertGrouper`)
Alerts are processed through a two-tier correlation strategy within a sliding time window (default 30 minutes):
1. **Fingerprint Match**: Checks for recent alerts with an identical cryptographic fingerprint linked to an open or active investigation. If found, records the alert and links it to the existing incident (`is_new = false`).
2. **Service-Level Open Incident**: If no fingerprint match exists, checks whether an incident for the same `service` is already open and investigating within the deduplication window. If so, correlates the alert into that incident (`is_new = false`).
3. **New Incident Creation**: If no correlation matches, opens a new incident in PostgreSQL (under tenant RLS `WithOrgTx`), attaches the initial alert, and dispatches an investigation job (`is_new = true`).

### 4. Asynchronous Redis Streams Investigation Queue
- Ingest HTTP handlers return immediately upon incident correlation.
- New incidents enqueue an `InvestigationJob` onto a Redis Stream (`stream:investigations`) using consumer groups (`triage-workers`).
- Background workers (`cmd/worker`) consume jobs via `XREADGROUP`, track acknowledgments (`XACK`), and invoke the triage agent runner.
- An in-memory queue (`MemoryQueue`) provides dependency-free local testing and offline execution.

### 5. Pluggable Log Adapter Interface (`LogAdapter`)
An extensible `LogAdapter` interface exposes a unified contract (`QueryLogs(ctx, query) ([]LogEntry, error)`):
- **`FileLogAdapter`**: Scans and parses structured JSON and plaintext logs from local files and directories with regex pattern filtering, timestamp windows, service scoping, and heuristic level inference (`DEBUG`, `INFO`, `WARN`, `ERROR`, `FATAL`).
- **`LokiLogAdapter`**: Executes LogQL range queries (`/loki/api/v1/query_range`) against Grafana Loki clusters, passing tenant scoping via `X-Scope-OrgID` headers.

## Alternatives Considered

- **Synchronous Agent Execution in Webhook Handlers**:
  - *Rejected*: LLM-driven root-cause investigation takes 5-30+ seconds. Webhook senders (Alertmanager, PagerDuty, Grafana) have strict timeout thresholds (5-10s) and retry on timeouts, which would cause cascading duplicate runs and thread starvation.
- **Redis Pub/Sub instead of Redis Streams**:
  - *Rejected*: Standard Pub/Sub lacks message persistence, delivery acknowledgments, consumer grouping, and backpressure buffering. Redis Streams provides distributed worker dispatch and crash recovery.
- **Strict Single Webhook URL vs Dual Path Addressing**:
  - *Decided*: We support both tenant-scoped URLs (`/api/v1/ingest/{org_id}/webhooks/{source}`) and authenticated generic URLs (`/api/v1/ingest/webhooks/{source}`). This allows monitoring tools that cannot inject custom auth headers to route to dedicated tenant URLs while retaining HMAC signature validation.

## Consequences

- **Positive**: Sub-millisecond webhook ingestion latency and immediate HTTP 200 acknowledgments to upstream monitoring systems.
- **Positive**: Eliminates alert storm duplicate triage runs by correlating repeat alerts into open incidents.
- **Positive**: 78.9% automated test coverage across parser implementations, queue transports, and log adapters.
- **Positive**: Clean separation between alert normalization and agent investigation workers.
- **Trade-off**: Requires running background worker processes (`cmd/worker`) alongside API instances to process queued investigation jobs.
