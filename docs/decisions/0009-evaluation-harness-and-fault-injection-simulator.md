# ADR 0009: Benchmark Evaluation Harness and Fault-Injection Simulator Architecture

## Status
Accepted

## Context
As the autonomous triage agent expands in capability, evaluating prompt modifications, tool integrations, and LLM reasoning models requires a rigorous, reproducible, and objective benchmarking framework. Manual spot-checking is insufficient for high-stakes incident triage:
1. **Anti-Hallucination & Evidence Precision**: The agent must never fabricate error messages, invent unobserved metrics, or cite diagnostic tools it never executed.
2. **Ground-Truth Calibration**: Hypotheses must match documented failure modes across realistic cloud-native outage topologies (message queues, connection pools, memory leaks, third-party timeouts, cache partition buffer overflows).
3. **Regression Prevention in CI**: Gating pull requests on quantitative performance metrics requires deterministic, offline execution with zero reliance on paid external APIs or flaky network dependencies.
4. **Chaos Resilience & Noise Distraction**: Production telemetry contains background noise, transient socket warnings, and concurrent unrelated releases. The agent must be tested against adversarial distractors to verify it does not pursue red herrings.

---

## Decision

We implement an Evaluation Harness (`apps/api/internal/eval` and `cmd/eval`) and a Fault-Injection Simulator (`apps/api/internal/simulate` and `cmd/simulate`) backed by declarative benchmark scenarios (`scenarios/*.yaml`):

### 1. Benchmark Scenarios Specification (`scenarios/*.yaml`)
Scenarios define self-contained incident failure fixtures:
- **`rabbitmq-backlog.yaml`**: RabbitMQ prefetch change in release v2.14.0 causing unacked message accumulation, worker thread starvation, and order lock deadlocks.
- **`postgres-pool-exhaustion.yaml`**: Unbounded reconciliation query holding idle in transaction connections, saturating all 100 client connection slots.
- **`bad-deployment-oom.yaml`**: Release v3.8.1 in-memory map leak triggering container OOMKilled exit code 137 and CrashLoopBackOff.
- **`downstream-gateway-timeout.yaml`**: External Stripe gateway 30s latency spike causing HTTP thread pool starvation and circuit breaker trip.
- **`redis-replica-flap.yaml`**: Mass cache eviction burst exceeding `client-output-buffer-limit slave`, terminating replication link and triggering resync storms.

Each scenario provides:
- **Mock Environment**: Diagnostic fixtures for `recent_deploys`, `query_metrics`, `log_search`, `service_topology`, `search_runbooks`, and `similar_incidents`.
- **Ground Truth Criteria**: Expected root-cause patterns, forbidden hallucinated causes, minimum confidence thresholds, required evidence citations, mandatory diagnostic tools, and expected remediation action.

### 2. Multi-Dimensional Scoring Engine (`internal/eval/scorer.go`)
Evaluation scores agent investigation results along 6 calibrated axes:
1. **Hypothesis Accuracy Score (35% weight)**: Verifies whether the top-ranked hypothesis identifies the ground-truth mechanism of failure and penalizes forbidden claims.
2. **Evidence Citation Score (25% weight)**: Verifies direct quotation of observed telemetry values and parameters without missing key facts.
3. **Tool Economy Score (15% weight)**: Assesses whether mandatory tools were queried without wasteful or looping diagnostic steps.
4. **Remediation Alignment Score (15% weight)**: Assesses whether the proposed checklist or remediation action directly resolves the verified root cause.
5. **Confidence Calibration Score (10% weight)**: Evaluates whether confidence ratings reflect evidentiary strength without overconfidence on ambiguous traces.
6. **Hallucination Penalty (-20% deduction)**: Strictly penalizes citations to uncalled tools or unobserved server failures.

Scenarios pass when `OverallScore >= 80.0%`, `AccuracyScore >= 80.0%`, `EvidenceScore >= 60.0%`, `HallucinationScore < 20.0%`, and no fatal errors occur.

### 3. Baseline Comparison & CI Regression Gating (`internal/eval/comparator.go`)
- The evaluation CLI (`cmd/eval`) supports `--compare=<baseline.json>` and `--save-baseline=<baseline.json>`.
- Generates deltas for Pass Rate, Accuracy, Mean Score, Token Consumption, and USD Cost.
- Detects and flags any scenario regression (scenarios passing in baseline that now fail), exiting with non-zero code to fail CI.

### 4. Deterministic Fault-Injection Simulator (`internal/simulate`)
The simulator (`cmd/simulate`) enables deterministic chaos fault injection:
- **Chaos Levels**:
  - `0 (Clean Trace)`: Nominal sequence of deploy -> metric surge -> error logs -> firing alert.
  - `1 (Background Noise)`: Injects realistic ambient logs (GC pauses, health check probes, routine metrics).
  - `2 (Transient Jitter)`: Injects connection retries and latency blips.
  - `3 (Adversarial Red Herrings)`: Injects concurrent benign deployments on adjacent microservices and misleading TLS warning logs to test agent resilience against false correlation.
- **Execution Modes**:
  - `Dry-Run`: Outputs a formatted terminal trace table with timestamp, classification, and event summary.
  - `Live Webhook Dispatch`: POSTs alerts to the platform ingestion endpoint with HMAC-SHA256 signature verification.

---

## Consequences

### Positive
- 100% reproducible benchmark suite running in under 5ms per scenario in offline mode.
- Comprehensive coverage of complex failure modes across cloud-native architectures.
- Immediate detection of prompt degradation or hallucination regressions via CI gating.
- Simulator provides reliable demo and end-to-end load testing capabilities.

### Negative
- Adding new system diagnostic tools requires updating scenario environment schemas.
