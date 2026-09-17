package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRunBalanceRefreshDuePersistsAndThrottlesSnapshots(t *testing.T) {
	now := time.Now().UTC()
	account := upstreamBalanceTestAccount("https://relay.example/v1")
	account.Status = StatusActive
	account.Schedulable = true
	account.Extra = map[string]any{UpstreamBillingProbeEnabledExtraKey: true}
	repo := &upstreamBillingProbeAccountRepo{accounts: map[int64]*Account{account.ID: account}}
	upstream := &upstreamBalanceHTTPStub{responses: []*http.Response{
		upstreamBalanceResponse(http.StatusOK, `{"data":{"balance":"12.5","currency":"USD"}}`),
		upstreamBalanceResponse(http.StatusOK, `{"data":{"balance":"19.75","currency":"USD"}}`),
	}}
	accountTest := newUpstreamBalanceTestService(upstream)
	accountTest.accountRepo = repo
	svc := NewUpstreamBillingProbeService(repo, accountTest, nil)
	svc.now = func() time.Time { return now }

	require.NoError(t, svc.RunBalanceRefreshDue(context.Background()))
	require.Len(t, upstream.requests, 1)
	stored, err := repo.GetByID(context.Background(), account.ID)
	require.NoError(t, err)
	snapshotAt := upstreamBalanceSnapshotFetchedAt(stored.Extra)
	require.False(t, snapshotAt.IsZero())

	require.NoError(t, svc.RunBalanceRefreshDue(context.Background()))
	require.Len(t, upstream.requests, 1, "a fresh snapshot must not be queried again")

	now = snapshotAt.Add(upstreamBillingProbeDefaultIntervalMinutes*time.Minute + time.Second)
	require.NoError(t, svc.RunBalanceRefreshDue(context.Background()))
	require.Len(t, upstream.requests, 2)
	stored, err = repo.GetByID(context.Background(), account.ID)
	require.NoError(t, err)
	snapshot, ok := stored.Extra[upstreamBalanceSnapshotExtraKey].(upstreamBalanceSnapshot)
	require.True(t, ok)
	require.Equal(t, 19.75, *snapshot.Entries[0].Remaining)
}

func TestRunBalanceRefreshDueBacksOffFailedAttempts(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	account := upstreamBalanceTestAccount("https://relay.example/v1")
	account.Status = StatusActive
	account.Schedulable = true
	account.Extra = map[string]any{UpstreamBillingProbeEnabledExtraKey: true}
	repo := &upstreamBillingProbeAccountRepo{accounts: map[int64]*Account{account.ID: account}}
	upstream := &upstreamBalanceHTTPStub{responses: []*http.Response{
		upstreamBalanceResponse(http.StatusUnauthorized, `{"error":"rejected"}`),
		upstreamBalanceResponse(http.StatusUnauthorized, `{"error":"rejected"}`),
	}}
	accountTest := newUpstreamBalanceTestService(upstream)
	accountTest.accountRepo = repo
	svc := NewUpstreamBillingProbeService(repo, accountTest, nil)
	svc.now = func() time.Time { return now }

	require.NoError(t, svc.RunBalanceRefreshDue(context.Background()))
	require.Len(t, upstream.requests, 1)
	require.NoError(t, svc.RunBalanceRefreshDue(context.Background()))
	require.Len(t, upstream.requests, 1, "failed accounts must not be hammered every minute")

	restarted := NewUpstreamBillingProbeService(repo, accountTest, nil)
	restarted.now = func() time.Time { return now }
	require.NoError(t, restarted.RunBalanceRefreshDue(context.Background()))
	require.Len(t, upstream.requests, 1, "failed-attempt backoff must survive a process restart")

	now = now.Add(upstreamBillingProbeDefaultIntervalMinutes*time.Minute + time.Second)
	require.NoError(t, svc.RunBalanceRefreshDue(context.Background()))
	require.Len(t, upstream.requests, 2)
}

func TestRunBalanceRefreshDueRequiresExistingProbeOptInAndSchedulableAccount(t *testing.T) {
	optedOut := upstreamBalanceTestAccount("https://relay.example/v1")
	optedOut.ID = 1
	optedOut.Status = StatusActive
	optedOut.Schedulable = true
	optedOut.Extra = map[string]any{UpstreamBillingProbeEnabledExtraKey: false}
	unschedulable := upstreamBalanceTestAccount("https://relay.example/v1")
	unschedulable.ID = 2
	unschedulable.Status = StatusActive
	unschedulable.Schedulable = false
	unschedulable.Extra = map[string]any{UpstreamBillingProbeEnabledExtraKey: true}
	unsupported := upstreamBalanceTestAccount("https://relay.example/v1")
	unsupported.ID = 3
	unsupported.Platform = "unsupported-platform"
	unsupported.Status = StatusActive
	unsupported.Schedulable = true
	unsupported.Extra = map[string]any{UpstreamBillingProbeEnabledExtraKey: true}
	repo := &upstreamBillingProbeAccountRepo{accounts: map[int64]*Account{1: optedOut, 2: unschedulable, 3: unsupported}}
	upstream := &upstreamBalanceHTTPStub{}
	accountTest := newUpstreamBalanceTestService(upstream)
	accountTest.accountRepo = repo

	require.NoError(t, NewUpstreamBillingProbeService(repo, accountTest, nil).RunBalanceRefreshDue(context.Background()))
	require.Empty(t, upstream.requests)
}

func TestRunBalanceRefreshDueHonorsGlobalProbeSwitch(t *testing.T) {
	account := upstreamBalanceTestAccount("https://relay.example/v1")
	account.Status = StatusActive
	account.Schedulable = true
	account.Extra = map[string]any{UpstreamBillingProbeEnabledExtraKey: true}
	repo := &upstreamBillingProbeAccountRepo{accounts: map[int64]*Account{account.ID: account}}
	upstream := &upstreamBalanceHTTPStub{}
	settings := &upstreamBillingProbeSettingRepo{values: map[string]string{
		SettingKeyUpstreamBillingProbeSettings: `{"enabled":false,"interval_minutes":30}`,
	}}

	require.NoError(t, newUpstreamBillingProbeTestService(repo, upstream, settings).RunBalanceRefreshDue(context.Background()))
	require.Empty(t, upstream.requests)
}
