# 13. Google Gemini AI Engine, Stripe Billing Subscriptions, and Tiered Rate Limiting

Date: 2026-10-09

## Status
Accepted

## Context
The autonomous incident response engine required two foundational enhancements:
1. **AI Engine Provider**: Support for Google Gemini (`gemini-2.5-flash` for high-throughput, cost-efficient triage and `gemini-2.5-pro` for deep root-cause synthesis), featuring native function calling, system instructions, and token cost accounting.
2. **SaaS Monetization & Quota Architecture**: A tiered subscription model featuring a Free plan ($0/mo) and paid tiers (Pro at $49/mo, Enterprise at $299+/mo) processed via Stripe Checkout and Customer Portal, gated by tenant-aware API rate limits and monthly investigation run quotas.

## Decision

### 1. Google Gemini AI Client (`apps/api/internal/agent/gemini_client.go`)
- Implemented `GeminiLLMClient` complying with the `LLMClient` contract (`Complete(ctx, req) (*LLMResponse, error)`).
- Interacts with Google Gemini v1beta REST API (`/models/{model}:generateContent`).
- Translates `RoleSystem` to Gemini `systemInstruction`, `RoleUser` to `user`, and `RoleAssistant` to `model`.
- Supports multi-turn function calling with tool schemas mapped to Gemini's `FunctionDeclarations`, returning `ToolCalls` and handling `FunctionResponse` observations.
- Token cost tracking:
  - `gemini-2.5-flash`: $0.075 / 1M prompt tokens, $0.30 / 1M candidate tokens.
  - `gemini-2.5-pro`: $1.25 / 1M prompt tokens, $5.00 / 1M candidate tokens.
- Worker (`apps/api/cmd/worker/main.go`) and evaluation harness (`runner.go`) dynamically instantiate `GeminiLLMClient` when `LLM_PROVIDER=gemini` or `GEMINI_API_KEY` is provided.

### 2. Multi-Tenant Database Migration (`00003_add_subscriptions_and_plans.sql`)
- Augmented the `orgs` table with subscription attributes:
  - `plan_tier VARCHAR(32) NOT NULL DEFAULT 'free'` (`free`, `pro`, `enterprise`).
  - `stripe_customer_id VARCHAR(255)` & `stripe_subscription_id VARCHAR(255)` (indexed).
  - `subscription_status VARCHAR(32) NOT NULL DEFAULT 'active'`.
  - `current_period_end TIMESTAMPTZ`.
  - `monthly_runs_limit INT NOT NULL DEFAULT 10`.
  - `monthly_runs_used INT NOT NULL DEFAULT 0`.
  - `monthly_runs_reset_at TIMESTAMPTZ NOT NULL DEFAULT now()`.
- Added queries in `orgs.sql`: `GetOrgByStripeCustomerID`, `GetOrgByStripeSubscriptionID`, `UpdateOrgSubscription`, `IncrementOrgMonthlyRunsUsed`, and `ResetOrgMonthlyRuns`.

### 3. Stripe Billing Service (`apps/api/internal/billing/`)
- Implemented a zero-dependency HTTP Stripe client and webhook signature verifier (`service.go`):
  - `CreateCheckoutSession`: Generates Stripe Checkout URLs for subscription initiation (`mode=subscription`, line items, client reference ID).
  - `CreatePortalSession`: Generates Stripe Customer Portal URLs for subscription, card, and invoice management.
  - `HandleWebhook`: Verifies Stripe HMAC-SHA256 signatures (`Stripe-Signature: t=...,v1=...`) with timestamp tolerance and processes events:
    - `checkout.session.completed`: Upgrades tenant to Pro tier (250 runs limit).
    - `customer.subscription.updated`: Synchronizes status changes (`active`, `past_due`, `canceled`).
    - `customer.subscription.deleted`: Downgrades tenant to Free tier (10 runs limit).
    - `invoice.payment_succeeded`: Resets monthly runs used on renewal cycles.
  - Test/Mock fallback: Automatically simulates checkout and portal redirects without requiring external network connectivity when running offline.

### 4. Tiered Rate Limiting & Quota Guard
- **Tier-Adaptive Rate Limiter** (`apps/api/internal/server/middleware/ratelimit.go`):
  - Free: 10 req/min (capacity 20 burst).
  - Pro: 60 req/min (capacity 100 burst).
  - Enterprise: 300 req/min (capacity 500 burst).
  - Sets `X-RateLimit-Limit`, `X-RateLimit-Remaining`, and `X-RateLimit-Tier` response headers.
  - Returns `429 Too Many Requests` with tier details and `/billing` upgrade recommendation.
- **Investigation Run Quota Guard** (`apps/api/internal/server/handlers/runs.go`):
  - Checks `monthly_runs_used < monthly_runs_limit` on `POST /incidents/{id}/runs`.
  - When exceeded, rejects run creation with `402 Payment Required` (`code: QUOTA_EXCEEDED`) and prompts user to upgrade to Pro.

### 5. Frontend Billing & Plans Management (`apps/web/`)
- Created `PricingModal.svelte`: Accessible dialog comparing Free ($0/mo), Pro ($49/mo), and Enterprise ($299/mo), displaying quota usage progress, Gemini AI engine highlights, and direct buttons for Stripe Checkout and Stripe Customer Portal.
- Updated `Navbar.svelte`: Displays tenant plan badge (`PRO` / `FREE`) with live run usage counter and an "Upgrade" button.
- Updated `+page.svelte`: Syncs subscription state on mount, opens `PricingModal` on 402 quota errors, and handles Stripe redirect callbacks.

## Consequences
- **Self-Sustaining SaaS Model**: Free tier costs less than $0.03/user/month on Gemini 2.5 Flash while providing conversion pathways to Pro ($49/mo).
- **Security & Integrity**: Webhook signatures strictly validated via constant-time HMAC comparison. Quotas enforced in database transactions.
- **Zero-Dependency Resilience**: Stripe REST client and signature verifier use Go standard library without multi-megabyte SDK bloat.
- **Quality Gates**: All Go and Web test suites (`make test`), linters (`make lint`), and production builds (`make build`) pass with code 0.
