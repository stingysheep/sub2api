-- User 41 confirmed that redeem code 326 was paid credit, not free credit.
-- Keep the user's total balance unchanged; only correct source attribution in
-- the code, historical allocation rows, and issued-free counter.
UPDATE redeem_codes
SET balance_source = 'paid'
WHERE id = 326
  AND type = 'balance'
  AND used_by = 41;

UPDATE usage_balance_allocations
SET paid_cost = COALESCE(paid_cost, 0) + COALESCE(free_cost, 0),
    free_cost = 0
WHERE user_id = 41
  AND COALESCE(free_cost, 0) <> 0;

UPDATE users
SET free_balance_issued = 0
WHERE id = 41;
