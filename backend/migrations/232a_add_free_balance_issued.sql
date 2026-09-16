-- Track the cumulative amount of free balance issued to each user.
--
-- This column was added to the Ent schema together with the split free/paid
-- balance fields, but older SQL-only installations never received the matching
-- DDL.  Keep the backfill inside the column-missing branch so databases that
-- already track the counter retain their historical value.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'users'
          AND column_name = 'free_balance_issued'
    ) THEN
        ALTER TABLE users
            ADD COLUMN free_balance_issued NUMERIC(20,8) NOT NULL DEFAULT 0;

        UPDATE users
        SET free_balance_issued = GREATEST(free_balance, 0);
    END IF;
END
$$;
