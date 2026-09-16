//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type authFinancialRepo struct {
	*authRepoStub
	state *APIKeyFinancialState
	err   error
	calls int
}

func (r *authFinancialRepo) GetAPIKeyFinancialState(context.Context, int64, int64) (*APIKeyFinancialState, error) {
	r.calls++
	return r.state, r.err
}

func TestAPIKeyService_FinancialAuthorityTwoInstancesCopiesAndDoesNotRevive(t *testing.T) {
	for _, path := range []string{"L2", "L1", "no_redis", "redis_failed", "singleflight"} {
		t.Run(path, func(t *testing.T) {
			original := &APIKey{ID: 3, UserID: 2, Quota: 100, Status: StatusActive, User: &User{ID: 2, Role: RoleUser, Status: StatusActive, Balance: 5}}
			entry := &APIKeyAuthCacheEntry{Snapshot: &APIKeyAuthSnapshot{Version: apiKeyAuthSnapshotVersion, APIKeyID: 3, UserID: 2, Quota: 100, Status: StatusActive, User: APIKeyAuthUserSnapshot{ID: 2, Role: RoleUser, Status: StatusActive, Balance: 5}}}
			cache := &authCacheStub{getAuthCache: func(context.Context, string) (*APIKeyAuthCacheEntry, error) { return entry, nil }}
			cfg := &config.Config{APIKeyAuth: config.APIKeyAuthCacheConfig{L2TTLSeconds: 60}}
			if path == "L1" {
				cfg.APIKeyAuth.L1Size = 1000
				cfg.APIKeyAuth.L1TTLSeconds = 60
			}
			if path == "no_redis" {
				cfg.APIKeyAuth.L2TTLSeconds = 0
			}
			if path == "singleflight" {
				cfg.APIKeyAuth.L2TTLSeconds = 0
				cfg.APIKeyAuth.Singleflight = true
			}
			if path == "redis_failed" {
				cache.getAuthCache = func(context.Context, string) (*APIKeyAuthCacheEntry, error) {
					return nil, errors.New("synthetic Redis failed")
				}
			}
			state := &APIKeyFinancialState{Balance: 5, Quota: 1, Status: StatusActive}
			repo := &authFinancialRepo{authRepoStub: &authRepoStub{getByKeyForAuth: func(context.Context, string) (*APIKey, error) { return original, nil }}, state: state}
			legacy := &authBalanceVersionRepo{balance: 999}
			first := NewAPIKeyService(repo, legacy, nil, nil, nil, cache, cfg)
			second := NewAPIKeyService(repo, legacy, nil, nil, nil, cache, cfg)
			for _, svc := range []*APIKeyService{first, second} {
				got, err := svc.GetByKey(context.Background(), "synthetic-financial")
				require.NoError(t, err)
				require.Equal(t, 1.0, got.Quota)
				require.Equal(t, 5.0, got.User.Balance)
				if svc.authCacheL1 != nil {
					svc.authCacheL1.Wait()
				}
			}
			state.Balance, state.QuotaUsed, state.Status = 4, 1, StatusAPIKeyQuotaExhausted // recovered PG debit; no invalidate delivered
			for _, svc := range []*APIKeyService{first, second} {
				got, err := svc.GetByKey(context.Background(), "synthetic-financial")
				require.NoError(t, err)
				require.Equal(t, 1.0, got.QuotaUsed)
				require.Equal(t, StatusAPIKeyQuotaExhausted, got.Status)
				require.True(t, got.IsQuotaExhausted())
			}
			state.Status = "disabled"
			state.QuotaUsed = 0
			got, err := first.GetByKey(context.Background(), "synthetic-financial")
			require.NoError(t, err)
			require.Equal(t, "disabled", got.Status)
			past := time.Now().Add(-time.Hour)
			state.Status = StatusActive
			state.ExpiresAt = &past
			got, err = first.GetByKey(context.Background(), "synthetic-financial")
			require.NoError(t, err)
			require.True(t, got.IsExpired())
			*got.ExpiresAt = time.Now().Add(time.Hour)
			require.Equal(t, past, *state.ExpiresAt)
			state.Status = StatusAPIKeyExpired
			state.ExpiresAt = nil
			got, err = first.GetByKey(context.Background(), "synthetic-financial")
			require.NoError(t, err)
			require.Equal(t, StatusAPIKeyExpired, got.Status)
			repo.err = errors.New("synthetic primary failed")
			got, err = second.GetByKey(context.Background(), "synthetic-financial")
			require.Nil(t, got)
			require.ErrorIs(t, err, ErrBillingServiceUnavailable)
			repo.err = ErrAPIKeyNotFound
			got, err = first.GetByKey(context.Background(), "synthetic-financial")
			require.Nil(t, got)
			require.ErrorIs(t, err, ErrAPIKeyNotFound)
			require.Zero(t, legacy.calls, "one combined primary statement replaces separate auth balance lookup")
			require.Equal(t, 100.0, original.Quota)
			require.Equal(t, 5.0, original.User.Balance)
			require.Equal(t, 100.0, entry.Snapshot.Quota)
			require.Zero(t, entry.Snapshot.QuotaUsed)
			require.Equal(t, StatusActive, entry.Snapshot.Status)
		})
	}
}
