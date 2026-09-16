package service

import (
	"context"
	"time"
)

type APIKeyFinancialState struct {
	Balance   float64
	Quota     float64
	QuotaUsed float64
	Status    string
	ExpiresAt *time.Time
}

// APIKeyFinancialStateReader bypasses auth snapshots and reads only primary
// financial/enforcement fields. It never reads a credential column.
type APIKeyFinancialStateReader interface {
	GetAPIKeyFinancialState(context.Context, int64, int64) (*APIKeyFinancialState, error)
}
