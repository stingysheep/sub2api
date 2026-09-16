package repository

import (
	"context"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestAPIKeyFinancialStatePrimaryNoCredentialsOwnerAndFailures(t *testing.T) {
	for _, scenario := range []string{"found", "missing", "db_error", "scan_error"} {
		t.Run(scenario, func(t *testing.T) {
			op, m := operatorMock(t)
			repo := &apiKeyRepository{client: op.client}
			q := m.ExpectQuery(`SELECT u.balance,k.quota,k.quota_used,k.status,k.expires_at FROM api_keys k JOIN users u ON u.id=k.user_id AND u.deleted_at IS NULL WHERE k.id=\$1 AND k.user_id=\$2 AND k.deleted_at IS NULL`).WithArgs(int64(3), int64(2))
			expiry := time.Now().UTC().Truncate(time.Second)
			switch scenario {
			case "found":
				q.WillReturnRows(sqlmock.NewRows([]string{"balance", "quota", "quota_used", "status", "expires_at"}).AddRow("4.00000000", "1.00000000", "1.00000000", "quota_exhausted", expiry))
			case "missing":
				q.WillReturnRows(sqlmock.NewRows([]string{"balance", "quota", "quota_used", "status", "expires_at"}))
			case "db_error":
				q.WillReturnError(errors.New("synthetic primary failed"))
			case "scan_error":
				q.WillReturnRows(sqlmock.NewRows([]string{"balance", "quota", "quota_used", "status", "expires_at"}).AddRow("invalid", 1, 1, "active", nil))
			}
			got, err := repo.GetAPIKeyFinancialState(context.Background(), 3, 2)
			if scenario == "found" {
				require.NoError(t, err)
				require.Equal(t, 4.0, got.Balance)
				require.Equal(t, 1.0, got.QuotaUsed)
				require.Equal(t, expiry, *got.ExpiresAt)
			} else {
				require.Nil(t, got)
				require.Error(t, err)
			}
			if scenario == "missing" {
				require.ErrorIs(t, err, service.ErrAPIKeyNotFound)
			}
			require.NoError(t, m.ExpectationsWereMet())
		})
	}
}
