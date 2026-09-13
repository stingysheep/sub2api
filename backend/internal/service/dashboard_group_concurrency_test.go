package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type dashboardGroupConcurrencyCacheStub struct {
	ConcurrencyCache
	apiKeyConcurrency    map[int64]int
	apiKeyConcurrencyErr error
}

func (s *dashboardGroupConcurrencyCacheStub) TrackAPIKeySlot(context.Context, int64, string) error {
	return nil
}
func (s *dashboardGroupConcurrencyCacheStub) ReleaseAPIKeySlot(context.Context, int64, string) error {
	return nil
}
func (s *dashboardGroupConcurrencyCacheStub) GetAPIKeyConcurrencyBatch(ctx context.Context, ids []int64) (map[int64]int, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.apiKeyConcurrency, s.apiKeyConcurrencyErr
}

type dashboardGroupKeysStub struct {
	APIKeyRepository
	keys  []GroupConcurrencyKey
	err   error
	calls int
}

func (r *dashboardGroupKeysStub) ListGroupConcurrencyKeys(_ context.Context, afterID int64, limit int) ([]GroupConcurrencyKey, error) {
	r.calls++
	if r.err != nil {
		return nil, r.err
	}
	out := make([]GroupConcurrencyKey, 0, limit)
	for _, key := range r.keys {
		if key.APIKeyID > afterID {
			out = append(out, key)
			if len(out) == limit {
				break
			}
		}
	}
	return out, nil
}
func TestDashboardGroupConcurrency_AggregatesKeysAndKeepsIdleGroups(t *testing.T) {
	repo := &dashboardGroupKeysStub{keys: []GroupConcurrencyKey{
		{APIKeyID: 1, GroupID: 10, GroupName: "group A", Platform: "openai"},
		{APIKeyID: 2, GroupID: 10, GroupName: "group A", Platform: "openai"},
		{APIKeyID: 3, GroupID: 20, GroupName: "group B", Platform: "openai"},
		{APIKeyID: 4},
	}}
	cache := &dashboardGroupConcurrencyCacheStub{apiKeyConcurrency: map[int64]int{1: 2, 2: 3, 4: 1}}
	svc := &APIKeyService{apiKeyRepo: repo, concurrencyService: &ConcurrencyService{cache: cache}}
	got, err := svc.GetDashboardGroupConcurrency(context.Background())
	require.NoError(t, err)
	require.Equal(t, []DashboardGroupConcurrency{
		{GroupID: 10, GroupName: "group A", Platform: "openai", CurrentInUse: 5},
		{GroupID: 0, CurrentInUse: 1},
		{GroupID: 20, GroupName: "group B", Platform: "openai", CurrentInUse: 0},
	}, got.Groups)
	require.False(t, got.Timestamp.IsZero())
}
func TestDashboardGroupConcurrency_PagesWithoutDuplicatingKeys(t *testing.T) {
	repo := &dashboardGroupKeysStub{}
	cache := &dashboardGroupConcurrencyCacheStub{apiKeyConcurrency: map[int64]int{}}
	for id := int64(1); id <= 1001; id++ {
		repo.keys = append(repo.keys, GroupConcurrencyKey{APIKeyID: id, GroupID: 10})
		cache.apiKeyConcurrency[id] = 1
	}
	svc := &APIKeyService{apiKeyRepo: repo, concurrencyService: &ConcurrencyService{cache: cache}}
	got, err := svc.GetDashboardGroupConcurrency(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1001, got.Groups[0].CurrentInUse)
	require.Equal(t, 3, repo.calls)
}
func TestDashboardGroupConcurrency_ErrorsAreNotIdle(t *testing.T) {
	unavailable := errors.New("unavailable")
	for _, test := range []struct {
		name              string
		repoErr, cacheErr error
	}{
		{name: "database", repoErr: unavailable}, {name: "redis", cacheErr: unavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := &dashboardGroupKeysStub{keys: []GroupConcurrencyKey{{APIKeyID: 1, GroupID: 10}}, err: test.repoErr}
			svc := &APIKeyService{apiKeyRepo: repo, concurrencyService: &ConcurrencyService{cache: &dashboardGroupConcurrencyCacheStub{apiKeyConcurrencyErr: test.cacheErr}}}
			got, err := svc.GetDashboardGroupConcurrency(context.Background())
			require.ErrorIs(t, err, unavailable)
			require.Nil(t, got)
		})
	}
	var missing *APIKeyService
	_, err := missing.GetDashboardGroupConcurrency(context.Background())
	require.Error(t, err)
}
func TestDashboardGroupConcurrency_EmptyIsArray(t *testing.T) {
	svc := &APIKeyService{apiKeyRepo: &dashboardGroupKeysStub{}, concurrencyService: &ConcurrencyService{cache: &dashboardGroupConcurrencyCacheStub{}}}
	got, err := svc.GetDashboardGroupConcurrency(context.Background())
	require.NoError(t, err)
	require.NotNil(t, got.Groups)
	require.Empty(t, got.Groups)
}
