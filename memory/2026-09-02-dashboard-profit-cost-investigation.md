# Dashboard and profit cost investigation

- Symptom: dashboard orange costs were much larger than expected, and profit analysis did not visibly account for more than 6 units of consumed free credit.
- Root cause 1: role-scoped dashboard reads used the historical `account_rate_multiplier` snapshot, while the agreed profitability basis recalculates `COALESCE(account_stats_cost, total_cost)` with the account's current multiplier.
- Root cause 2: all 795 ordinary-user balance requests were attributed correctly, but the profit page omitted `total_free_balance_cost`. It showed only the corresponding upstream cost. The default current-month filter also excluded 5.946201 units consumed in August.
- Production evidence at investigation time: lifetime free credit consumed 6.293453; September consumed 0.347252; September corresponding upstream cost 0.192763; zero allocation joins missing. Current-rate admin upstream cost was 15.920024 today and 71.929092 lifetime.
- Fix: dashboard range/role aggregation now joins current accounts for the orange cost. Profit analysis separates lifetime issued, lifetime consumed, selected-period consumed, and selected-period upstream cost.
- Verification: repository unit regression passed; ProfitView regression passed; frontend typecheck and production build passed; candidate image `sub2api:codex-v0.1.185-shep.10` is healthy on local port 18084 and returns the new API fields. Full frontend suite had 1803 passes and 14 unrelated pre-existing failures in GroupsView Pinia setup and route timing. Full backend run had one service-package flake; isolated service rerun passed.
- Release verification: deployed `sub2api:codex-v0.1.185-shep.10` with matching local/remote image ID. Container healthy with zero restarts; internal and external health endpoints returned 200; public page loaded in about 0.67 s with zero console errors. PostgreSQL, Redis, and Caddy were not rebuilt.
- Status: fixed, deployed, and verified.
