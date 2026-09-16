package service

import (
	"context"
	"time"
)

// BalanceCacheFiller optionally supports fenced cache-aside writes without
// changing BillingCache implementations. A nonempty lease must be acquired
// BEFORE reading the DB. Every balance mutation must atomically revoke it.
// Expired, revoked or consumed leases must never become valid again.
type BalanceCacheFiller interface {
	BeginUserBalanceFill(ctx context.Context, userID int64) (string, error)
	FillUserBalance(ctx context.Context, userID int64, balance float64, lease string) error
}

// UserBalanceVersionReader reads the operator generation from the primary DB.
// It is deliberately independent of the auth snapshot and Redis availability.
type UserBalanceVersionReader interface {
	GetUserBalanceVersion(ctx context.Context, userID int64) (balance float64, version int64, err error)
}

// BalanceCacheVersionGuard atomically revokes the value and every fill lease
// before accepting a new operator generation. False rejects an older DB read.
type BalanceCacheVersionGuard interface {
	EnsureUserBalanceVersion(ctx context.Context, userID, version int64) (bool, error)
}

// SubscriptionCacheData represents cached subscription data
type SubscriptionCacheData struct {
	Status       string
	ExpiresAt    time.Time
	DailyUsage   float64
	WeeklyUsage  float64
	MonthlyUsage float64
	Version      int64
}
