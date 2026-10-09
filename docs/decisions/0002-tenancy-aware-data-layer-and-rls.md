# ADR 0002: Tenancy-Aware Data Layer and PostgreSQL Row-Level Security (RLS)

## Status
Accepted

## Context
`triage` is built from day one as a multi-tenant platform serving multiple engineering organizations. Critical telemetry, investigation traces, root-cause hypotheses, runbooks, and remediation proposals must be strictly isolated between organizations. A data leak across tenant boundaries is unacceptable.

We evaluated architectural patterns for multi-tenant database isolation:
1. Shared database, shared schema with application-level filtering (`org_id` WHERE clauses).
2. Shared database, separate schema per tenant (`CREATE SCHEMA org_...`).
3. Shared database, shared schema enforced at the database engine level via PostgreSQL Row-Level Security (RLS).
4. Separate database per tenant.

## Decision
We choose **Shared Database, Shared Schema with PostgreSQL Row-Level Security (RLS)** and composite indexes:
- Every tenant table contains an `org_id UUID NOT NULL REFERENCES orgs(id)` column.
- Row-Level Security is enabled on every tenant table with policies enforcing:
  `USING (org_id = current_setting('app.org_id', true)::uuid)`
  `WITH CHECK (org_id = current_setting('app.org_id', true)::uuid)`
- The application executes tenant operations inside scoped transactions via `Store.WithOrgTx`, setting `SET LOCAL app.org_id = '<tenant-uuid>'`.
- All `sqlc` queries explicitly include `WHERE org_id = $1` as defense-in-depth.
- All composite indexes begin with `org_id` (e.g., `(org_id, status)`, `(org_id, opened_at)`).

## Alternatives Considered
- **Schema-per-tenant**:
  - *Pros*: Natural physical namespace separation.
  - *Cons*: Severe migration overhead (running migrations hundreds of times), connection pool exhaustion, lack of cross-tenant aggregate analytics for platform admins, and schema proliferation limits.
- **Application-only filtering (no RLS)**:
  - *Pros*: Simple setup.
  - *Cons*: Vulnerable to catastrophic accidental leaks if a developer or agent forgets a single `WHERE org_id = ...` clause in a new query or raw query.
- **Database-per-tenant**:
  - *Pros*: Complete physical isolation.
  - *Cons*: Expensive, slow provisioning for new tenants, high infrastructure maintenance for SaaS/hackathon scale.

## Consequences
- **Positive**: Defense-in-depth; even if an application bug omits `org_id`, the PostgreSQL engine refuses to return or mutate rows belonging to other tenants.
- **Positive**: Schema migrations run once via Goose across all tenants.
- **Positive**: Composite indexes `(org_id, ...)` guarantee query performance scales linearly per tenant.
- **Trade-off**: Requires database connections to use `SET LOCAL app.org_id` within transactions for tenant queries. Background maintenance workers and global jobs must explicitly use elevated/admin roles or pass the target `org_id`.
