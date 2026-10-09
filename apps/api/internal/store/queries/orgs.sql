-- name: CreateOrg :one
INSERT INTO orgs (name, slug)
VALUES ($1, $2)
RETURNING *;

-- name: GetOrgByID :one
SELECT * FROM orgs
WHERE id = $1;

-- name: GetOrgBySlug :one
SELECT * FROM orgs
WHERE slug = $1;

-- name: ListOrgs :many
SELECT * FROM orgs
ORDER BY created_at DESC;

-- name: GetOrgByStripeCustomerID :one
SELECT * FROM orgs
WHERE stripe_customer_id = $1;

-- name: GetOrgByStripeSubscriptionID :one
SELECT * FROM orgs
WHERE stripe_subscription_id = $1;

-- name: UpdateOrgSubscription :one
UPDATE orgs
SET plan_tier = $2,
    stripe_customer_id = $3,
    stripe_subscription_id = $4,
    subscription_status = $5,
    current_period_end = $6,
    monthly_runs_limit = $7,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: IncrementOrgMonthlyRunsUsed :one
UPDATE orgs
SET monthly_runs_used = monthly_runs_used + 1,
    updated_at = now()
WHERE id = $1
RETURNING monthly_runs_used;

-- name: ResetOrgMonthlyRuns :one
UPDATE orgs
SET monthly_runs_used = 0,
    monthly_runs_reset_at = now(),
    updated_at = now()
WHERE id = $1
RETURNING *;
