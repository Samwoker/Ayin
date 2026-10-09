# ADR 0005: Read-Only Investigation Tools and Organization Policy Registry

## Status
Accepted

## Context
In Phase 5, the autonomous incident triage agent requires tools to explore telemetry, infrastructure state, historical incidents, and documentation:
1. **Safety & Zero-Side-Effect Guarantee**: Investigation tools must be strictly read-only. Destructive operations (pod restarts, rollbacks, cache flushes) must never be executed during investigation; they are proposed as human-approved remediations.
2. **Tenancy Boundaries**: Every tool invocation must operate strictly within the tenant's security context (`org_id`). A tenant must never access another tenant's logs, runbooks, deployments, or historical incidents.
3. **Resilience & Fault Tolerance**: External queries (Prometheus, Loki, databases) can hang, timeout, or panic. The tool subsystem must enforce per-execution timeouts, isolate panics, and return structured error diagnostics to the agent rather than crashing the runtime.
4. **Policy & Compliance**: Organizations must be able to selectively allowlist, disable, or gate access to specific tools (e.g., disable log searching or restrict metric access based on organizational compliance).
5. **Model Context Protocol (MCP) Compatibility**: Tools must publish standard JSON Schema parameter specifications so they can be seamlessly bound to LLM function calling and exported over the official Go MCP SDK.

## Decision

We implement a decoupled, safe, read-only investigation tool registry:

### 1. Unified Tool Interface & Structured Results
Every tool implements the `Tool` interface:
```go
type Tool interface {
    Name() string
    Description() string
    Schema() ToolSchema
    Execute(ctx context.Context, orgID pgtype.UUID, input json.RawMessage) (ToolResult, error)
}
```
All invocations produce a standardized `ToolResult` containing execution status (`Success`), structured `Data`, error messages, measured `LatencyMs`, and diagnostic metadata.

### 2. Organization Policy and Access Control (`PolicyProvider`)
- Tool execution is mediated by `PolicyProvider` and `AllowlistPolicyProvider`.
- Supports global tool kill-switches, per-tenant allowlists, and per-tenant explicit tool denials.
- Disallowed tools fail fast without executing any network or database calls.

### 3. Registry Execution Lifecycle & Panic Recovery
The `ToolRegistry.Execute` lifecycle guarantees:
1. Tool lookup and existence validation.
2. Tenant authorization check via `PolicyProvider.IsToolAllowed`.
3. Per-tool execution timeout enforcement (`WithTimeout`).
4. Panic recovery wrapper (`recover()`) capturing stack traces and returning structured tool error results.
5. High-precision latency measurement in milliseconds.

### 4. Six Core Investigation Tools
1. **`log_search`**: Leverages the `LogAdapter` interface (`FileLogAdapter` or `LokiLogAdapter`) to search service logs with regex pattern filtering and time-window bounds.
2. **`recent_deploys`**: Retrieves service release metadata, deployed versions, git commit hashes, deployers, and rollback targets to pinpoint release regressions.
3. **`service_topology`**: Traverses the microservice architecture graph to identify upstream consumers, downstream APIs, databases, message queues, and SLO tiers.
4. **`query_metrics`**: Executes PromQL queries over time windows, evaluating CPU, memory leaks, queue backpressure, latency percentiles, and error rate spikes with calculated summary statistics (`min`, `max`, `avg`, `latest`).
5. **`search_runbooks`**: Queries internal SOPs and markdown runbooks in PostgreSQL under tenant RLS transaction (`WithOrgTx`), providing mitigation checklists and commands.
6. **`similar_incidents`**: Searches prior resolved incidents in PostgreSQL under tenant RLS transaction (`WithOrgTx`) to identify recurrent failure patterns and previously successful remediations.

### 5. API Exposure
- `GET /api/v1/tools`: Discovers all available tools and JSON parameter schemas for the authenticated organization.
- `POST /api/v1/tools/{name}/execute`: Allows authorized responders (`responder` role or above) to run diagnostic tools on-demand within their tenant boundary.

## Alternatives Considered

- **Embedding Raw Functions Directly in the LLM Agent Loop**:
  - *Rejected*: Direct function binding lacks runtime isolation, schema self-documentation, tenant policy enforcement, and cannot be reused across API endpoints or the MCP CLI server.
- **Granting Mutating Abilities to Investigation Tools**:
  - *Rejected*: Violates core principle of least privilege. Investigation is strictly diagnostic; remediation proposals are handled through the human-in-the-loop approval workflow in Phase 7.

## Consequences

- **Positive**: 79.5% automated test coverage across registry, policy provider, and all 6 tool implementations.
- **Positive**: Strict tenant isolation enforced at the registry layer and within SQL transactions (`WithOrgTx`).
- **Positive**: Safe panic and timeout isolation protects the agent runner and API server.
- **Positive**: Tools publish standard JSON Schemas ready for LLM tool calling in Phase 6 and MCP server export in Phase 8.
- **Trade-off**: Requires mock/provider adapters when testing or running in disconnected local environments without live Prometheus/Loki clusters.
