ALTER TABLE users
    ADD COLUMN IF NOT EXISTS free_balance NUMERIC(20,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS paid_balance NUMERIC(20,8) NOT NULL DEFAULT 0;

UPDATE users
SET free_balance = balance, paid_balance = 0
WHERE COALESCE(free_balance, 0) = 0
  AND COALESCE(paid_balance, 0) = 0
  AND balance <> 0;

CREATE TABLE IF NOT EXISTS usage_balance_allocations (
    request_id TEXT NOT NULL,
    api_key_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    free_cost NUMERIC(20,10) NOT NULL DEFAULT 0,
    paid_cost NUMERIC(20,10) NOT NULL DEFAULT 0,
    unfunded_cost NUMERIC(20,10) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (request_id, api_key_id)
);

CREATE INDEX IF NOT EXISTS idx_usage_balance_allocations_user_created
    ON usage_balance_allocations (user_id, created_at);

INSERT INTO usage_balance_allocations (request_id, api_key_id, user_id, free_cost, paid_cost, unfunded_cost, created_at)
SELECT ul.request_id, ul.api_key_id, ul.user_id, GREATEST(ul.actual_cost, 0), 0, 0, ul.created_at
FROM usage_logs ul
WHERE ul.request_id IS NOT NULL AND ul.request_id <> ''
  AND ul.billing_type = 0 AND ul.actual_cost > 0
ON CONFLICT (request_id, api_key_id) DO NOTHING;
