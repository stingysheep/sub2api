package repository

import (
	"context"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *userRepository) GetPaidBalanceSummary(ctx context.Context, limit int) (*service.PaidBalanceSummary, error) {
	if r.sql == nil {
		return nil, fmt.Errorf("sql executor is not configured")
	}
	if limit < 1 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	const summaryQuery = `
		SELECT
			COALESCE(SUM(CASE WHEN paid_balance > 0 THEN paid_balance ELSE 0 END), 0),
			COUNT(*) FILTER (WHERE paid_balance > 0)
		FROM users
		WHERE deleted_at IS NULL`

	var summary service.PaidBalanceSummary
	summaryRows, err := r.sql.QueryContext(ctx, summaryQuery)
	if err != nil {
		return nil, err
	}
	if !summaryRows.Next() {
		closeErr := summaryRows.Err()
		_ = summaryRows.Close()
		if closeErr != nil {
			return nil, closeErr
		}
		return nil, fmt.Errorf("paid balance summary query returned no rows")
	}
	if err := summaryRows.Scan(&summary.TotalPaidBalance, &summary.UsersWithPaidBalance); err != nil {
		_ = summaryRows.Close()
		return nil, err
	}
	if err := summaryRows.Err(); err != nil {
		_ = summaryRows.Close()
		return nil, err
	}
	if err := summaryRows.Close(); err != nil {
		return nil, err
	}

	const rankingQuery = `
		SELECT id, email, username, paid_balance
		FROM users
		WHERE deleted_at IS NULL AND paid_balance > 0
		ORDER BY paid_balance DESC, id ASC
		LIMIT $1`
	rows, err := r.sql.QueryContext(ctx, rankingQuery, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	summary.Ranking = make([]service.PaidBalanceRankingItem, 0, limit)
	for rows.Next() {
		var item service.PaidBalanceRankingItem
		if err := rows.Scan(&item.UserID, &item.Email, &item.Username, &item.PaidBalance); err != nil {
			return nil, err
		}
		item.Rank = len(summary.Ranking) + 1
		summary.Ranking = append(summary.Ranking, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &summary, nil
}
