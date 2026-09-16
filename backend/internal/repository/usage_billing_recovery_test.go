package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type recoveryBillingStub struct {
	service.UsageBillingRepository
	mu           sync.Mutex
	committed    map[string]*service.UsageBillingApplyResult
	usage        *recoveryUsageStub
	fingerprints map[string]string
	err          error
	committedErr error
	fenceErr     error
	applyNil     bool
	applySkipped bool
	commitLost   bool
	applyCalls   int
	debits       int
	fences       int
}

func (b *recoveryBillingStub) Apply(_ context.Context, cmd *service.UsageBillingCommand) (*service.UsageBillingApplyResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.applyCalls++
	if b.err != nil {
		return nil, b.err
	}
	if b.applyNil {
		return nil, nil
	}
	if b.applySkipped {
		return &service.UsageBillingApplyResult{}, nil
	}
	if result := b.committed[cmd.RequestID]; result != nil {
		return &service.UsageBillingApplyResult{}, nil
	}
	result := &service.UsageBillingApplyResult{Applied: true, FreeBalanceCost: 0.25, PaidBalanceCost: 0.75}
	b.committed[cmd.RequestID] = result
	b.fingerprints[cmd.RequestID] = cmd.RequestFingerprint
	b.debits++
	if cmd.Recovery {
		b.fences++
	}
	if b.commitLost {
		b.commitLost = false
		return nil, errors.New("synthetic commit ACK lost")
	}
	copy := *result
	return &copy, nil
}

func (b *recoveryBillingStub) CommittedUsageBilling(_ context.Context, cmd *service.UsageBillingCommand) (*service.UsageBillingApplyResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.committedErr != nil {
		return nil, b.committedErr
	}
	if result := b.committed[cmd.RequestID]; result != nil {
		if b.fingerprints[cmd.RequestID] != cmd.RequestFingerprint {
			return nil, service.ErrUsageBillingRequestConflict
		}
		copy := *result
		copy.Applied = false
		return &copy, nil
	}
	return nil, nil
}

func (b *recoveryBillingStub) FenceRecoveredBalance(context.Context, int64) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.fences++
	return b.fenceErr
}

type recoveryUsageStub struct {
	service.UsageLogRepository
	err     error
	rows    map[string]service.UsageLog
	calls   int
	ackLost bool
}

func (u *recoveryUsageStub) Create(_ context.Context, log *service.UsageLog) (bool, error) {
	u.calls++
	if u.err != nil {
		return false, u.err
	}
	_, exists := u.rows[log.RequestID]
	if !exists {
		u.rows[log.RequestID] = *log
	}
	if u.ackLost {
		u.ackLost = false
		return false, errors.New("synthetic usage ACK lost")
	}
	return !exists, nil
}

func recoveryStubs() (*recoveryBillingStub, *recoveryUsageStub) {
	usage := &recoveryUsageStub{rows: make(map[string]service.UsageLog)}
	return &recoveryBillingStub{committed: make(map[string]*service.UsageBillingApplyResult), fingerprints: make(map[string]string), usage: usage}, usage
}

func (b *recoveryBillingStub) VerifyCommittedUsage(_ context.Context, cmd *service.UsageBillingCommand, log *service.UsageLog) error {
	stored, ok := b.usage.rows[cmd.RequestID]
	if !ok {
		return errors.New("synthetic usage not confirmed")
	}
	if stored.ActualCost != log.ActualCost || stored.TotalCost != log.TotalCost || stored.FreeBalanceCost != log.FreeBalanceCost || stored.PaidBalanceCost != log.PaidBalanceCost || stored.UnfundedBalanceCost != log.UnfundedBalanceCost {
		return errUsageRecoveryConflict
	}
	return nil
}

func recoveryCommand(id string) (*service.UsageBillingCommand, *service.UsageLog) {
	cmd := &service.UsageBillingCommand{RequestID: id, UserID: 2, APIKeyID: 3, AccountID: 4, Model: "synthetic-model", BalanceCost: 1, APIKeyQuotaCost: 1, RequestPayloadHash: strings.Repeat("a", 64)}
	log := &service.UsageLog{RequestID: id, UserID: 2, APIKeyID: 3, AccountID: 4, Model: "synthetic-model", ActualCost: 1, TotalCost: 0.8}
	cmd.Normalize()
	return cmd, log
}

func recoverNow(t *testing.T, r *usageBillingRecoveryRepository) {
	t.Helper()
	_, err := r.journal.Exec("UPDATE billing_recovery SET next_at=0 WHERE state='pending'")
	require.NoError(t, err)
	require.NoError(t, r.RecoverOnce(context.Background()))
}

func recoveryCount(t *testing.T, r *usageBillingRecoveryRepository, state string) int {
	t.Helper()
	var count int
	require.NoError(t, r.journal.QueryRow("SELECT count(*) FROM billing_recovery WHERE state=?", state).Scan(&count))
	return count
}

func TestUsageBillingRecovery_ReopenApplyFailureCommitAndUsageAckLoss(t *testing.T) {
	for _, failure := range []string{"apply_failed", "commit_ack_lost", "usage_failed", "usage_ack_lost"} {
		t.Run(failure, func(t *testing.T) {
			base, usage := recoveryStubs()
			path := filepath.Join(recoveryFixtureDir(t), "recovery.sqlite")
			r, err := openUsageBillingRecovery(base, usage, path, 10, 1024*1024)
			require.NoError(t, err)
			cmd, log := recoveryCommand("synthetic-" + failure)
			switch failure {
			case "apply_failed":
				base.err = errors.New("synthetic PG unavailable")
			case "commit_ack_lost":
				base.commitLost = true
			case "usage_failed":
				usage.err = errors.New("synthetic usage unavailable")
			case "usage_ack_lost":
				usage.ackLost = true
			}
			_, err = r.ApplyWithUsage(context.Background(), cmd, log)
			require.ErrorIs(t, err, service.ErrUsageBillingRecoveryRequired)
			require.Equal(t, 1, recoveryCount(t, r, "pending"))
			r.Close()
			base.err, usage.err = nil, nil
			r, err = openUsageBillingRecovery(base, usage, path, 10, 1024*1024)
			require.NoError(t, err)
			defer r.Close()
			recoverNow(t, r)
			require.Equal(t, 1, base.debits, "money effect must remain exactly once across reopen and lost ACK")
			require.Equal(t, 1, base.fences)
			require.Len(t, usage.rows, 1)
			require.Equal(t, 1.0, usage.rows[cmd.RequestID].ActualCost)
			require.Equal(t, 0.25, usage.rows[cmd.RequestID].FreeBalanceCost)
			require.Equal(t, 0.75, usage.rows[cmd.RequestID].PaidBalanceCost)
			require.Zero(t, recoveryCount(t, r, "pending"))
			recoverNow(t, r)
			require.Equal(t, 1, base.debits)
		})
	}
}

func TestUsageBillingRecovery_ForegroundCommittedRetryFencesBeforeUsageAck(t *testing.T) {
	base, usage := recoveryStubs()
	r, err := openUsageBillingRecovery(base, usage, filepath.Join(recoveryFixtureDir(t), "foreground.sqlite"), 10, 1024*1024)
	require.NoError(t, err)
	defer r.Close()
	cmd, log := recoveryCommand("synthetic-foreground-ack-lost")
	base.commitLost = true
	_, err = r.ApplyWithUsage(context.Background(), cmd, log)
	require.ErrorIs(t, err, service.ErrUsageBillingRecoveryRequired)

	result, err := r.ApplyWithUsage(context.Background(), cmd, log)
	require.NoError(t, err)
	require.False(t, result.Applied, "a recovered commit must not appear as a new Apply")
	require.Equal(t, 1, base.debits)
	require.Equal(t, 1, base.fences)
	require.Len(t, usage.rows, 1)
	require.Zero(t, recoveryCount(t, r, "pending"))
}

func TestUsageBillingRecovery_ForegroundFenceFailureRetainsIntentForWorker(t *testing.T) {
	base, usage := recoveryStubs()
	r, err := openUsageBillingRecovery(base, usage, filepath.Join(recoveryFixtureDir(t), "foreground-fence.sqlite"), 10, 1024*1024)
	require.NoError(t, err)
	defer r.Close()
	cmd, log := recoveryCommand("synthetic-foreground-fence-failure")
	base.commitLost = true
	_, err = r.ApplyWithUsage(context.Background(), cmd, log)
	require.ErrorIs(t, err, service.ErrUsageBillingRecoveryRequired)

	base.fenceErr = errors.New("synthetic cache fence unavailable")
	_, err = r.ApplyWithUsage(context.Background(), cmd, log)
	require.ErrorIs(t, err, service.ErrUsageBillingRecoveryRequired)
	require.Equal(t, 1, recoveryCount(t, r, "pending"))
	require.Empty(t, usage.rows)
	require.Equal(t, 1, base.debits)

	base.fenceErr = nil
	recoverNow(t, r)
	require.Zero(t, recoveryCount(t, r, "pending"))
	require.Len(t, usage.rows, 1)
	require.Equal(t, 1, base.debits)
}

func TestUsageBillingRecovery_ForegroundFenceOnlyConfirmedPositiveBalance(t *testing.T) {
	for _, tc := range []struct {
		name              string
		balanceCost       float64
		subscriptionCost  float64
		commitLost        bool
		committedQueryErr error
		wantFence         int
	}{
		{name: "new_applied", balanceCost: 1, wantFence: 0},
		{name: "zero_balance_recovered", balanceCost: 0, commitLost: true, wantFence: 0},
		{name: "subscription_without_balance", balanceCost: 0, subscriptionCost: 1, commitLost: true, wantFence: 0},
		{name: "nil_or_not_applied_query_error", balanceCost: 1, commitLost: true, committedQueryErr: errors.New("synthetic committed lookup unavailable"), wantFence: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base, usage := recoveryStubs()
			r, err := openUsageBillingRecovery(base, usage, filepath.Join(recoveryFixtureDir(t), "foreground-fence-conditions.sqlite"), 10, 1024*1024)
			require.NoError(t, err)
			defer r.Close()
			cmd, log := recoveryCommand("synthetic-foreground-fence-" + tc.name)
			cmd.BalanceCost = tc.balanceCost
			cmd.SubscriptionCost = tc.subscriptionCost
			cmd.RequestFingerprint = ""
			cmd.Normalize()
			base.commitLost = tc.commitLost
			if tc.commitLost {
				_, err = r.ApplyWithUsage(context.Background(), cmd, log)
				require.ErrorIs(t, err, service.ErrUsageBillingRecoveryRequired)
			}
			base.committedErr = tc.committedQueryErr
			_, err = r.ApplyWithUsage(context.Background(), cmd, log)
			if tc.committedQueryErr != nil {
				require.ErrorIs(t, err, service.ErrUsageBillingRecoveryRequired)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.wantFence, base.fences)
		})
	}
}

func TestUsageBillingRecovery_UnconfirmedNilOrSkippedApplyNeverFences(t *testing.T) {
	for _, mode := range []string{"nil", "not_applied"} {
		t.Run(mode, func(t *testing.T) {
			base, usage := recoveryStubs()
			r, err := openUsageBillingRecovery(base, usage, filepath.Join(recoveryFixtureDir(t), "unconfirmed-apply.sqlite"), 10, 1024*1024)
			require.NoError(t, err)
			defer r.Close()
			cmd, log := recoveryCommand("synthetic-unconfirmed-" + mode)
			base.committedErr = errors.New("synthetic committed lookup unavailable")
			base.applyNil = mode == "nil"
			base.applySkipped = mode == "not_applied"
			_, err = r.ApplyWithUsage(context.Background(), cmd, log)
			require.ErrorIs(t, err, service.ErrUsageBillingRecoveryRequired)
			require.Zero(t, base.fences)
			require.Zero(t, base.debits)
			require.Empty(t, usage.rows)
			require.Equal(t, 1, recoveryCount(t, r, "pending"))
		})
	}
}

func TestUsageBillingRecovery_QuarantinedIntentBlocksForegroundReplay(t *testing.T) {
	for _, reason := range []string{"time_dependent_command", "retry_limit", "usage_conflict"} {
		t.Run(reason, func(t *testing.T) {
			base, usage := recoveryStubs()
			r, err := openUsageBillingRecovery(base, usage, filepath.Join(recoveryFixtureDir(t), "quarantine.sqlite"), 10, 1024*1024)
			require.NoError(t, err)
			defer r.Close()
			cmd, log := recoveryCommand("synthetic-quarantine-" + reason)
			require.NoError(t, r.enqueue(context.Background(), cmd, log))
			_, err = r.journal.Exec("UPDATE billing_recovery SET state='quarantine',reason=?,attempts=7 WHERE request_id=?", reason, cmd.RequestID)
			require.NoError(t, err)

			beforeApply, beforeUsage := base.applyCalls, usage.calls
			_, err = r.ApplyWithUsage(context.Background(), cmd, log)
			require.ErrorIs(t, err, service.ErrUsageBillingRecoveryRequired)
			require.ErrorContains(t, err, "quarantined")
			require.Equal(t, beforeApply, base.applyCalls)
			require.Equal(t, beforeUsage, usage.calls)
			var state, gotReason string
			var attempts int
			require.NoError(t, r.journal.QueryRow("SELECT state,reason,attempts FROM billing_recovery WHERE request_id=?", cmd.RequestID).Scan(&state, &gotReason, &attempts))
			require.Equal(t, "quarantine", state)
			require.Equal(t, reason, gotReason)
			require.Equal(t, 7, attempts)
		})
	}
}

func TestUsageBillingRecovery_UnknownIntentStateFailsClosedAndPendingRetries(t *testing.T) {
	t.Run("fingerprint_conflict_precedes_quarantine", func(t *testing.T) {
		base, usage := recoveryStubs()
		r, err := openUsageBillingRecovery(base, usage, filepath.Join(recoveryFixtureDir(t), "quarantine-conflict.sqlite"), 10, 1024*1024)
		require.NoError(t, err)
		defer r.Close()
		cmd, log := recoveryCommand("synthetic-quarantine-conflict")
		require.NoError(t, r.enqueue(context.Background(), cmd, log))
		_, err = r.journal.Exec("UPDATE billing_recovery SET state='quarantine',reason='retry_limit' WHERE request_id=?", cmd.RequestID)
		require.NoError(t, err)
		conflict := *cmd
		conflict.BalanceCost = 2
		conflict.RequestFingerprint = ""
		conflict.Normalize()
		_, err = r.ApplyWithUsage(context.Background(), &conflict, log)
		require.ErrorIs(t, err, service.ErrUsageBillingRequestConflict)
		require.Equal(t, 0, base.applyCalls)
		require.Equal(t, 0, usage.calls)
	})

	t.Run("unknown_state", func(t *testing.T) {
		base, usage := recoveryStubs()
		r, err := openUsageBillingRecovery(base, usage, filepath.Join(recoveryFixtureDir(t), "unknown-state.sqlite"), 10, 1024*1024)
		require.NoError(t, err)
		defer r.Close()
		cmd, log := recoveryCommand("synthetic-unknown-state")
		require.NoError(t, r.enqueue(context.Background(), cmd, log))
		_, err = r.journal.Exec("UPDATE billing_recovery SET state='unexpected' WHERE request_id=?", cmd.RequestID)
		require.NoError(t, err)
		_, err = r.ApplyWithUsage(context.Background(), cmd, log)
		require.ErrorIs(t, err, service.ErrUsageBillingRecoveryRequired)
		require.Equal(t, 0, base.applyCalls)
		require.Equal(t, 0, usage.calls)
	})

	t.Run("pending_same_fingerprint_retries", func(t *testing.T) {
		base, usage := recoveryStubs()
		r, err := openUsageBillingRecovery(base, usage, filepath.Join(recoveryFixtureDir(t), "pending.sqlite"), 10, 1024*1024)
		require.NoError(t, err)
		defer r.Close()
		cmd, log := recoveryCommand("synthetic-pending-retry")
		base.err = errors.New("synthetic unavailable")
		_, err = r.ApplyWithUsage(context.Background(), cmd, log)
		require.ErrorIs(t, err, service.ErrUsageBillingRecoveryRequired)
		base.err = nil
		_, err = r.ApplyWithUsage(context.Background(), cmd, log)
		require.NoError(t, err)
		require.Equal(t, 2, base.applyCalls)
		require.Len(t, usage.rows, 1)
	})
}

func TestUsageBillingRecovery_PendingTimeDependentRetryQuarantinesBeforeApply(t *testing.T) {
	for _, field := range []string{"subscription", "key_window", "account_window"} {
		t.Run(field, func(t *testing.T) {
			base, usage := recoveryStubs()
			r, err := openUsageBillingRecovery(base, usage, filepath.Join(recoveryFixtureDir(t), "pending-time.sqlite"), 10, 1024*1024)
			require.NoError(t, err)
			defer r.Close()
			cmd, log := recoveryCommand("synthetic-pending-time-" + field)
			switch field {
			case "subscription":
				cmd.SubscriptionCost = 1
			case "key_window":
				cmd.APIKeyRateLimitCost = 1
			case "account_window":
				cmd.AccountQuotaCost = 1
			}
			cmd.RequestFingerprint = ""
			cmd.Normalize()
			base.err = errors.New("synthetic unavailable")
			_, err = r.ApplyWithUsage(context.Background(), cmd, log)
			require.ErrorIs(t, err, service.ErrUsageBillingRecoveryRequired)
			base.err = nil

			_, err = r.ApplyWithUsage(context.Background(), cmd, log)
			require.ErrorIs(t, err, service.ErrUsageBillingRecoveryRequired)
			require.Equal(t, 1, base.applyCalls, "an existing time-dependent intent must never replay Apply")
			require.Empty(t, usage.rows)
			var state, reason string
			require.NoError(t, r.journal.QueryRow("SELECT state,reason FROM billing_recovery WHERE request_id=?", cmd.RequestID).Scan(&state, &reason))
			require.Equal(t, "quarantine", state)
			require.Equal(t, "time_dependent_command", reason)
		})
	}
}

func TestUsageBillingRecovery_NewTimeCommandAndCommittedPendingRetry(t *testing.T) {
	for _, field := range []string{"subscription", "key_window", "account_window"} {
		t.Run(field, func(t *testing.T) {
			base, usage := recoveryStubs()
			r, err := openUsageBillingRecovery(base, usage, filepath.Join(recoveryFixtureDir(t), "new-or-committed-time.sqlite"), 10, 1024*1024)
			require.NoError(t, err)
			defer r.Close()
			setCost := func(cmd *service.UsageBillingCommand) {
				switch field {
				case "subscription":
					cmd.SubscriptionCost = 1
				case "key_window":
					cmd.APIKeyRateLimitCost = 1
				case "account_window":
					cmd.AccountQuotaCost = 1
				}
				cmd.RequestFingerprint = ""
				cmd.Normalize()
			}

			cmd, log := recoveryCommand("synthetic-new-time-" + field)
			setCost(cmd)
			_, err = r.ApplyWithUsage(context.Background(), cmd, log)
			require.NoError(t, err, "a new time-dependent intent remains a normal first Apply")
			require.Equal(t, 1, base.applyCalls)

			cmd, log = recoveryCommand("synthetic-committed-time-" + field)
			setCost(cmd)
			base.commitLost = true
			_, err = r.ApplyWithUsage(context.Background(), cmd, log)
			require.ErrorIs(t, err, service.ErrUsageBillingRecoveryRequired)
			before := base.applyCalls
			result, err := r.ApplyWithUsage(context.Background(), cmd, log)
			require.NoError(t, err)
			require.False(t, result.Applied)
			require.Equal(t, before, base.applyCalls, "a known committed intent only writes usage")
			require.Contains(t, usage.rows, cmd.RequestID)
		})
	}
}

func TestUsageBillingRecovery_PendingCommittedLookupFailureDoesNotApply(t *testing.T) {
	base, usage := recoveryStubs()
	r, err := openUsageBillingRecovery(base, usage, filepath.Join(recoveryFixtureDir(t), "pending-query-error.sqlite"), 10, 1024*1024)
	require.NoError(t, err)
	defer r.Close()
	cmd, log := recoveryCommand("synthetic-pending-query-error")
	base.err = errors.New("synthetic unavailable")
	_, err = r.ApplyWithUsage(context.Background(), cmd, log)
	require.ErrorIs(t, err, service.ErrUsageBillingRecoveryRequired)
	base.err = nil
	base.committedErr = errors.New("synthetic committed lookup unavailable")
	before := base.applyCalls
	_, err = r.ApplyWithUsage(context.Background(), cmd, log)
	require.ErrorIs(t, err, service.ErrUsageBillingRecoveryRequired)
	require.Equal(t, before, base.applyCalls)
	require.Empty(t, usage.rows)
}

func TestUsageBillingRecovery_LateUsageAckCannotDeleteQuarantine(t *testing.T) {
	base, usage := recoveryStubs()
	r, err := openUsageBillingRecovery(base, usage, filepath.Join(recoveryFixtureDir(t), "late-ack.sqlite"), 10, 1024*1024)
	require.NoError(t, err)
	defer r.Close()
	cmd, log := recoveryCommand("synthetic-late-quarantine-ack")
	require.NoError(t, r.enqueue(context.Background(), cmd, log))
	_, err = r.journal.Exec("UPDATE billing_recovery SET state='quarantine',reason='usage_conflict' WHERE request_id=?", cmd.RequestID)
	require.NoError(t, err)
	err = r.persistUsage(context.Background(), cmd, log, &service.UsageBillingApplyResult{FreeBalanceCost: 0.25, PaidBalanceCost: 0.75})
	require.ErrorContains(t, err, "no longer pending")
	require.Contains(t, usage.rows, cmd.RequestID)
	var state, reason string
	require.NoError(t, r.journal.QueryRow("SELECT state,reason FROM billing_recovery WHERE request_id=?", cmd.RequestID).Scan(&state, &reason))
	require.Equal(t, "quarantine", state)
	require.Equal(t, "usage_conflict", reason)
}

func TestUsageBillingRecovery_ConcurrentDeletedAckRequiresVerifiedUsage(t *testing.T) {
	base, usage := recoveryStubs()
	r, err := openUsageBillingRecovery(base, usage, filepath.Join(recoveryFixtureDir(t), "missing-ack.sqlite"), 10, 1024*1024)
	require.NoError(t, err)
	defer r.Close()
	cmd, log := recoveryCommand("synthetic-missing-ack")
	usage.rows[cmd.RequestID] = service.UsageLog{RequestID: cmd.RequestID, ActualCost: log.ActualCost, TotalCost: log.TotalCost, FreeBalanceCost: 0.25, PaidBalanceCost: 0.75}
	require.NoError(t, r.persistUsage(context.Background(), cmd, log, &service.UsageBillingApplyResult{FreeBalanceCost: 0.25, PaidBalanceCost: 0.75}))
}

func TestUsageBillingRecovery_TimeCommandsQuarantineUnlessAlreadyCommitted(t *testing.T) {
	for _, field := range []string{"subscription", "key_window", "account_window"} {
		for _, committed := range []bool{false, true} {
			t.Run(field+map[bool]string{false: "_failed", true: "_committed"}[committed], func(t *testing.T) {
				base, usage := recoveryStubs()
				r, err := openUsageBillingRecovery(base, usage, filepath.Join(recoveryFixtureDir(t), "recovery.sqlite"), 10, 1024*1024)
				require.NoError(t, err)
				defer r.Close()
				cmd, log := recoveryCommand("synthetic-time")
				switch field {
				case "subscription":
					cmd.SubscriptionCost = 1
				case "key_window":
					cmd.APIKeyRateLimitCost = 1
				case "account_window":
					cmd.AccountQuotaCost = 1
				}
				cmd.RequestFingerprint = ""
				cmd.Normalize()
				if committed {
					base.commitLost = true
				} else {
					base.err = errors.New("synthetic PG unavailable")
				}
				_, err = r.ApplyWithUsage(context.Background(), cmd, log)
				require.Error(t, err)
				base.err = nil
				recoverNow(t, r)
				if committed {
					require.Zero(t, recoveryCount(t, r, "quarantine"))
					require.Len(t, usage.rows, 1)
					require.Equal(t, 1, base.debits)
				} else {
					require.Equal(t, 1, recoveryCount(t, r, "quarantine"))
					require.Zero(t, base.debits)
					require.Empty(t, usage.rows)
				}
				require.Equal(t, 1, base.applyCalls, "time-dependent commands never run twice")
			})
		}
	}
}

func TestUsageBillingRecovery_CapacityReadonlyDiskFullFailBeforeBilling(t *testing.T) {
	for _, failure := range []string{"capacity", "readonly", "disk_full"} {
		t.Run(failure, func(t *testing.T) {
			base, usage := recoveryStubs()
			r, err := openUsageBillingRecovery(base, usage, filepath.Join(recoveryFixtureDir(t), "recovery.sqlite"), 1, 1024*1024)
			require.NoError(t, err)
			defer r.Close()
			if failure == "capacity" {
				cmd, log := recoveryCommand("synthetic-first")
				base.err = errors.New("synthetic unavailable")
				_, err = r.ApplyWithUsage(context.Background(), cmd, log)
				require.Error(t, err)
			}
			if failure == "readonly" {
				_, err = r.journal.Exec("PRAGMA query_only=ON")
				require.NoError(t, err)
			}
			if failure == "disk_full" {
				var pages int
				require.NoError(t, r.journal.QueryRow("PRAGMA page_count").Scan(&pages))
				_, err = r.journal.Exec("PRAGMA max_page_count=" + fmtInt(pages))
				require.NoError(t, err)
			}
			before := base.applyCalls
			cmd, log := recoveryCommand("synthetic-second")
			if failure == "disk_full" {
				log.RequestedModel = strings.Repeat("x", 20000)
			}
			_, err = r.ApplyWithUsage(context.Background(), cmd, log)
			require.ErrorIs(t, err, service.ErrUsageBillingRecoveryRequired)
			if failure == "disk_full" {
				require.ErrorContains(t, err, "full")
			}
			require.Equal(t, before, base.applyCalls, "failed durable insert must not call money Apply")
		})
	}
}

func fmtInt(n int) string { return strconv.Itoa(n) }

func TestUsageBillingRecovery_SensitiveAllowlistCorruptionConflictAndRetryLimit(t *testing.T) {
	base, usage := recoveryStubs()
	r, err := openUsageBillingRecovery(base, usage, filepath.Join(recoveryFixtureDir(t), "recovery.sqlite"), 10, 1024*1024)
	require.NoError(t, err)
	defer r.Close()
	cmd, log := recoveryCommand("synthetic-sensitive")
	ua, ip, session := "SYNTHETIC_UA_CANARY", "192.0.2.123", "SYNTHETIC_SESSION_CANARY"
	log.UserAgent = &ua
	log.IPAddress = &ip
	log.SessionID = &session
	log.APIKey = &service.APIKey{Key: "SYNTHETIC_KEY_CANARY"}
	base.err = errors.New("synthetic unavailable")
	_, err = r.ApplyWithUsage(context.Background(), cmd, log)
	require.Error(t, err)
	var payload []byte
	require.NoError(t, r.journal.QueryRow("SELECT payload FROM billing_recovery").Scan(&payload))
	for _, denied := range []string{ua, ip, session, "UserAgent", "IPAddress", "SessionID", "SYNTHETIC_KEY_CANARY", strings.Repeat("a", 64)} {
		require.NotContains(t, string(payload), denied)
	}
	conflict := *cmd
	conflict.BalanceCost = 2
	conflict.RequestFingerprint = ""
	conflict.Normalize()
	before := base.applyCalls
	_, err = r.ApplyWithUsage(context.Background(), &conflict, log)
	require.ErrorIs(t, err, service.ErrUsageBillingRequestConflict)
	require.Equal(t, before, base.applyCalls)
	_, err = r.journal.Exec("UPDATE billing_recovery SET payload=? WHERE request_id=?", []byte(`{"Command":{"BalanceCost":999}}`), cmd.RequestID)
	require.NoError(t, err)
	base.err = nil
	recoverNow(t, r)
	require.Equal(t, 1, recoveryCount(t, r, "quarantine"))
	require.Zero(t, base.debits)
	cmd, log = recoveryCommand("synthetic-retry")
	base.err = errors.New("synthetic down")
	_, err = r.ApplyWithUsage(context.Background(), cmd, log)
	require.Error(t, err)
	for range 11 {
		recoverNow(t, r)
	}
	require.Equal(t, 2, recoveryCount(t, r, "quarantine"))
	before = base.applyCalls
	recoverNow(t, r)
	require.Equal(t, before, base.applyCalls)
}

func TestUsageBillingRecovery_AbnormalProcessExitPreservesCommittedJournal(t *testing.T) {
	if path := os.Getenv("BILLING_RECOVERY_CRASH_PATH"); path != "" {
		base, usage := recoveryStubs()
		r, err := openUsageBillingRecovery(base, usage, path, 10, 1024*1024)
		if err != nil {
			os.Exit(24)
		}
		cmd, log := recoveryCommand("synthetic-abnormal-exit")
		if r.enqueue(context.Background(), cmd, log) != nil {
			os.Exit(25)
		}
		os.Exit(23) // no Close/checkpoint/destructor
	}
	path := filepath.Join(recoveryFixtureDir(t), "crash.sqlite")
	child := exec.Command(os.Args[0], "-test.run=^TestUsageBillingRecovery_AbnormalProcessExitPreservesCommittedJournal$")
	child.Env = append(os.Environ(), "BILLING_RECOVERY_CRASH_PATH="+path)
	err := child.Run()
	var exit *exec.ExitError
	require.ErrorAs(t, err, &exit)
	require.Equal(t, 23, exit.ExitCode())
	base, usage := recoveryStubs()
	r, err := openUsageBillingRecovery(base, usage, path, 10, 1024*1024)
	require.NoError(t, err)
	defer r.Close()
	require.Equal(t, 1, recoveryCount(t, r, "pending"))
	recoverNow(t, r)
	require.Equal(t, 1, base.debits)
	require.Len(t, usage.rows, 1)
}

func TestUsageBillingRecovery_CorruptPhysicalFileRejectsStartup(t *testing.T) {
	path := filepath.Join(recoveryFixtureDir(t), "corrupt.sqlite")
	require.NoError(t, os.WriteFile(path, []byte("synthetic corrupt SQLite data"), 0600))
	base, usage := recoveryStubs()
	_, err := openUsageBillingRecovery(base, usage, path, 10, 1024*1024)
	require.Error(t, err)
}

func TestUsageBillingRecovery_ExistingZeroCostDuplicateQuarantinesWithoutAck(t *testing.T) {
	base, usage := recoveryStubs()
	r, err := openUsageBillingRecovery(base, usage, filepath.Join(recoveryFixtureDir(t), "duplicate.sqlite"), 10, 1024*1024)
	require.NoError(t, err)
	defer r.Close()
	cmd, log := recoveryCommand("synthetic-zero-cost")
	usage.rows[cmd.RequestID] = service.UsageLog{RequestID: cmd.RequestID, ActualCost: 0}
	_, err = r.ApplyWithUsage(context.Background(), cmd, log)
	require.ErrorIs(t, err, errUsageRecoveryConflict)
	require.Equal(t, 1, recoveryCount(t, r, "quarantine"))
	require.Equal(t, 1, base.debits)
	recoverNow(t, r)
	require.Equal(t, 1, base.debits)
	require.Zero(t, usage.rows[cmd.RequestID].ActualCost, "historical conflicting usage is preserved for manual review")
}

func TestUsageBillingRecovery_TwoSameJournalWorkersClaimOnce(t *testing.T) {
	base, usage := recoveryStubs()
	path := filepath.Join(recoveryFixtureDir(t), "shared.sqlite")
	first, err := openUsageBillingRecovery(base, usage, path, 10, 1024*1024)
	require.NoError(t, err)
	defer first.Close()
	second, err := openUsageBillingRecovery(base, usage, path, 10, 1024*1024)
	require.NoError(t, err)
	defer second.Close()
	cmd, log := recoveryCommand("synthetic-shared-claim")
	require.NoError(t, first.enqueue(context.Background(), cmd, log))
	_, err = first.journal.Exec("UPDATE billing_recovery SET next_at=0 WHERE request_id=?", cmd.RequestID)
	require.NoError(t, err)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, r := range []*usageBillingRecoveryRepository{first, second} {
		wg.Add(1)
		go func(r *usageBillingRecoveryRepository) { defer wg.Done(); errs <- r.RecoverOnce(context.Background()) }(r)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, 1, base.applyCalls)
	require.Equal(t, 1, base.debits)
	require.Equal(t, 1, base.fences)
	require.Equal(t, 1, usage.calls)
	require.Len(t, usage.rows, 1)
}

func BenchmarkUsageBillingRecoveryDurableIntent(b *testing.B) {
	base, usage := recoveryStubs()
	r, err := openUsageBillingRecovery(base, usage, filepath.Join(recoveryFixtureDir(b), "benchmark.sqlite"), 10000, 64*1024*1024)
	if err != nil {
		b.Fatal(err)
	}
	defer r.Close()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd, log := recoveryCommand(fmt.Sprintf("synthetic-benchmark-%d", i))
		if _, err := r.ApplyWithUsage(context.Background(), cmd, log); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	b.ReportMetric(2, "sqlite_commit/op")
}

func TestUsageBillingRecovery_ExistingJournalCapacityShrinkRejectsStartup(t *testing.T) {
	base, usage := recoveryStubs()
	path := filepath.Join(recoveryFixtureDir(t), "capacity.sqlite")
	r, err := openUsageBillingRecovery(base, usage, path, 100, 1024*1024)
	require.NoError(t, err)
	for i := 0; i < 5; i++ {
		cmd, log := recoveryCommand(fmt.Sprintf("synthetic-large-%d", i))
		log.RequestedModel = strings.Repeat("x", 10000)
		require.NoError(t, r.enqueue(context.Background(), cmd, log))
	}
	r.Close()
	_, err = openUsageBillingRecovery(base, usage, path, 100, 32768)
	require.ErrorContains(t, err, "exceeds capacity")
}

// Durable fixtures are synthetic and preserved. No recursive test cleanup is used.
func recoveryFixtureDir(t interface {
	Helper()
	Name() string
}) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "..", ".local", "autonomous-backlog", "billing-test-fixtures", fmt.Sprintf("fixture_%d", time.Now().UnixNano())))
	if err != nil {
		panic(err)
	}
	if err := os.MkdirAll(path, 0700); err != nil {
		panic(err)
	}
	return path
}
