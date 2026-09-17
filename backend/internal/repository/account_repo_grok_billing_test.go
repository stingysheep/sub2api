package repository

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGrokBillingSnapshotIsSchedulerNeutral(t *testing.T) {
	t.Parallel()

	require.True(t, isSchedulerNeutralExtraKey("grok_billing_snapshot"))
	require.False(t, shouldEnqueueSchedulerOutboxForExtraUpdates(map[string]any{
		"grok_billing_snapshot": map[string]any{"usage_percent": 50},
	}))
}

func TestUpstreamBalanceRefreshFieldsAreSchedulerNeutral(t *testing.T) {
	t.Parallel()

	for _, key := range []string{"upstream_balance_snapshot", "upstream_balance_attempted_at"} {
		require.True(t, isSchedulerNeutralExtraKey(key))
		require.False(t, shouldEnqueueSchedulerOutboxForExtraUpdates(map[string]any{key: "value"}))
	}
}
