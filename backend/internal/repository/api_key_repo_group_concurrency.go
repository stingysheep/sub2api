package repository

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// ListGroupConcurrencyKeys reads only IDs, admin-visible user labels and group labels;
// no key material, credentials, account pools or usage history are loaded.
func (r *apiKeyRepository) ListGroupConcurrencyKeys(ctx context.Context, afterID int64, limit int) ([]service.GroupConcurrencyKey, error) {
	if limit <= 0 || limit > 500 {
		limit = 500
	}
	rows, err := r.sql.QueryContext(ctx, `SELECT k.id, u.id,
 COALESCE(NULLIF(u.email, ''), NULLIF(u.username, ''), 'user-' || u.id::text),
 COALESCE(g.id, 0), COALESCE(g.name, ''), COALESCE(g.platform, '')
 FROM api_keys k
 JOIN users u ON u.id = k.user_id AND u.deleted_at IS NULL AND u.role = 'user'
 LEFT JOIN groups g ON g.id = k.group_id AND g.deleted_at IS NULL
 WHERE k.deleted_at IS NULL AND k.id > $1 ORDER BY k.id ASC LIMIT $2`, afterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]service.GroupConcurrencyKey, 0, limit)
	for rows.Next() {
		var key service.GroupConcurrencyKey
		if err := rows.Scan(&key.APIKeyID, &key.UserID, &key.UserLabel, &key.GroupID, &key.GroupName, &key.Platform); err != nil {
			return nil, err
		}
		result = append(result, key)
	}
	return result, rows.Err()
}
