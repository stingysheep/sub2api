-- Reconcile the three group fields historically added outside the migration chain.
-- Existing Fast flags and over-limit policies must survive upgrades and replay.
ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS force_openai_fast BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS free_openai_fast BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS max_reasoning_effort_over_limit VARCHAR(20) NOT NULL DEFAULT 'downgrade';

-- Older manually provisioned columns may be nullable or lack the Ent defaults.
UPDATE groups SET force_openai_fast = FALSE WHERE force_openai_fast IS NULL;
UPDATE groups SET free_openai_fast = FALSE WHERE free_openai_fast IS NULL;
UPDATE groups SET max_reasoning_effort_over_limit = 'downgrade'
WHERE max_reasoning_effort_over_limit IS NULL;

ALTER TABLE groups
    ALTER COLUMN force_openai_fast SET DEFAULT FALSE,
    ALTER COLUMN force_openai_fast SET NOT NULL,
    ALTER COLUMN free_openai_fast SET DEFAULT FALSE,
    ALTER COLUMN free_openai_fast SET NOT NULL,
    ALTER COLUMN max_reasoning_effort_over_limit SET DEFAULT 'downgrade',
    ALTER COLUMN max_reasoning_effort_over_limit SET NOT NULL;
