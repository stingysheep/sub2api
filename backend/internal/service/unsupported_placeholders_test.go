package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUnsupportedCredentialAndProxyProbesFailExplicitly(t *testing.T) {
	ctx := context.Background()
	err := (&AccountService{}).TestCredentials(ctx, 1)
	require.ErrorContains(t, err, "unsupported")
	err = (&ProxyService{}).TestConnection(ctx, 1)
	require.ErrorContains(t, err, "unsupported")
}

func TestRefreshAccountCredentialsFailsExplicitly(t *testing.T) {
	_, err := (*adminServiceImpl)(nil).RefreshAccountCredentials(context.Background(), 1)
	require.ErrorContains(t, err, "unsupported")
}
