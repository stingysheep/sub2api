package service

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// accountRecoveryCoordinator observes accounts involved in failover without
// issuing another billable upstream request. The channel-monitor subsystem
// remains responsible for active probes.
type accountRecoveryCoordinator struct {
	repo        AccountRepository
	onRecovered func(context.Context, int64)
	mu          sync.Mutex
	active      map[int64]struct{}
	recovered   map[int64]time.Time
}

const recoveredAccountMarkerTTL = 10 * time.Minute

func newAccountRecoveryCoordinator(repo AccountRepository) *accountRecoveryCoordinator {
	if repo == nil {
		return nil
	}
	return &accountRecoveryCoordinator{repo: repo, active: make(map[int64]struct{}), recovered: make(map[int64]time.Time)}
}

// setOnRecovered installs the reconciliation hook used by gateway services.
// The coordinator itself only observes durable account state; the service owns
// refreshing any scheduler snapshot that may have excluded the account while
// its temporary quarantine was active.
func (c *accountRecoveryCoordinator) setOnRecovered(fn func(context.Context, int64)) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.onRecovered = fn
	c.mu.Unlock()
}

func (c *accountRecoveryCoordinator) schedule(accountID int64) {
	if c == nil || c.repo == nil || accountID <= 0 {
		return
	}
	c.mu.Lock()
	if _, ok := c.active[accountID]; ok {
		c.mu.Unlock()
		return
	}
	c.active[accountID] = struct{}{}
	c.mu.Unlock()
	slog.Debug("account_recovery_observation_started", "account_id", accountID)
	go c.observe(accountID)
}

func (c *accountRecoveryCoordinator) observe(accountID int64) {
	defer func() {
		c.mu.Lock()
		delete(c.active, accountID)
		c.mu.Unlock()
	}()
	for _, delay := range []time.Duration{30 * time.Second, time.Minute, 2 * time.Minute, 5 * time.Minute} {
		time.Sleep(delay)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		account, err := c.repo.GetByID(ctx, accountID)
		cancel()
		if err == nil && account != nil && account.IsSchedulable() {
			c.mu.Lock()
			c.recovered[accountID] = time.Now()
			onRecovered := c.onRecovered
			c.mu.Unlock()
			if onRecovered != nil {
				// Recovery is an internal reconciliation task and must not inherit a
				// request context that may have been canceled with the failed call.
				reconcileCtx, reconcileCancel := context.WithTimeout(context.Background(), 5*time.Second)
				onRecovered(reconcileCtx, accountID)
				reconcileCancel()
			}
			slog.Info("account_recovery_observed", "account_id", accountID)
			return
		}
	}
	slog.Debug("account_recovery_observation_expired", "account_id", accountID)
}

func (c *accountRecoveryCoordinator) isRecovered(accountID int64, now time.Time) bool {
	if c == nil || accountID <= 0 {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	markedAt, ok := c.recovered[accountID]
	if !ok {
		return false
	}
	if now.Sub(markedAt) > recoveredAccountMarkerTTL {
		delete(c.recovered, accountID)
		return false
	}
	return true
}
