# ADR 0006: Autonomous Investigation Agent, Budget Enforcement, and Hypothesis Synthesis

## Status
Accepted

## Context
In Phase 6, the system requires an autonomous reasoning engine that investigates active alerts, invokes diagnostic tools through multi-turn function calling, adheres to strict organizational budget ceilings, and synthesizes ranked root-cause hypotheses with concrete evidence and actionable first-response checklists:
1. **Safety & Budget Ceilings**: Autonomous LLM agents can enter degenerative loops or invoke expensive models indefinitely. The system must enforce multi-dimensional budget caps (maximum execution steps, token ceilings, dollar-denominated cost caps, and run timeouts). When caps are exceeded, the agent must fail gracefully into a terminal `budget_exhausted` status rather than hanging or incurring unbounded costs.
2. **Anti-Hallucination & Evidence Citations**: AI incident analysis is prone to hallucinated errors or false confidence. Hypotheses must be grounded strictly in empirical evidence collected from tool observations (e.g. specific commit hashes, log error lines, metric values, topology paths) and scored with a calibrated confidence factor (0.00 to 1.00).
3. **Observability & Streaming Auditability**: Incident responders cannot wait in the dark for a multi-turn investigation to conclude. Every thought, tool call, latency measurement, and token cost must be persisted immediately to PostgreSQL under tenant RLS boundaries and broadcast in real time to connected clients over Server-Sent Events (SSE).
4. **Decoupled LLM Architecture**: The backend must support interchangeable model backends (OpenAI GPT-4o, Azure OpenAI, Anthropic, local vLLM/Ollama) and provide a deterministic multi-turn mock for unit tests, CI pipelines, and benchmarks without requiring live external API keys.

## Decision

We implement a modular, budget-constrained autonomous investigation engine in `apps/api/internal/agent`:

### 1. Multi-Turn Reasoning State Machine (`AgentRunner`)
The `AgentRunner` orchestrates the complete investigation lifecycle:
1. **Run Initialization**: Records the investigation run in PostgreSQL under tenant RLS (`WithOrgTx`), locks the prompt version ID, and broadcasts `run_started` over SSE.
2. **Dynamic Tool Schema Binding**: Discovers all authorized diagnostic tools via `ToolRegistry.ListAvailable(ctx, orgID)` and translates their JSON schemas into OpenAI-compatible `ToolDefinition` function declarations.
3. **Multi-Turn Step Loop**: Iterates through reasoning steps until the model outputs structured hypotheses or hits budget limits:
   - Evaluates accumulated token usage and USD cost against configured ceilings before every LLM call.
   - Dispatches model chat completions with accumulated conversation history and tool declarations.
   - If the model emits `tool_calls`, dispatches them concurrently or sequentially through `ToolRegistry.Execute`, logs the execution step to PostgreSQL `run_steps`, broadcasts `step_completed` over SSE, and appends the tool observation back to the conversation history.
   - If the model emits a final evaluation containing structured JSON hypotheses, parses and validates them via `ExtractHypothesesFromText`, records the `conclusion` step, and transitions the run to `succeeded`.
4. **Hypothesis & Status Persistence**: Persists all validated root-cause hypotheses into PostgreSQL (`hypotheses` table) under tenant RLS, updates the run's final tokens, cost, duration, and status, and broadcasts `run_completed` over SSE.

### 2. Multi-Dimensional Budget Enforcement (`AgentConfig`)
Investigations operate under strict, configurable guardrails:
- **`MaxSteps`**: Hard limit on reasoning turns (default: 8 steps). Prevents cyclic loops.
- **`MaxTokens`**: Cumulative prompt + completion token threshold (default: 25,000 tokens).
- **`MaxCostUSD`**: Cumulative monetary expenditure cap (default: $1.50 per run).
- **`RunTimeout`**: Overall context deadline (default: 90 seconds).
If any ceiling is reached, execution breaks immediately, recording status `budget_exhausted` and returning all intermediate telemetry gathered up to that point.

### 3. Evidence-Based Hypothesis Contract (`HypothesisDTO`)
The system prompt (`DefaultSystemPrompt`) enforces strict synthesis rules:
- **Ranked Structure**: 1-indexed ordering by probability.
- **Calibrated Confidence**:
  - `0.85 - 1.00`: Direct causality substantiated by matching error logs, deployment commit hashes, and metric spikes.
  - `0.60 - 0.84`: High circumstantial telemetry correlation.
  - `< 0.60`: Secondary possibility or inconclusive telemetry.
- **Evidence Citations**: Unseen or hallucinated values are strictly prohibited. Every claim must cite observed tool outputs.
- **Actionable Checklist**: Immediate verification steps and proposed safe remediations for human responders.

### 4. Step Trace Persistence & Real-Time SSE Broadcasting
- Every plan, tool execution, and conclusion is saved to `run_steps` with input/output payloads encoded as valid JSONB, token metrics, millisecond latency, and errors.
- Real-time updates are streamed via `sse.Broker` topics (`run_started`, `step_completed`, `run_completed`) allowing frontend responders to follow the live investigation trace as it unfolds.

### 5. Pluggable LLM Client Interface & Deterministic Reasoning Mock
- `LLMClient` interface:
  ```go
  type LLMClient interface {
      CreateChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error)
  }
  ```
- **`OpenAILLMClient`**: Production HTTP client supporting OpenAI-compatible chat completion endpoints, function calling, custom base URLs, configurable timeouts, token accounting, and cost calculation.
- **`MockLLMClient`**: Thread-safe deterministic multi-turn simulation:
  - Turn 1: Formulates plan -> calls `recent_deploys` and `query_metrics`.
  - Turn 2: Analyzes release -> calls `log_search` and `service_topology`.
  - Turn 3: Inspects dependencies -> calls `search_runbooks`.
  - Turn 4: Concludes investigation -> outputs formatted JSON hypotheses with confidence ratings and checklists.
  - Supports scripted turn injection and error injection for chaos and boundary testing.

### 6. Background Worker Integration
The worker process in `apps/api/cmd/worker/main.go` wires `AgentRunner` to the Redis Streams queue (`stream:investigations`). It manages bounded worker concurrency (`WorkerConcurrency`), graceful drain timeouts via `sync.WaitGroup`, and end-to-end investigation execution.

## Alternatives Considered

- **LangChain / Third-Party Agent Frameworks**:
  - *Rejected*: Third-party frameworks introduce excessive abstractions, heavy runtime overhead, lack native Go 1.23 support, and make multi-tenant PostgreSQL RLS context propagation error-prone.
- **Single-Shot Prompting (No Tool Loop)**:
  - *Rejected*: Ingested alerts lack deep diagnostic context. Single-shot prompts frequently hallucinate missing details. An autonomous multi-turn tool loop gathers ground-truth telemetry before synthesizing findings.
- **Direct Database Mutation by Agent**:
  - *Rejected*: The agent is strictly diagnostic and read-only. Remediation execution must always pass through human approval workflows.
