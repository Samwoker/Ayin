-- +goose Up
ALTER TABLE orgs ADD COLUMN IF NOT EXISTS webhook_secret TEXT NOT NULL DEFAULT 'dev_webhook_secret_default_32bytes';

-- Update demo orgs with dedicated secrets
UPDATE orgs SET webhook_secret = 'acme_webhook_secret_32bytes_hex_123' WHERE slug = 'acme';
UPDATE orgs SET webhook_secret = 'globex_webhook_secret_32bytes_hex_456' WHERE slug = 'globex';

-- +goose Down
ALTER TABLE orgs DROP COLUMN IF EXISTS webhook_secret;
