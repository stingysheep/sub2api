package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyRepositoryGroupConcurrency_ProjectionAndBounds(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &apiKeyRepository{sql: db}
	mock.ExpectQuery(`SELECT k.id, u.id,.*FROM api_keys k.*JOIN users u ON u.id = k.user_id AND u.deleted_at IS NULL AND u.role = 'user'.*LEFT JOIN groups g ON g.id = k.group_id AND g.deleted_at IS NULL.*WHERE k.deleted_at IS NULL AND k.id > \$1 ORDER BY k.id ASC LIMIT \$2`).
		WithArgs(int64(500), 500).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "user_label", "group_id", "name", "platform"}).AddRow(501, 1001, "alice@example.com", 10, "A", "openai").AddRow(502, 1002, "bob@example.com", 0, "", ""))
	keys, err := repo.ListGroupConcurrencyKeys(context.Background(), 500, 5000)
	require.NoError(t, err)
	require.Len(t, keys, 2)
	require.Equal(t, int64(501), keys[0].APIKeyID)
	require.Equal(t, int64(1001), keys[0].UserID)
	require.Equal(t, "alice@example.com", keys[0].UserLabel)
	require.Equal(t, int64(0), keys[1].GroupID)
	require.NoError(t, mock.ExpectationsWereMet())
}
func TestAPIKeyRepositoryGroupConcurrency_ReadError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	unavailable := errors.New("database unavailable")
	mock.ExpectQuery("SELECT k.id").WithArgs(int64(0), 500).WillReturnError(unavailable)
	_, err = (&apiKeyRepository{sql: db}).ListGroupConcurrencyKeys(context.Background(), 0, 0)
	require.ErrorIs(t, err, unavailable)
	require.NoError(t, mock.ExpectationsWereMet())
}
