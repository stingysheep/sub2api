//go:build unit

package repository

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

type operatorVersionRepo struct {
	service.UserRepository
	mu      sync.Mutex
	balance float64
	version int64
	calls   int
	err     error
}

func (r *operatorVersionRepo) GetUserBalanceVersion(context.Context, int64) (float64, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	return r.balance, r.version, r.err
}

func (r *operatorVersionRepo) commit(balance float64, version int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.balance, r.version = balance, version
}

func TestOperatorBalanceVersion_TwoInstancesCrashRestartAndMissedPubSub(t *testing.T) {
	cache, mr := newMiniRedisCache(t)
	t.Cleanup(func() { _ = cache.rdb.Close() })
	other := &billingCache{rdb: redis.NewClient(&redis.Options{Addr: mr.Addr()})}
	t.Cleanup(func() { _ = other.rdb.Close() })
	repo := &operatorVersionRepo{balance: 100}
	first := service.NewBillingCacheService(cache, repo, nil, nil, nil, nil, &config.Config{}, nil)
	second := service.NewBillingCacheService(other, repo, nil, nil, nil, nil, &config.Config{}, nil)
	t.Cleanup(first.Stop)
	t.Cleanup(second.Stop)
	ctx := context.Background()
	// Existing unversioned cache from a previous process must be discarded.
	require.NoError(t, cache.SetUserBalance(ctx, 7, 999))
	balance, err := first.GetUserBalance(ctx, 7)
	require.NoError(t, err)
	require.Equal(t, float64(100), balance)
	first.Stop()
	before := repo.calls
	balance, err = second.GetUserBalance(ctx, 7)
	require.NoError(t, err)
	require.Equal(t, float64(100), balance)
	require.Equal(t, before+1, repo.calls, "cache hit adds exactly one bounded DB query")
	// Simulate commit followed by process death: no dirty bit, invalidation,
	// pubsub, or other application action occurs after this DB change.
	repo.commit(0, 1)
	err = second.CheckBillingEligibility(ctx, &service.User{ID: 7}, nil, nil, nil, "")
	require.ErrorIs(t, err, service.ErrInsufficientBalance)
	second.Stop()
	restarted := service.NewBillingCacheService(cache, repo, nil, nil, nil, nil, &config.Config{}, nil)
	t.Cleanup(restarted.Stop)
	balance, err = restarted.GetUserBalance(ctx, 7)
	require.NoError(t, err)
	require.Zero(t, balance)
	// A later top-up is also visible without a delivered invalidation.
	repo.commit(25, 2)
	balance, err = restarted.GetUserBalance(ctx, 7)
	require.NoError(t, err)
	require.Equal(t, float64(25), balance)
}

func TestOperatorBalanceVersion_CurrentGenerationCacheHitUsesAuthoritativeBalance(t *testing.T) {
	tests := []struct {
		name          string
		dbBalance     float64
		cachedBalance float64
		expectBalance float64
		expectError   error
	}{
		{
			name:          "stale high cache is rejected",
			dbBalance:     0,
			cachedBalance: 100,
			expectBalance: 0,
			expectError:   service.ErrInsufficientBalance,
		},
		{
			name:          "stale low cache does not hide top-up",
			dbBalance:     25,
			cachedBalance: 0,
			expectBalance: 25,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache, _ := newMiniRedisCache(t)
			t.Cleanup(func() { _ = cache.rdb.Close() })
			repo := &operatorVersionRepo{balance: tt.dbBalance, version: 0}
			svc := service.NewBillingCacheService(cache, repo, nil, nil, nil, nil, &config.Config{}, nil)
			t.Cleanup(svc.Stop)
			ctx := context.Background()
			matched, err := cache.EnsureUserBalanceVersion(ctx, 7, 0)
			require.NoError(t, err)
			require.True(t, matched)
			require.NoError(t, cache.SetUserBalance(ctx, 7, tt.cachedBalance))

			if tt.expectError != nil {
				err := svc.CheckBillingEligibility(ctx, &service.User{ID: 7}, nil, nil, nil, "")
				require.ErrorIs(t, err, tt.expectError)
			}
			balance, err := svc.GetUserBalance(ctx, 7)
			require.NoError(t, err)
			require.Equal(t, tt.expectBalance, balance)
		})
	}
}

func TestOperatorBalanceVersion_RevokesPreCommitFillAndRejectsOldGeneration(t *testing.T) {
	cache, mr := newMiniRedisCache(t)
	t.Cleanup(func() { _ = cache.rdb.Close() })
	other := &billingCache{rdb: redis.NewClient(&redis.Options{Addr: mr.Addr()})}
	t.Cleanup(func() { _ = other.rdb.Close() })
	ctx := context.Background()
	ok, err := cache.EnsureUserBalanceVersion(ctx, 7, 0)
	require.NoError(t, err)
	require.True(t, ok)
	oldLease, err := cache.BeginUserBalanceFill(ctx, 7)
	require.NoError(t, err)
	require.NotEmpty(t, oldLease)
	// A pre-commit DB read is delayed after acquiring its fill lease.
	ok, err = other.EnsureUserBalanceVersion(ctx, 7, 1)
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, cache.FillUserBalance(ctx, 7, 100, oldLease))
	require.False(t, mr.Exists(billingBalanceKey(7)))
	newLease, err := other.BeginUserBalanceFill(ctx, 7)
	require.NoError(t, err)
	require.NoError(t, other.FillUserBalance(ctx, 7, 0, newLease))
	ok, err = cache.EnsureUserBalanceVersion(ctx, 7, 0)
	require.NoError(t, err)
	require.False(t, ok, "late DB reader cannot reset the committed generation")
	balance, err := cache.GetUserBalance(ctx, 7)
	require.NoError(t, err)
	require.Zero(t, balance)
	// Full bigint precision is required for monotonic comparisons.
	ok, err = cache.EnsureUserBalanceVersion(ctx, 7, 9007199254740993)
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = other.EnsureUserBalanceVersion(ctx, 7, 9007199254740992)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestOperatorBalanceVersion_CommitWithoutInvalidationRevokesQueuedStaleFill(t *testing.T) {
	cache, mr := newMiniRedisCache(t)
	t.Cleanup(func() { _ = cache.rdb.Close() })
	other := &billingCache{rdb: redis.NewClient(&redis.Options{Addr: mr.Addr()})}
	t.Cleanup(func() { _ = other.rdb.Close() })
	delayed := &delayedBalanceFillCache{billingCache: cache, started: make(chan struct{}), release: make(chan struct{})}
	var once sync.Once
	release := func() { once.Do(func() { close(delayed.release) }) }
	// LIFO cleanup releases the blocked worker before Stop waits on it.
	repo := &operatorVersionRepo{balance: 100}
	first := service.NewBillingCacheService(delayed, repo, nil, nil, nil, nil, &config.Config{}, nil)
	t.Cleanup(first.Stop)
	t.Cleanup(release)
	second := service.NewBillingCacheService(other, repo, nil, nil, nil, nil, &config.Config{}, nil)
	t.Cleanup(second.Stop)
	ctx := context.Background()
	balance, err := first.GetUserBalance(ctx, 7)
	require.NoError(t, err)
	require.Equal(t, float64(100), balance)
	select {
	case <-delayed.started:
	case <-time.After(time.Second):
		t.Fatal("old queued fill did not start")
	}
	repo.commit(0, 1) // no post-commit cache callback
	balance, err = second.GetUserBalance(ctx, 7)
	require.NoError(t, err)
	require.Zero(t, balance)
	second.Stop()
	release()
	first.Stop()
	balance, err = cache.GetUserBalance(ctx, 7)
	require.NoError(t, err)
	require.Zero(t, balance, "old asynchronous filler cannot revive the pre-commit amount")
}

func TestOperatorBalanceVersion_ExpiryRevokesLeaseAndAuthorityErrorsReject(t *testing.T) {
	cache, mr := newMiniRedisCache(t)
	t.Cleanup(func() { _ = cache.rdb.Close() })
	ctx := context.Background()
	_, err := cache.EnsureUserBalanceVersion(ctx, 7, 1)
	require.NoError(t, err)
	lease, err := cache.BeginUserBalanceFill(ctx, 7)
	require.NoError(t, err)
	mr.FastForward(balanceFillLeaseTTL + time.Second)
	require.NoError(t, cache.FillUserBalance(ctx, 7, 100, lease))
	require.False(t, mr.Exists(billingBalanceKey(7)))
	repo := &operatorVersionRepo{balance: 3, version: 1}
	svc := service.NewBillingCacheService(cache, repo, nil, nil, nil, nil, &config.Config{}, nil)
	t.Cleanup(svc.Stop)
	require.NoError(t, cache.SetUserBalance(ctx, 7, 999))
	repo.err = errors.New("primary unavailable")
	_, err = svc.GetUserBalance(ctx, 7)
	require.ErrorContains(t, err, "primary unavailable", "a cache hit cannot conceal primary failure")
	repo.err = nil
	// Closed Redis must use exactly the authoritative result, not an auth/cache snapshot.
	require.NoError(t, cache.rdb.Close())
	before := repo.calls
	balance, err := svc.GetUserBalance(ctx, 7)
	require.NoError(t, err)
	require.Equal(t, float64(3), balance)
	require.Equal(t, before+1, repo.calls)
	noCache := service.NewBillingCacheService(nil, repo, nil, nil, nil, nil, &config.Config{}, nil)
	t.Cleanup(noCache.Stop)
	before = repo.calls
	balance, err = noCache.GetUserBalance(ctx, 7)
	require.NoError(t, err)
	require.Equal(t, float64(3), balance)
	require.Equal(t, before+1, repo.calls, "nil cache requires one DB lookup and no full user load")
}
