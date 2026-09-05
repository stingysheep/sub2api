package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRedeemCodeBalanceSourceMigration(t *testing.T) {
	content, err := FS.ReadFile("236_redeem_code_balance_source.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS balance_source VARCHAR(10) NOT NULL DEFAULT 'free'")
	require.Contains(t, sql, "UPDATE redeem_codes SET balance_source = 'paid' WHERE type = 'balance' AND status = 'unused'")
	require.Contains(t, sql, "ORDER BY created_at DESC, id DESC LIMIT 5")
	require.Contains(t, sql, "value = 30 AND used_at = TIMESTAMPTZ '2026-09-03 01:47:53+08'")
	require.Contains(t, sql, "notes ILIKE '%[balance_source=paid]%'")
	require.Contains(t, sql, "notes ILIKE '%[balance_source=free]%'")
}
