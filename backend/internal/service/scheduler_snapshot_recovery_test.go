//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type recoveryAccountRepo struct {
	*batchAccountQueryRepo
	account *Account
}

func (r *recoveryAccountRepo) GetByID(context.Context, int64) (*Account, error) {
	return r.account, nil
}

func TestRefreshRecoveredAccountRebuildsAffectedSchedulerBuckets(t *testing.T) {
	account := &Account{
		ID:          17,
		Platform:    PlatformOpenAI,
		GroupIDs:    []int64{42},
		Status:      StatusActive,
		Schedulable: true,
	}
	repo := &recoveryAccountRepo{
		batchAccountQueryRepo: newBatchAccountQueryRepo(),
		account:               account,
	}
	cache := newBulkEventSnapshotCache()
	svc := NewSchedulerSnapshotService(cache, nil, repo, nil, &config.Config{RunMode: config.RunModeStandard})

	require.NoError(t, svc.RefreshRecoveredAccount(context.Background(), account.ID))

	set, deleted := cache.accountWrites()
	require.Equal(t, []int64{account.ID}, set)
	require.Empty(t, deleted)
	require.ElementsMatch(t, schedulerBucketsForTest(account.GroupIDs, account.Platform), cache.capturedBuckets())
}
