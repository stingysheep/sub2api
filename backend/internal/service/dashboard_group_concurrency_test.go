package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type dashboardGroupConcurrencyCacheStub struct {
	ConcurrencyCache
	apiKeyConcurrency    map[int64]int
	apiKeyConcurrencyErr error
	userLoads            map[int64]*UserLoadInfo
	userLoadsErr         error
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
func (s *dashboardGroupConcurrencyCacheStub) GetUsersLoadBatch(ctx context.Context, users []UserWithConcurrency) (map[int64]*UserLoadInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.userLoadsErr != nil {
		return nil, s.userLoadsErr
	}
	loads := make(map[int64]*UserLoadInfo, len(users))
	for _, user := range users {
		if load := s.userLoads[user.ID]; load != nil {
			copy := *load
			loads[user.ID] = &copy
		} else {
			loads[user.ID] = &UserLoadInfo{UserID: user.ID}
		}
	}
	return loads, nil
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
		{APIKeyID: 1, UserID: 101, UserLabel: "a***e", GroupID: 10, GroupName: "group A", Platform: "openai"},
		{APIKeyID: 2, UserID: 101, UserLabel: "a***e", GroupID: 10, GroupName: "group A", Platform: "openai"},
		{APIKeyID: 3, UserID: 202, UserLabel: "b***b", GroupID: 20, GroupName: "group B", Platform: "openai"},
		{APIKeyID: 4, UserID: 303, UserLabel: "c***c"},
	}}
	cache := &dashboardGroupConcurrencyCacheStub{
		apiKeyConcurrency: map[int64]int{1: 2, 2: 3, 4: 1},
		userLoads: map[int64]*UserLoadInfo{
			101: {UserID: 101, CurrentConcurrency: 6},
			202: {UserID: 202},
			303: {UserID: 303, CurrentConcurrency: 3},
		},
	}
	svc := &APIKeyService{apiKeyRepo: repo, concurrencyService: &ConcurrencyService{cache: cache}}
	got, err := svc.GetDashboardGroupConcurrency(context.Background())
	require.NoError(t, err)
	require.Equal(t, []DashboardGroupConcurrency{
		{GroupID: 10, GroupName: "group A", Platform: "openai", CurrentInUse: 5, ActiveUsers: 1, Users: []DashboardGroupConcurrencyUser{{UserID: 101, UserLabel: "a***e", CurrentInUse: 5}}},
		{GroupID: 0, CurrentInUse: 1, ActiveUsers: 1, Users: []DashboardGroupConcurrencyUser{{UserID: 303, UserLabel: "c***c", CurrentInUse: 1}}},
		{GroupID: 20, GroupName: "group B", Platform: "openai", CurrentInUse: 0, Users: []DashboardGroupConcurrencyUser{}},
	}, got.Groups)
	require.Equal(t, 9, got.CurrentConcurrency)
	require.Equal(t, 2, got.ActiveUsers)
	require.Equal(t, []DashboardGroupConcurrencyUser{
		{UserID: 101, UserLabel: "a***e", CurrentInUse: 6},
		{UserID: 303, UserLabel: "c***c", CurrentInUse: 3},
	}, got.Users)
	require.Equal(t, 6, got.GroupAttributedSlots)
	require.Equal(t, 3, got.UnattributedConcurrency)
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
	t.Run("user concurrency", func(t *testing.T) {
		repo := &dashboardGroupKeysStub{keys: []GroupConcurrencyKey{{APIKeyID: 1, UserID: 10, GroupID: 10}}}
		cache := &dashboardGroupConcurrencyCacheStub{userLoadsErr: unavailable}
		svc := &APIKeyService{apiKeyRepo: repo, concurrencyService: &ConcurrencyService{cache: cache}}
		got, err := svc.GetDashboardGroupConcurrency(context.Background())
		require.ErrorIs(t, err, unavailable)
		require.Nil(t, got)
	})
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

type dashboardGroupKeysFunc struct {
	APIKeyRepository
	list func(context.Context, int64, int) ([]GroupConcurrencyKey, error)
}

func (r *dashboardGroupKeysFunc) ListGroupConcurrencyKeys(ctx context.Context, afterID int64, limit int) ([]GroupConcurrencyKey, error) {
	return r.list(ctx, afterID, limit)
}

func TestDashboardGroupConcurrency_ReusesSampleAndCopiesResults(t *testing.T) {
	repo := &dashboardGroupKeysStub{keys: []GroupConcurrencyKey{{APIKeyID: 1, GroupID: 10}}}
	svc := &APIKeyService{apiKeyRepo: repo, concurrencyService: &ConcurrencyService{cache: &dashboardGroupConcurrencyCacheStub{apiKeyConcurrency: map[int64]int{1: 3}}}}
	first, err := svc.GetDashboardGroupConcurrency(context.Background())
	require.NoError(t, err)
	timestamp := first.Timestamp
	first.Groups[0].CurrentInUse = 999
	first.Users = append(first.Users, DashboardGroupConcurrencyUser{UserID: 999})
	first.Timestamp = time.Time{}
	second, err := svc.GetDashboardGroupConcurrency(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, repo.calls, "polling within the sample window must not rescan keys")
	require.Equal(t, 3, second.Groups[0].CurrentInUse)
	require.Empty(t, second.Users)
	require.Equal(t, timestamp, second.Timestamp)
}

func TestDashboardGroupConcurrency_ConcurrentRequestsShareSample(t *testing.T) {
	var calls atomic.Int32
	repo := &dashboardGroupKeysFunc{list: func(context.Context, int64, int) ([]GroupConcurrencyKey, error) {
		calls.Add(1)
		return []GroupConcurrencyKey{{APIKeyID: 1, GroupID: 10}}, nil
	}}
	svc := &APIKeyService{apiKeyRepo: repo, concurrencyService: &ConcurrencyService{cache: &dashboardGroupConcurrencyCacheStub{}}}
	const clients = 32
	start := make(chan struct{})
	results := make(chan *DashboardGroupConcurrencySnapshot, clients)
	errors := make(chan error, clients)
	var wg sync.WaitGroup
	for i := 0; i < clients; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			result, err := svc.GetDashboardGroupConcurrency(context.Background())
			results <- result
			errors <- err
		}()
	}
	close(start)
	wg.Wait()
	require.EqualValues(t, 1, calls.Load())
	var timestamp time.Time
	for i := 0; i < clients; i++ {
		require.NoError(t, <-errors)
		result := <-results
		if i == 0 {
			timestamp = result.Timestamp
		}
		require.Equal(t, timestamp, result.Timestamp)
	}
}

func TestDashboardGroupConcurrency_CallerCancellationDoesNotCancelSample(t *testing.T) {
	started := make(chan context.Context, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	var calls atomic.Int32
	repo := &dashboardGroupKeysFunc{list: func(ctx context.Context, _ int64, _ int) ([]GroupConcurrencyKey, error) {
		calls.Add(1)
		select {
		case started <- ctx:
		default:
		}
		select {
		case <-release:
			return nil, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}}
	svc := &APIKeyService{apiKeyRepo: repo, concurrencyService: &ConcurrencyService{cache: &dashboardGroupConcurrencyCacheStub{}}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, err := svc.GetDashboardGroupConcurrency(ctx)
		result <- err
	}()
	var sampleCtx context.Context
	select {
	case sampleCtx = <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("sample did not start")
	}
	cancel()
	select {
	case err := <-result:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("caller cancellation blocked behind the sample")
	}
	require.NoError(t, sampleCtx.Err(), "the initiating caller must not cancel the shared sample")
	deadline, ok := sampleCtx.Deadline()
	require.True(t, ok)
	require.WithinDuration(t, time.Now().Add(4*time.Second), deadline, time.Second)
	waiterCtx, waiterCancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer waiterCancel()
	_, err := svc.GetDashboardGroupConcurrency(waiterCtx)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.EqualValues(t, 1, calls.Load(), "a waiter must join the existing sample")
	require.NoError(t, sampleCtx.Err())
	unblock()
	got, err := svc.GetDashboardGroupConcurrency(context.Background())
	require.NoError(t, err)
	require.NotNil(t, got.Groups)
	require.EqualValues(t, 1, calls.Load())
}

func TestDashboardGroupConcurrency_IsolatesServiceInstances(t *testing.T) {
	for _, groupID := range []int64{10, 20} {
		svc := &APIKeyService{apiKeyRepo: &dashboardGroupKeysStub{keys: []GroupConcurrencyKey{{APIKeyID: 1, GroupID: groupID}}}, concurrencyService: &ConcurrencyService{cache: &dashboardGroupConcurrencyCacheStub{}}}
		got, err := svc.GetDashboardGroupConcurrency(context.Background())
		require.NoError(t, err)
		require.Equal(t, groupID, got.Groups[0].GroupID)
	}
}

func TestDashboardGroupConcurrency_RetriesFailedSample(t *testing.T) {
	repo := &dashboardGroupKeysStub{err: errors.New("database unavailable")}
	svc := &APIKeyService{apiKeyRepo: repo, concurrencyService: &ConcurrencyService{cache: &dashboardGroupConcurrencyCacheStub{}}}
	got, err := svc.GetDashboardGroupConcurrency(context.Background())
	require.Error(t, err)
	require.Nil(t, got)
	repo.err = nil
	got, err = svc.GetDashboardGroupConcurrency(context.Background())
	require.NoError(t, err)
	require.NotNil(t, got.Groups)
	require.Equal(t, 2, repo.calls)
}

func TestDashboardGroupConcurrency_ExpiredSampleRefreshesAndDoesNotHideErrors(t *testing.T) {
	repo := &dashboardGroupKeysStub{keys: []GroupConcurrencyKey{{APIKeyID: 1, GroupID: 10}}}
	cache := &dashboardGroupConcurrencyCacheStub{apiKeyConcurrency: map[int64]int{1: 3}}
	svc := &APIKeyService{apiKeyRepo: repo, concurrencyService: &ConcurrencyService{cache: cache}}
	got, err := svc.GetDashboardGroupConcurrency(context.Background())
	require.NoError(t, err)
	require.Equal(t, 3, got.Groups[0].CurrentInUse)
	expire := func() {
		svc.dashboardGroupConcurrency.mu.Lock()
		svc.dashboardGroupConcurrency.expiresAt = time.Now().Add(-time.Second)
		svc.dashboardGroupConcurrency.mu.Unlock()
	}
	expire()
	cache.apiKeyConcurrency[1] = 7
	got, err = svc.GetDashboardGroupConcurrency(context.Background())
	require.NoError(t, err)
	require.Equal(t, 7, got.Groups[0].CurrentInUse)
	require.Equal(t, 2, repo.calls)
	expire()
	cache.apiKeyConcurrencyErr = errors.New("redis unavailable")
	got, err = svc.GetDashboardGroupConcurrency(context.Background())
	require.ErrorIs(t, err, cache.apiKeyConcurrencyErr)
	require.Nil(t, got, "an expired successful sample must not hide a failed refresh")
	cache.apiKeyConcurrencyErr = nil
	got, err = svc.GetDashboardGroupConcurrency(context.Background())
	require.NoError(t, err)
	require.Equal(t, 7, got.Groups[0].CurrentInUse)
	require.Equal(t, 4, repo.calls)
}

func TestDashboardGroupConcurrency_DoesNotShareCallerScope(t *testing.T) {
	type scopeKey struct{}
	observedScope := make(chan any, 1)
	repo := &dashboardGroupKeysFunc{list: func(ctx context.Context, _ int64, _ int) ([]GroupConcurrencyKey, error) {
		observedScope <- ctx.Value(scopeKey{})
		return nil, nil
	}}
	svc := &APIKeyService{apiKeyRepo: repo, concurrencyService: &ConcurrencyService{cache: &dashboardGroupConcurrencyCacheStub{}}}
	_, err := svc.GetDashboardGroupConcurrency(context.WithValue(context.Background(), scopeKey{}, "caller-specific"))
	require.NoError(t, err)
	require.Nil(t, <-observedScope)
}

func TestDashboardGroupConcurrency_CanceledCallerDoesNotStartSample(t *testing.T) {
	repo := &dashboardGroupKeysStub{}
	svc := &APIKeyService{apiKeyRepo: repo, concurrencyService: &ConcurrencyService{cache: &dashboardGroupConcurrencyCacheStub{}}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err := svc.GetDashboardGroupConcurrency(ctx)
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, got)
	require.Zero(t, repo.calls)
}

func TestDashboardGroupConcurrency_RejectsInvalidProjectionOrdering(t *testing.T) {
	repo := &dashboardGroupKeysFunc{list: func(context.Context, int64, int) ([]GroupConcurrencyKey, error) {
		return []GroupConcurrencyKey{{APIKeyID: 2}, {APIKeyID: 1}}, nil
	}}
	svc := &APIKeyService{apiKeyRepo: repo, concurrencyService: &ConcurrencyService{cache: &dashboardGroupConcurrencyCacheStub{}}}
	got, err := svc.GetDashboardGroupConcurrency(context.Background())
	require.ErrorContains(t, err, "invalid concurrency projection ordering")
	require.Nil(t, got)
}
