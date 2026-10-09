# ADR 0010: Responder CLI and Model Context Protocol (MCP) Server Architecture

## Status
Accepted

## Context
In addition to the web response console, incident responders and automation systems require robust command-line and machine-readable interfaces:
1. **Terminal-First Responder Workflow**: Site reliability engineers (SREs) and on-call responders during production outages need fast CLI access to list active incidents, inspect ranked hypotheses, stream live reasoning traces, and execute Human-in-the-Loop (HITL) remediations without leaving their terminal.
2. **External AI Tool Integration via MCP**: Modern developer environments (Claude Desktop, Cursor IDE, external autonomous coding agents) communicate through the open **Model Context Protocol (MCP)** specification. Providing native MCP support allows third-party models to safely query Triage's 6 read-only diagnostic tools (`log_search`, `recent_deploys`, `service_topology`, `query_metrics`, `search_runbooks`, `similar_incidents`) within the tenant's security boundary.
3. **Multi-Transport Interoperability**: The MCP server must support both `stdio` (for local IDE / desktop client integration) and `sse` (for remote agent and microservice connectivity).

---

## Decision

We implement the Responder CLI in `apps/api/internal/cli` (`cmd/triage`) and the Model Context Protocol Server in `apps/api/internal/mcp` (`cmd/mcp`):

### 1. Responder CLI Architecture (`cmd/triage` & `internal/cli`)
Built with Cobra (`github.com/spf13/cobra`) providing high-density terminal tables and optional `--json` machine outputs:
- **Global Configuration**:
  - Persistent flags: `--api-url`, `--api-key`, `--token`, `--org-id`, `--json`.
  - Fallback environment variables: `TRIAGE_API_URL`, `TRIAGE_API_KEY`, `TRIAGE_TOKEN`, `TRIAGE_ORG_ID`.
- **Command Hierarchy**:
  - `triage version` / `triage whoami`: Identity, organization context, and build verification.
  - `triage incidents [list|get|create|resolve]`: Filter by status, service, and severity tier.
  - `triage runs [list|get|tail]`: Inspect agent runs and stream live diagnostic steps via SSE (`tail`).
  - `triage remediations [list|get|approve|reject|rollback]`: Inspect risk tiers, remaining TTL countdowns, and execute HITL decisions with explicit confirmation checks (`--yes` to bypass).
  - `triage tools [list|exec]`: Inspect registered diagnostic tools and directly execute queries with JSON parameters.

### 2. Model Context Protocol (MCP) Server (`cmd/mcp` & `internal/mcp`)
Adheres strictly to the JSON-RPC 2.0 MCP protocol specification (v2024-11-05):
- **Core RPC Methods**:
  - `initialize`: Returns server info (`triage-mcp-server`) and declared capabilities (`tools`, `resources`, `prompts`).
  - `notifications/initialized`: Acknowledges client session establishment.
  - `ping`: Health verification.
  - `tools/list`: Formats and exposes all 6 diagnostic tools with dynamic JSON Schema parameter definitions.
  - `tools/call`: Executes the requested diagnostic tool within tenant `org_id` isolation and returns text content.
  - `resources/list` & `resources/read`: Exposes platform overview (`triage://system/overview`) and tool catalog (`triage://tools/catalog`).
  - `prompts/list` & `prompts/get`: Provides pre-configured incident investigation templates (`investigate_incident`, `review_remediation`).
- **Transports**:
  - **`stdio`**: Standard input/output line scanner. In stdio mode, standard output is strictly reserved for valid JSON-RPC lines, and logging is isolated to standard error (`os.Stderr`).
  - **`sse`**: HTTP server providing `/sse` event stream and `/message` JSON-RPC POST endpoint.

---

## Consequences

### Positive
- Responders can monitor and remediate incidents completely from the command line.
- Native interoperability with Claude Desktop, Cursor, and any MCP-compliant AI assistant.
- Strict tenant security policies and read-only boundaries are enforced identically across REST, Web, CLI, and MCP surfaces.

### Negative
- CLI binary requires network connectivity to the Triage API control plane.
