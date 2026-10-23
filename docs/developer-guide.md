# Ayin (ዓይን) — Developer & Integration Guide 👁️

> Complete guide for engineers connecting repositories, CI/CD pipelines, monitoring stacks, and AI IDEs to the **Ayin** autonomous incident triage agent.

---

## 1. Quick Architecture Overview

Ayin (*Amharic for "Eye"*) operates as an autonomous, read-only diagnostic agent:

```mermaid
flowchart LR
    CI["GitHub Actions / GitLab CI"] -->|1. Register Deploys| AYIN["Ayin Control Plane (:8080)"]
    MONITOR["Prometheus / Grafana / PagerDuty"] -->|2. Webhook Alerts| AYIN
    AYIN -->|3. Read-Only Diagnostics| CLOUD["Logs / Metrics / Topology"]
    AYIN -->|4. Ranked Hypotheses & Fixes| UI["Ayin Web Console (:3000)"]
    AYIN -->|5. MCP Server (:8081 or stdio)| IDE["Cursor / Claude Desktop"]
```

---

## 2. GitHub Actions Integration

Track every deployment so that when an incident occurs, Ayin immediately correlates the outage with the exact commit hash and PR author.

Add this step to `.github/workflows/deploy.yml`:

```yaml
name: Deploy Production
on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout Code
        uses: actions/checkout@v4

      - name: Deploy Service
        run: ./scripts/deploy-prod.sh

      # Register deployment with Ayin
      - name: Notify Ayin (ዓይን)
        if: success()
        run: |
          curl -s -X POST "https://ayin.your-domain.com/api/v1/ingest/webhooks/generic" \
            -H "Content-Type: application/json" \
            -d '{
              "title": "Production Deployment: ${{ github.repository }}",
              "service": "${{ github.event.repository.name }}",
              "severity": "info",
              "status": "resolved",
              "description": "Commit ${{ github.sha }} deployed by ${{ github.actor }}",
              "labels": {
                "git_commit": "${{ github.sha }}",
                "git_branch": "${{ github.ref_name }}",
                "version": "${{ github.ref_name }}"
              }
            }'
```

---

## 3. GitLab CI/CD Integration

In your `.gitlab-ci.yml`, register deployments in your production deploy stage:

```yaml
deploy_production:
  stage: deploy
  script:
    - ./scripts/deploy-prod.sh
    # Register release with Ayin
    - |
      curl -s -X POST "https://ayin.your-domain.com/api/v1/ingest/webhooks/generic" \
        -H "Content-Type: application/json" \
        -d "{
          \"title\": \"Production Deployment: $CI_PROJECT_NAME\",
          \"service\": \"$CI_PROJECT_NAME\",
          \"severity\": \"info\",
          \"status\": \"resolved\",
          \"description\": \"Commit $CI_COMMIT_SHA deployed by $GITLAB_USER_LOGIN\",
          \"labels\": {
            \"git_commit\": \"$CI_COMMIT_SHA\",
            \"git_branch\": \"$CI_COMMIT_REF_NAME\"
          }
        }"
  only:
    - main
```

---

## 4. Alert Ingestion (Monitoring Webhooks)

When an outage occurs, your monitoring system forwards the alert to Ayin.

### A. Prometheus Alertmanager
In your `alertmanager.yml`:

```yaml
receivers:
  - name: 'ayin-alerts'
    webhook_configs:
      - url: 'https://ayin.your-domain.com/api/v1/ingest/webhooks/alertmanager'
        send_resolved: true

route:
  receiver: 'ayin-alerts'
  group_by: ['alertname', 'service']
  group_wait: 10s
  repeat_interval: 1h
```

### B. Grafana Contact Point
1. In Grafana, navigate to **Alerting > Contact points > Add contact point**.
2. Select **Webhook**.
3. Set URL to `https://ayin.your-domain.com/api/v1/ingest/webhooks/grafana`.
4. Enable HTTP POST.

### C. Generic / Datadog / Sentry / CloudWatch
Send standard JSON payloads to `POST /api/v1/ingest/webhooks/generic`:

```json
{
  "title": "500 Internal Server Error surge",
  "service": "checkout-api",
  "severity": "sev1",
  "status": "firing",
  "description": "5xx error rate exceeded 5% over 5 minutes"
}
```

---

## 5. Model Context Protocol (MCP) Setup

Connect your coding assistant (**Cursor IDE**, **Claude Desktop**, etc.) directly to Ayin’s live diagnostic tools.

Add this entry to your IDE's MCP settings (e.g. `~/.cursor/mcp.json` or `claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "ayin": {
      "command": "/path/to/Ayin/apps/api/bin/mcp",
      "args": [
        "-transport", "stdio",
        "-org-id", "00000000-0000-0000-0000-000000000001"
      ]
    }
  }
}
```

### Tools exposed to your AI assistant:
* `log_search`: Query live error logs via regex and time windows.
* `recent_deploys`: Check recently merged commit SHAs and authors.
* `query_metrics`: Run PromQL queries for memory, CPU, or queue backlogs.
* `service_topology`: Inspect upstream callers and downstream databases.
* `search_runbooks`: Search team SOPs using pgvector embeddings.
* `similar_incidents`: Find past resolved outages matching the current symptom.

---

## 6. Security & Multi-Tenancy

* **Read-Only Safety**: The autonomous agent cannot restart servers, alter code, or delete data on its own.
* **Human-in-the-Loop (HITL)**: Any proposed remediation (such as a pod rollback or scale action) must be confirmed by a human responder in the web console or via CLI.
* **PostgreSQL Row-Level Security (RLS)**: Every alert, metric query, runbook, and audit log is partitioned strictly by `org_id`.
