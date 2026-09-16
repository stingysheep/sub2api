package service

import (
	"context"
	"errors"
	"github.com/stretchr/testify/require"
	"testing"
)

type operatorRepoStub struct {
	calls    int
	commands []OperatorBalanceCommand
	result   *OperatorBalanceAdjustmentResult
	err      error
}

func (r *operatorRepoStub) AdjustOperatorBalance(_ context.Context, c OperatorBalanceCommand) (*OperatorBalanceAdjustmentResult, error) {
	r.calls++
	r.commands = append(r.commands, c)
	if r.err != nil {
		return nil, r.err
	}
	if r.result != nil {
		copy := *r.result
		return &copy, nil
	}
	return &OperatorBalanceAdjustmentResult{OperationID: c.OperationID}, nil
}
func TestOperatorBalanceExactAmount(t *testing.T) {
	for _, v := range []string{"0", "-1", "+1", "1e2", "NaN", "Infinity", "1.000000001", "1000000000000", " 1", "1."} {
		_, e := ParseOperatorBalanceAmount(v)
		require.Error(t, e, v)
	}
	for _, v := range []string{"0.00000001", "999999999999.99999999", "01.20"} {
		n, e := ParseOperatorBalanceAmount(v)
		require.NoError(t, e)
		again, e := ParseOperatorBalanceAmount(FormatOperatorBalanceUnits(n))
		require.NoError(t, e)
		require.Zero(t, n.Cmp(again))
	}
}
func TestOperatorBalanceFingerprintAndNamespace(t *testing.T) {
	r := &operatorRepoStub{}
	s := NewOperatorBalanceService(r, nil, nil)
	req := OperatorBalanceAdjustmentRequest{Operation: "add", Amount: "1", Reason: "test", IdempotencyKey: "request-123"}
	_, e := s.Adjust(context.Background(), 1, 2, req)
	require.NoError(t, e)
	req.Amount = "1.00000000"
	req.Source = "free"
	_, e = s.Adjust(context.Background(), 1, 2, req)
	require.NoError(t, e)
	require.Equal(t, r.commands[0].Fingerprint, r.commands[1].Fingerprint)
	require.Len(t, r.commands[0].OperationID, 31)
	require.NotContains(t, r.commands[0].OperationID, req.IdempotencyKey)
	req.Reason = "different"
	_, e = s.Adjust(context.Background(), 1, 2, req)
	require.NoError(t, e)
	require.Equal(t, r.commands[0].OperationID, r.commands[2].OperationID)
	require.NotEqual(t, r.commands[0].Fingerprint, r.commands[2].Fingerprint)
	_, e = s.Adjust(context.Background(), 3, 2, req)
	require.NoError(t, e)
	require.NotEqual(t, r.commands[0].OperationID, r.commands[3].OperationID)
}

type operatorDirtyCache struct {
	billingCacheWorkerStub
	fail  bool
	reads int
}

func (c *operatorDirtyCache) GetUserBalance(context.Context, int64) (float64, error) {
	c.reads++
	return 999, nil
}
func (c *operatorDirtyCache) InvalidateUserBalance(context.Context, int64) error {
	if c.fail {
		return errors.New("offline")
	}
	return nil
}

type operatorBalanceUserRepo struct {
	UserRepository
	calls int
}

func (r *operatorBalanceUserRepo) GetByID(context.Context, int64) (*User, error) {
	r.calls++
	return &User{Balance: 7}, nil
}
func TestOperatorBalanceCommittedCacheFailureAndReplay(t *testing.T) {
	cache := &operatorDirtyCache{fail: true}
	users := &operatorBalanceUserRepo{}
	billing := &BillingCacheService{cache: cache, userRepo: users}
	repo := &operatorRepoStub{result: &OperatorBalanceAdjustmentResult{OperationID: "op_saved"}}
	svc := NewOperatorBalanceService(repo, billing, nil)
	req := OperatorBalanceAdjustmentRequest{Operation: "add", Amount: "1", Reason: "test", IdempotencyKey: "request-123"}
	result, e := svc.Adjust(context.Background(), 1, 2, req)
	require.NoError(t, e)
	require.False(t, result.CacheSynced)
	balance, e := billing.GetUserBalance(context.Background(), 2)
	require.NoError(t, e)
	require.Equal(t, float64(7), balance)
	require.Zero(t, cache.reads)
	cache.fail = false
	repo.result.Replayed = true
	result, e = svc.Adjust(context.Background(), 1, 2, req)
	require.NoError(t, e)
	require.True(t, result.Replayed)
	require.True(t, result.CacheSynced)
	_, dirty := billing.operatorBalanceDirty.Load(int64(2))
	require.False(t, dirty)
}
func TestOperatorRoleKeepsAdminStrict(t *testing.T) {
	u := &User{Role: RoleOperator}
	require.True(t, u.IsOperator())
	require.False(t, u.IsAdmin())
	role, e := normalizeUserRole(RoleOperator, RoleUser)
	require.NoError(t, e)
	require.Equal(t, RoleOperator, role)
}

type operatorBlockingInvalidation struct {
	billingCacheWorkerStub
	entered, release chan struct{}
}

func (c *operatorBlockingInvalidation) InvalidateUserBalance(context.Context, int64) error {
	close(c.entered)
	<-c.release
	return nil
}
func TestOperatorBalanceOldCacheAcknowledgementKeepsNewDirty(t *testing.T) {
	cache := &operatorBlockingInvalidation{entered: make(chan struct{}), release: make(chan struct{})}
	s := &BillingCacheService{cache: cache}
	s.MarkOperatorBalanceDirty(2)
	done := make(chan error, 1)
	go func() { done <- s.InvalidateUserBalance(context.Background(), 2) }()
	<-cache.entered
	s.MarkOperatorBalanceDirty(2)
	close(cache.release)
	require.NoError(t, <-done)
	_, dirty := s.operatorBalanceDirty.Load(int64(2))
	require.True(t, dirty, "older invalidation cannot clear a later committed adjustment")
}
