-- Track whether a balance redeem code represents paid or free credit.
-- Existing rows are backfilled conservatively from the owner's confirmed history:
--   * used balance codes are free, except the confirmed paid 2026-09-03 01:47:53 admin recharge;
--   * unused balance codes are paid, except the five most recently created $5 codes;
--   * admin adjustment records already carry their source in notes and are left unchanged.
ALTER TABLE redeem_codes
    ADD COLUMN IF NOT EXISTS balance_source VARCHAR(10) NOT NULL DEFAULT 'free';

UPDATE redeem_codes
SET balance_source = 'free'
WHERE type = 'balance';

UPDATE redeem_codes
SET balance_source = 'paid'
WHERE type = 'balance' AND status = 'unused';

WITH recent_free_codes AS (
    SELECT id
    FROM redeem_codes
    WHERE type = 'balance'
      AND status = 'unused'
      AND value = 5
    ORDER BY created_at DESC, id DESC
    LIMIT 5
)
UPDATE redeem_codes
SET balance_source = 'free'
WHERE id IN (SELECT id FROM recent_free_codes);

UPDATE redeem_codes
SET balance_source = 'paid'
WHERE type = 'admin_balance'
  AND notes ILIKE '%[balance_source=paid]%';

-- The owner confirmed this specific 30-dollar manual recharge was paid.
-- Match its business timestamp and amount rather than relying on the current
-- row id, so the migration remains portable across restored databases.
UPDATE redeem_codes
SET balance_source = 'paid'
WHERE type = 'admin_balance'
  AND value = 30
  AND used_at = TIMESTAMPTZ '2026-09-03 01:47:53+08';

UPDATE redeem_codes
SET balance_source = 'free'
WHERE type = 'admin_balance'
  AND notes ILIKE '%[balance_source=free]%';
