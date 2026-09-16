//go:build unit

package repository

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

type blockedBalanceFillRepo struct {
	service.UserRepository
	started chan struct{}
	release chan struct{}
}

type delayedBalanceFillCache struct {
	*billingCache
	started chan struct{}
	release chan struct{}
}

func (c *delayedBalanceFillCache) FillUserBalance(ctx context.Context, id int64, balance float64, lease string) error {
	close(c.started)
	select {
	case <-c.release:
		return c.billingCache.FillUserBalance(ctx, id, balance, lease)
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *blockedBalanceFillRepo) GetByID(ctx context.Context, id int64) (*service.User, error) {
	close(r.started)
	select {
	case <-r.release:
		return &service.User{ID: id, Balance: 100}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestBalanceFill_DoesNotResurrectAfterConcurrentMutation(t *testing.T) {
	for _, mutation := range []string{"invalidate", "deduct_miss", "deduct_hit", "replace"} {
		t.Run(mutation, func(t *testing.T) {
			cache, mr := newMiniRedisCache(t)
			t.Cleanup(func() { _ = cache.rdb.Close() })
			other := &billingCache{rdb: redis.NewClient(&redis.Options{Addr: mr.Addr()})}
			t.Cleanup(func() { _ = other.rdb.Close() })
			repo := &blockedBalanceFillRepo{started: make(chan struct{}), release: make(chan struct{})}
			var releaseOnce sync.Once
			release := func() { releaseOnce.Do(func() { close(repo.release) }) }
			svc := service.NewBillingCacheService(cache, repo, nil, nil, nil, nil, &config.Config{}, nil)
			t.Cleanup(svc.Stop)
			t.Cleanup(release)
			done := make(chan error, 1)
			go func() {
				_, err := svc.GetUserBalance(context.Background(), 7)
				done <- err
			}()
			select {
			case <-repo.started:
			case <-time.After(5 * time.Second):
				t.Fatal("balance load did not start")
			}
			ctx := context.Background()
			switch mutation {
			case "invalidate":
				require.NoError(t, other.InvalidateUserBalance(ctx, 7))
			case "deduct_miss":
				require.NoError(t, other.DeductUserBalance(ctx, 7, 20))
			case "deduct_hit":
				require.NoError(t, other.SetUserBalance(ctx, 7, 100))
				require.NoError(t, other.DeductUserBalance(ctx, 7, 20))
			case "replace":
				require.NoError(t, other.SetUserBalance(ctx, 7, 80))
			}
			release()
			require.NoError(t, <-done)
			svc.Stop()
			balance, err := cache.GetUserBalance(ctx, 7)
			if mutation == "invalidate" || mutation == "deduct_miss" {
				require.ErrorIs(t, err, redis.Nil, "stale DB read must not revive a missing balance")
			} else {
				require.NoError(t, err)
				require.Equal(t, float64(80), balance)
			}
		})
	}
}

func TestBalanceFill_LeaseOwnershipAndExpiration(t *testing.T) {
	cache, mr := newMiniRedisCache(t)
	t.Cleanup(func() { _ = cache.rdb.Close() })
	other := &billingCache{rdb: redis.NewClient(&redis.Options{Addr: mr.Addr()})}
	t.Cleanup(func() { _ = other.rdb.Close() })
	ctx := context.Background()
	lease, err := cache.BeginUserBalanceFill(ctx, 7)
	require.NoError(t, err)
	require.NotEmpty(t, lease)
	contender, err := other.BeginUserBalanceFill(ctx, 7)
	require.NoError(t, err)
	require.Empty(t, contender)
	require.NoError(t, other.FillUserBalance(ctx, 7, 100, "wrong-owner"))
	require.False(t, mr.Exists(billingBalanceKey(7)))

	// A queued writer can outlive the lease. Missing or reused keys must not
	// permit the old writer, including after another instance acquires a lease.
	mr.FastForward(balanceFillLeaseTTL + time.Second)
	require.NoError(t, cache.FillUserBalance(ctx, 7, 100, lease))
	require.False(t, mr.Exists(billingBalanceKey(7)))
	newLease, err := other.BeginUserBalanceFill(ctx, 7)
	require.NoError(t, err)
	require.NotEmpty(t, newLease)
	require.NotEqual(t, lease, newLease)
	require.NoError(t, cache.FillUserBalance(ctx, 7, 100, lease))
	require.False(t, mr.Exists(billingBalanceKey(7)))
	require.NoError(t, other.FillUserBalance(ctx, 7, 80, newLease))
	balance, err := cache.GetUserBalance(ctx, 7)
	require.NoError(t, err)
	require.Equal(t, float64(80), balance)
	require.False(t, mr.Exists(billingBalanceFillKey(7)))
	require.Positive(t, mr.TTL(billingBalanceKey(7)))
	require.LessOrEqual(t, mr.TTL(billingBalanceKey(7)), billingCacheTTL)

	// A consumed lease cannot be replayed after invalidation either.
	require.NoError(t, other.InvalidateUserBalance(ctx, 7))
	require.NoError(t, cache.FillUserBalance(ctx, 7, 100, newLease))
	require.False(t, mr.Exists(billingBalanceKey(7)))
}

func TestBalanceFill_RevokedLeaseCannotReplaceNewGeneration(t *testing.T) {
	cache, mr := newMiniRedisCache(t)
	t.Cleanup(func() { _ = cache.rdb.Close() })
	ctx := context.Background()
	oldLease, err := cache.BeginUserBalanceFill(ctx, 7)
	require.NoError(t, err)
	require.NoError(t, cache.DeductUserBalance(ctx, 7, 20))
	newLease, err := cache.BeginUserBalanceFill(ctx, 7)
	require.NoError(t, err)
	require.NotEmpty(t, newLease)
	require.NotEqual(t, oldLease, newLease)
	require.NoError(t, cache.FillUserBalance(ctx, 7, 100, oldLease))
	require.False(t, mr.Exists(billingBalanceKey(7)))
	require.NoError(t, cache.FillUserBalance(ctx, 7, 80, newLease))
	balance, err := cache.GetUserBalance(ctx, 7)
	require.NoError(t, err)
	require.Equal(t, float64(80), balance)
	lease, err := cache.BeginUserBalanceFill(ctx, 7)
	require.NoError(t, err)
	require.Empty(t, lease, "cache hits must not acquire a fill lease")
}

func TestBalanceFill_ServicePopulatesCacheWithoutMutation(t *testing.T) {
	cache, _ := newMiniRedisCache(t)
	t.Cleanup(func() { _ = cache.rdb.Close() })
	repo := &blockedBalanceFillRepo{started: make(chan struct{}), release: make(chan struct{})}
	close(repo.release)
	svc := service.NewBillingCacheService(cache, repo, nil, nil, nil, nil, &config.Config{}, nil)
	t.Cleanup(svc.Stop)
	balance, err := svc.GetUserBalance(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, float64(100), balance)
	svc.Stop()
	// A second DB read would close started twice and panic.
	balance, err = svc.GetUserBalance(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, float64(100), balance)
}

func TestBalanceFill_QueuedWriteCannotResurrectAfterMutation(t *testing.T) {
	for _, deduct := range []bool{false, true} {
		t.Run(map[bool]string{false: "invalidate", true: "deduct"}[deduct], func(t *testing.T) {
			cache, mr := newMiniRedisCache(t)
			t.Cleanup(func() { _ = cache.rdb.Close() })
			other := &billingCache{rdb: redis.NewClient(&redis.Options{Addr: mr.Addr()})}
			t.Cleanup(func() { _ = other.rdb.Close() })
			delayed := &delayedBalanceFillCache{billingCache: cache, started: make(chan struct{}), release: make(chan struct{})}
			repo := &blockedBalanceFillRepo{started: make(chan struct{}), release: make(chan struct{})}
			close(repo.release)
			svc := service.NewBillingCacheService(delayed, repo, nil, nil, nil, nil, &config.Config{}, nil)
			t.Cleanup(svc.Stop)
			var once sync.Once
			release := func() { once.Do(func() { close(delayed.release) }) }
			t.Cleanup(release)
			_, err := svc.GetUserBalance(context.Background(), 7)
			require.NoError(t, err)
			select {
			case <-delayed.started:
			case <-time.After(time.Second):
				t.Fatal("fill worker did not start")
			}
			if deduct {
				require.NoError(t, other.DeductUserBalance(context.Background(), 7, 20))
			} else {
				require.NoError(t, other.InvalidateUserBalance(context.Background(), 7))
			}
			release()
			svc.Stop()
			_, err = cache.GetUserBalance(context.Background(), 7)
			require.ErrorIs(t, err, redis.Nil)
		})
	}
}
