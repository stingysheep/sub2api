package service

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
)

const (
	upstreamBalanceRefreshMaxPerCycle  = 12
	upstreamBalanceRefreshLeaderKey    = "upstream:balance:refresh:leader"
	upstreamBalanceAttemptedAtExtraKey = "upstream_balance_attempted_at"
)

type upstreamBalanceRefreshCandidate struct {
	account   Account
	lastRunAt time.Time
}

// RunBalanceRefreshDue reuses the existing billing-probe switch and account opt-in.
// It only adds a sanitized balance observation; it never changes billing inputs.
func (s *UpstreamBillingProbeService) RunBalanceRefreshDue(ctx context.Context) error {
	if s == nil || s.accountRepo == nil || s.accountTestService == nil {
		return nil
	}
	s.balanceCycleMu.Lock()
	defer s.balanceCycleMu.Unlock()

	settings, err := s.getSettings(ctx)
	if err != nil {
		return err
	}
	if !settings.Enabled {
		return nil
	}
	refreshInterval := time.Duration(settings.IntervalMinutes) * time.Minute

	runRelease, acquired, err := s.tryAcquireLeaderLock(ctx, upstreamBalanceRefreshLeaderKey)
	if err != nil {
		return fmt.Errorf("acquire upstream balance refresh leader lock: %w", err)
	}
	if !acquired {
		return nil
	}
	defer runRelease()

	lockNow := time.Now()
	cadenceKey := fmt.Sprintf("%s:%d", upstreamBalanceRefreshLeaderKey, lockNow.Unix()/int64(upstreamBillingProbeCycleInterval/time.Second))
	cadenceRelease, acquired, err := s.tryAcquireLeaderLock(ctx, cadenceKey)
	if err != nil {
		return fmt.Errorf("acquire upstream balance refresh cadence lock: %w", err)
	}
	if !acquired {
		return nil
	}
	defer releaseUpstreamBillingProbeLeaderLock(cadenceRelease, lockNow.Truncate(upstreamBillingProbeCycleInterval).Add(upstreamBillingProbeCycleInterval))

	now := s.currentTime().UTC()
	accounts, err := s.accountRepo.ListActive(ctx)
	if err != nil {
		return fmt.Errorf("list active accounts for upstream balance refresh: %w", err)
	}
	candidates := make([]upstreamBalanceRefreshCandidate, 0, len(accounts))
	for i := range accounts {
		account := accounts[i]
		if !isUpstreamBalanceRefreshCandidate(&account) {
			continue
		}
		fetchedAt := upstreamBalanceSnapshotFetchedAt(account.Extra)
		attemptedAt := upstreamBalanceAttemptedAt(account.Extra)
		lastRunAt := fetchedAt
		if attemptedAt.After(lastRunAt) {
			lastRunAt = attemptedAt
		}
		if !lastRunAt.IsZero() && now.Sub(lastRunAt) < refreshInterval {
			continue
		}
		candidates = append(candidates, upstreamBalanceRefreshCandidate{account: account, lastRunAt: lastRunAt})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].lastRunAt.IsZero() != candidates[j].lastRunAt.IsZero() {
			return candidates[i].lastRunAt.IsZero()
		}
		if candidates[i].lastRunAt.Equal(candidates[j].lastRunAt) {
			return candidates[i].account.ID < candidates[j].account.ID
		}
		return candidates[i].lastRunAt.Before(candidates[j].lastRunAt)
	})
	if len(candidates) > upstreamBalanceRefreshMaxPerCycle {
		candidates = candidates[:upstreamBalanceRefreshMaxPerCycle]
	}
	if len(candidates) == 0 {
		return nil
	}

	var succeeded atomic.Int64
	var group errgroup.Group
	for i := range candidates {
		account := candidates[i].account
		group.Go(func() error {
			select {
			case s.probeSlots <- struct{}{}:
				defer func() { <-s.probeSlots }()
			case <-ctx.Done():
				return nil
			}
			attemptCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), upstreamBalancePersistTimeout)
			defer cancel()
			if updateErr := s.accountRepo.UpdateExtra(attemptCtx, account.ID, map[string]any{
				upstreamBalanceAttemptedAtExtraKey: now.Format(time.RFC3339Nano),
			}); updateErr != nil {
				slog.Warn("upstream_balance_attempt_persist_failed", "account_id", account.ID)
				return nil
			}
			result, queryErr := s.accountTestService.QueryUpstreamBalance(ctx, &account)
			if queryErr == nil && hasPersistableUpstreamBalanceResult(result) {
				succeeded.Add(1)
			}
			return nil
		})
	}
	_ = group.Wait()
	slog.Info("upstream_balance_refresh_cycle",
		"selected", len(candidates),
		"succeeded", succeeded.Load(),
		"refresh_interval_minutes", settings.IntervalMinutes,
	)
	return nil
}

func isUpstreamBalanceRefreshCandidate(account *Account) bool {
	if !isUpstreamBillingProbeAccount(account) || !account.IsSchedulable() || !upstreamBillingProbeEnabled(account) || account.ParentAccountID != nil || account.IsSyntheticUITest() {
		return false
	}
	return strings.TrimSpace(account.GetCredential("api_key")) != "" && strings.TrimSpace(account.GetCredential("base_url")) != ""
}

func upstreamBalanceAttemptedAt(extra map[string]any) time.Time {
	if extra == nil {
		return time.Time{}
	}
	value, ok := extra[upstreamBalanceAttemptedAtExtraKey].(string)
	if !ok {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}
