package ent_test

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/ent/redeemcode"
	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/stretchr/testify/require"
)

func TestRedeemCodeBalanceSourceValidatorIsInitialized(t *testing.T) {
	require.NotNil(t, redeemcode.BalanceSourceValidator)
	require.NoError(t, redeemcode.BalanceSourceValidator("free"))
	require.NoError(t, redeemcode.BalanceSourceValidator("paid"))
	require.Error(t, redeemcode.BalanceSourceValidator("unknown"))
}
