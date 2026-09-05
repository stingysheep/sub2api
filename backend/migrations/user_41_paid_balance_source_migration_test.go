package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUser41PaidBalanceSourceMigration(t *testing.T) {
	content, err := FS.ReadFile("237_correct_user_41_paid_balance_source.sql")
	require.NoError(t, err)
	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "WHERE id = 326 AND type = 'balance' AND used_by = 41")
	require.Contains(t, sql, "SET paid_cost = COALESCE(paid_cost, 0) + COALESCE(free_cost, 0), free_cost = 0")
	require.Contains(t, sql, "WHERE user_id = 41 AND COALESCE(free_cost, 0) <> 0")
	require.Contains(t, sql, "SET free_balance_issued = 0 WHERE id = 41")
}
