package repository

import (
	"context"
	"database/sql"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

var _ service.APIKeyFinancialStateReader = (*apiKeyRepository)(nil)

func (r *apiKeyRepository) GetAPIKeyFinancialState(ctx context.Context, keyID, userID int64) (*service.APIKeyFinancialState, error) {
	rows, err := clientFromContext(ctx, r.client).QueryContext(ctx, `SELECT u.balance,k.quota,k.quota_used,k.status,k.expires_at FROM api_keys k JOIN users u ON u.id=k.user_id AND u.deleted_at IS NULL WHERE k.id=$1 AND k.user_id=$2 AND k.deleted_at IS NULL`, keyID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, service.ErrAPIKeyNotFound
	}
	var state service.APIKeyFinancialState
	var expiry sql.NullTime
	if err := rows.Scan(&state.Balance, &state.Quota, &state.QuotaUsed, &state.Status, &expiry); err != nil {
		return nil, err
	}
	if expiry.Valid {
		value := expiry.Time
		state.ExpiresAt = &value
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &state, nil
}
