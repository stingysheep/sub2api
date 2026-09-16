//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type authBalanceVersionRepo struct {
	UserRepository
	balance float64
	version int64
	err     error
	calls   int
}

func (r *authBalanceVersionRepo) GetUserBalanceVersion(context.Context, int64) (float64, int64, error) {
	r.calls++
	return r.balance, r.version, r.err
}

func TestAPIKeyService_OperatorBalanceAuthorityOverridesEveryAuthPath(t *testing.T) {
	for _, path := range []string{"L2", "L1", "singleflight", "lookup"} {
		t.Run(path, func(t *testing.T) {
			entry := &APIKeyAuthCacheEntry{Snapshot: &APIKeyAuthSnapshot{
				Version: apiKeyAuthSnapshotVersion, APIKeyID: 1, UserID: 2, Status: StatusActive,
				User: APIKeyAuthUserSnapshot{ID: 2, Role: RoleUser, Status: StatusActive, Balance: 100},
			}}
			original := &APIKey{ID: 1, UserID: 2, Status: StatusActive, User: &User{ID: 2, Balance: 100}}
			repo := &authRepoStub{getByKeyForAuth: func(context.Context, string) (*APIKey, error) { return original, nil }}
			cache := &authCacheStub{getAuthCache: func(context.Context, string) (*APIKeyAuthCacheEntry, error) { return entry, nil }}
			cfg := &config.Config{APIKeyAuth: config.APIKeyAuthCacheConfig{L2TTLSeconds: 60}}
			if path == "L1" {
				cfg.APIKeyAuth.L1Size = 1000
				cfg.APIKeyAuth.L1TTLSeconds = 60
			}
			if path == "singleflight" {
				cfg.APIKeyAuth.Singleflight = true
				cfg.APIKeyAuth.L2TTLSeconds = 0
			}
			if path == "lookup" {
				cfg.APIKeyAuth.L2TTLSeconds = 0
			}
			authority := &authBalanceVersionRepo{balance: 0, version: 1}
			svc := NewAPIKeyService(repo, authority, nil, nil, nil, cache, cfg)
			got, err := svc.GetByKey(context.Background(), "synthetic-key")
			require.NoError(t, err)
			require.Zero(t, got.User.Balance)
			require.Equal(t, float64(100), entry.Snapshot.User.Balance)
			require.Equal(t, float64(100), original.User.Balance)
			if path == "L1" {
				require.NotNil(t, svc.authCacheL1)
				svc.authCacheL1.Wait()
				_, cached := svc.authCacheL1.Get(svc.authCacheKey("synthetic-key"))
				require.True(t, cached, "second call must exercise the populated L1")
				cache.getAuthCache = func(context.Context, string) (*APIKeyAuthCacheEntry, error) {
					t.Fatal("L1 hit unexpectedly queried L2")
					return nil, nil
				}
			}
			// No invalidate after the next commit; cached snapshots stay unchanged.
			authority.balance, authority.version = 5, 2
			got, err = svc.GetByKey(context.Background(), "synthetic-key")
			require.NoError(t, err)
			require.Equal(t, float64(5), got.User.Balance)
			require.Equal(t, 2, authority.calls)
			authority.err = errors.New("primary offline")
			got, err = svc.GetByKey(context.Background(), "synthetic-key")
			require.Nil(t, got)
			require.ErrorContains(t, err, "primary offline")
		})
	}
}
