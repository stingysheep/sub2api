package service

import "context"

// PaidBalanceRankingItem is one entry in the administrator's current paid-balance ranking.
type PaidBalanceRankingItem struct {
	Rank        int     `json:"rank"`
	UserID      int64   `json:"user_id"`
	Email       string  `json:"email"`
	Username    string  `json:"username"`
	PaidBalance float64 `json:"paid_balance"`
}

// PaidBalanceSummary contains the current paid balance held by all non-deleted users.
type PaidBalanceSummary struct {
	TotalPaidBalance     float64                  `json:"total_paid_balance"`
	UsersWithPaidBalance int64                    `json:"users_with_paid_balance"`
	Ranking              []PaidBalanceRankingItem `json:"ranking"`
}

// PaidBalanceSummaryReader keeps this read-only report optional for lightweight service test doubles.
type PaidBalanceSummaryReader interface {
	GetPaidBalanceSummary(ctx context.Context, limit int) (*PaidBalanceSummary, error)
}
