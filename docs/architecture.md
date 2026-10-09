# Architecture & System Design

`triage` is an autonomous, multi-tenant incident triage agent designed for cloud-native engineering organizations. It ingests alerts and logs, deploys read-only investigative tools, formulates and scores root-cause hypotheses with concrete evidence links, and generates actionable first-response checklists.

## High-Level Architecture Diagram

```mermaid
flowchart TD
    subgraph Ingestion ["Ingestion Layer"]
        AM["Alertmanager / Grafana"] -->|Webhook| INGEST["Webhook Handler (HMAC Verified)"]
        PD["PagerDuty / Generic"] -->|Webhook| INGEST
        LOKI["Loki / OTLP Logs"] -->|Stream| LOG_INGEST["Log Adapter"]
    end

    subgraph Platform ["Triage Control Plane"]
        INGEST --> GROUP["Alert Deduplication & Grouping"]
        LOG_INGEST --> GROUP
        GROUP --> DB[(PostgreSQL 16 + pgvector)]
        GROUP --> QUEUE[(Redis 7 / Asynq Queue)]

        API["HTTP API Server (Chi)"] <--> DB
        API <--> QUEUE
        SSE["SSE Event Stream"] --> WEB["SvelteKit Web App"]
    end

    subgraph AgentWorker ["Investigation Agent (Worker)"]
        QUEUE --> WORKER["Agent Loop Runner"]
        WORKER --> PLANNER["LLM Planner (OpenAI / Anthropic / Mock)"]
        WORKER --> GUARD["Guardrails & Redaction (PII / Secrets)"]
        WORKER --> TOOLS["Tool Registry (Read-Only Allowlist)"]
    end

    subgraph ReadOnlyTools ["Read-Only Investigative Tools"]
        TOOLS --> T_LOGS["log_search"]
        TOOLS --> T_DEPLOYS["recent_deploys"]
        TOOLS --> T_TOPO["service_topology"]
        TOOLS --> T_METRICS["query_metrics (Prometheus)"]
        TOOLS --> T_RUNBOOKS["search_runbooks (pgvector)"]
        TOOLS --> T_SIMILAR["similar_incidents"]
    end

    subgraph Observability ["Observability & Metrics"]
        API --> PROM["Prometheus"]
        WORKER --> PROM
        PROM --> GRAF["Grafana Dashboards"]
    end
```

## Core Tenets

1. **Safety First**: The investigative agent operates strictly within a read-only tool allowlist. It is physically incapable of mutating infrastructure during triage. Remediations are strictly proposed (never executed directly) and require human-in-the-loop responder approval.
2. **Multi-Tenancy from Day One**: Every database entity is partitioned by `org_id`, enforced via PostgreSQL Row-Level Security (RLS) with session settings (`SET LOCAL app.org_id`).
3. **Evidence-Linked Hypotheses**: Hypotheses produced by the LLM planner are rejected unless backed by traceable, non-hallucinated run steps and evidentiary excerpts.
4. **Deterministic Evaluation**: Every prompt change or agent iteration is continuously scored across 15 benchmark scenarios via automated evaluation CLI (`cmd/eval`), gating regressions in CI.
