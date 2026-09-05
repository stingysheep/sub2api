# Ent Redeem Balance Source Panic

- Date: 2026-09-04
- Scope: project `shep-server-ops`, production `sub2api`
- Status: resolved and deployed

## Symptom

Production `0.2.0-shep.2` recovered panics when the administrator balance adjustment flow created its audit redeem-code record. The application container stayed healthy, but the adjustment endpoint could not complete its audit write.

## Root Cause

The generated Ent create/update builders called `redeemcode.BalanceSourceValidator`, while `backend/ent/schema/redeem_code.go` declared the field without `Validate`. Ent runtime generation therefore initialized the default value but left the validator function nil. The create builder dereferenced that nil function.

## Fix

Declared the schema validator allowing only `free` and `paid`, regenerated the relevant Ent runtime bindings, and restored unrelated files accidentally rewritten by the local generator environment. Added schema and runtime regression tests in `backend/ent/schema/redeem_code_schema_test.go` and `backend/ent/redeemcode_runtime_test.go`.

## Evidence

- Ent schema/runtime tests passed under Go 1.27.
- Production image `sub2api:codex-v0.2.0-shep.3` built successfully and reported the expected version.
- Production container is healthy with restart count 0.
- Internal and both public HTTPS health endpoints returned HTTP 200.
- No `panic recovered` entries appeared after the `.3` restart window.
- Pre-release PostgreSQL backup: `/opt/sub2api/backups/postgres/sub2api-20260904-000138-before-shep3.sql`, SHA256 `7db3c60cc2aa5acc8233c91fc25cbf1b2ab6cfe8c89159c7c1e308aad2c31e70`.

## Residual Risk

The full `internal/service` test package remains blocked by pre-existing reasoning effort test calls using older function signatures. The application image itself compiled successfully; this unrelated test baseline should be repaired separately.
