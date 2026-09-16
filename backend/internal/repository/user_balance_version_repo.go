package repository

import (
	"context"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

var _ service.UserBalanceVersionReader = (*userRepository)(nil)

// GetUserBalanceVersion performs one bounded primary-key lookup. It must not
// use an auth snapshot or replica: operator commits are the authority here.
func (r *userRepository) GetUserBalanceVersion(ctx context.Context, userID int64) (float64, int64, error) {
	rows, err := clientFromContext(ctx, r.client).QueryContext(ctx,
		`SELECT balance, operator_balance_cache_version FROM users WHERE id=$1 AND deleted_at IS NULL`, userID)
	if err != nil {
		return 0, 0, fmt.Errorf("read authoritative balance: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, 0, err
		}
		return 0, 0, service.ErrUserNotFound
	}
	var balance float64
	var version int64
	if err := rows.Scan(&balance, &version); err != nil {
		return 0, 0, err
	}
	return balance, version, rows.Err()
}
