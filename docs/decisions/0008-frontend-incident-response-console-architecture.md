# ADR 0008: Frontend Incident Response Console Architecture

## Status
Accepted

## Context
In Phase 8, responders require a fast, high-density, real-time command console to monitor autonomous investigations, inspect diagnostic hypotheses, verify evidence citations, and execute Human-in-the-Loop (HITL) approved remediations.

Key design requirements:
1. **Modern Frontend Stack**: Built with SvelteKit 2 and Svelte 5 runes (`$state`, `$derived`, `$effect`) for performant reactivity without legacy store boilerplate or leaky state subscriptions.
2. **Real-Time Responsiveness**: Instant visibility into agent reasoning steps and remediation lifecycle updates via Server-Sent Events (SSE).
3. **Information Density & Visual Hierarchy**: Incident responders under high-stress conditions need clear severity badges, calibrated confidence meters, collapsible diagnostic payload viewers, and action checklists.
4. **Safety-First HITL Controls**: Explicit confirmation dialogs for destructive or mutating operations, clear risk tier indicators (`low`, `medium`, `high`, `critical`), TTL expiration countdowns, and documented rollback plans.
5. **Standalone Resilience**: Graceful degradation to a deterministic demo dataset when the API backend is unreachable or when previewing UI components standalone.

---

## Decision

We implement the Incident Response Console in `apps/web` utilizing SvelteKit 2, Svelte 5, and Tailwind CSS:

### 1. Svelte 5 Runes Reactivity Model
All components in `apps/web/src/lib/components/` and routes use Svelte 5 runes:
- `$state(...)`: Manages mutable reactive state (e.g., active incident, filter criteria, active tab, modal visibility, step traces).
- `$derived(...)`: Computes filtered incident lists, statistics (active counts, Sev1 counts, pending approvals), and sorted hypotheses without unnecessary re-renders.
- `$effect(...)`: Manages side effects, including SSE `EventSource` connections, TTL countdown timers, and window keyboard event bindings.

### 2. Component Architecture & Master-Detail Layout
The UI is structured into modular, single-responsibility components:
- `Navbar.svelte`: Live backend health indicator (ping latency, connected/offline status pill), active tenant switch badge, alert simulation trigger, and manual refresh controls.
- `StatsBar.svelte`: Global operational counters displaying active incidents, Sev1 critical count, autonomous investigation runs, and pending HITL approvals.
- `IncidentList.svelte`: Master column providing search-as-you-type, status tabs (`all`, `active`, `resolved`), severity indicators (Sev1-Sev4), elapsed time-ago stamps, and active selection state.
- `IncidentDetail.svelte`: Detail column coordinator containing header metadata, severity/status badge controls, tab bar switcher, and sub-view routing.
- `InvestigationTrace.svelte`: Live agent trace viewer presenting run budget metrics (tokens used, USD cost, duration), an active SSE pulse indicator, and expandable step-by-step diagnostic reasoning events with formatted JSON tool inputs/outputs.
- `HypothesesView.svelte`: Ranked root-cause hypotheses with calibrated confidence gauges, anti-hallucination evidence citations, and interactive first-response triage checklists.
- `RemediationsView.svelte`: Catalog of proposed remediations featuring action icons, risk tier pills, real-time TTL expiration countdowns, parameter inspection, rollback plans, approve/reject/rollback action buttons, and chronological audit event timeline.
- `RunbookViewer.svelte`: Integrated standard operating procedure (SOP) viewer with markdown formatting and copyable shell commands.
- `ApprovalModal.svelte`: Modal dialog enforcing human verification before remediation execution, allowing responder notes, rejection rationale capture, and manual remediation proposal.

### 3. Real-Time Streaming & Optimistic UI Updates
The console connects to the backend SSE endpoint (`/api/v1/incidents/{id}/stream`) when an incident is selected:
- Dispatched events (`run.step`, `run.completed`, `remediation.approved`, `remediation.executed`, `remediation.rejected`) automatically update local component state in real time.
- Optimistic updates ensure approve, reject, and rollback actions provide instant visual feedback while awaiting network confirmation.
- Safe reconnection and error handling prevent console freezing upon network interruption.

### 4. Resilient Demo Dataset & Production API Client
The TypeScript client (`apps/web/src/lib/api/client.ts`) is designed with fallback resilience:
- In production, it targets the backend REST and SSE endpoints with JWT authentication and tenant organization headers.
- If the backend is unavailable or running in standalone demo mode, the console seamlessly falls back to `apps/web/src/lib/demo-data.ts`, providing a fully interactive scenario with pre-populated incidents, agent reasoning traces, hypotheses, and remediations.

---

## Consequences

### Positive
- Responders have an integrated single-pane-of-glass incident dashboard.
- Zero external UI library bloat; clean, accessible Tailwind styling with dark-mode aesthetic.
- Strict TypeScript type safety matching the backend OpenAPI 3.1 specification.
- Full offline and demo capability for development, testing, and sales presentations.

### Negative
- Requires maintaining TypeScript types in sync with OpenAPI schema changes (automated via `make generate`).
