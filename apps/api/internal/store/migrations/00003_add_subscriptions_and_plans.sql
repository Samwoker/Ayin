-- +goose Up
ALTER TABLE orgs ADD COLUMN IF NOT EXISTS plan_tier VARCHAR(32) NOT NULL DEFAULT 'free';
ALTER TABLE orgs ADD COLUMN IF NOT EXISTS stripe_customer_id VARCHAR(255);
ALTER TABLE orgs ADD COLUMN IF NOT EXISTS stripe_subscription_id VARCHAR(255);
ALTER TABLE orgs ADD COLUMN IF NOT EXISTS subscription_status VARCHAR(32) NOT NULL DEFAULT 'active';
ALTER TABLE orgs ADD COLUMN IF NOT EXISTS current_period_end TIMESTAMPTZ;
ALTER TABLE orgs ADD COLUMN IF NOT EXISTS monthly_runs_limit INT NOT NULL DEFAULT 10;
ALTER TABLE orgs ADD COLUMN IF NOT EXISTS monthly_runs_used INT NOT NULL DEFAULT 0;
ALTER TABLE orgs ADD COLUMN IF NOT EXISTS monthly_runs_reset_at TIMESTAMPTZ NOT NULL DEFAULT now();

CREATE INDEX IF NOT EXISTS idx_orgs_stripe_customer ON orgs (stripe_customer_id);
CREATE INDEX IF NOT EXISTS idx_orgs_stripe_subscription ON orgs (stripe_subscription_id);

-- Update demo orgs (acme can have 'pro' tier for demonstration with 250 runs limit, globex on 'free' with 10)
UPDATE orgs SET plan_tier = 'pro', monthly_runs_limit = 250, monthly_runs_used = 12 WHERE slug = 'acme';
UPDATE orgs SET plan_tier = 'free', monthly_runs_limit = 10, monthly_runs_used = 2 WHERE slug = 'globex';

-- +goose Down
DROP INDEX IF EXISTS idx_orgs_stripe_subscription;
DROP INDEX IF EXISTS idx_orgs_stripe_customer;
ALTER TABLE orgs DROP COLUMN IF EXISTS monthly_runs_reset_at;
ALTER TABLE orgs DROP COLUMN IF EXISTS monthly_runs_used;
ALTER TABLE orgs DROP COLUMN IF EXISTS monthly_runs_limit;
ALTER TABLE orgs DROP COLUMN IF EXISTS current_period_end;
ALTER TABLE orgs DROP COLUMN IF EXISTS subscription_status;
ALTER TABLE orgs DROP COLUMN IF EXISTS stripe_subscription_id;
ALTER TABLE orgs DROP COLUMN IF EXISTS stripe_customer_id;
ALTER TABLE orgs DROP COLUMN IF EXISTS plan_tier;
