-- An operator adjustment commits this generation with its balance and receipt.
-- Normal usage billing retains its existing amount and cache-deduction rules.
ALTER TABLE users ADD COLUMN IF NOT EXISTS operator_balance_cache_version bigint NOT NULL DEFAULT 0;
