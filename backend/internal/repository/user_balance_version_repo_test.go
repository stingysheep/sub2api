package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUserBalanceVersionPrimaryLookupAndFailures(t *testing.T) {
	for _, scenario := range []string{"found", "missing", "db_error", "scan_error"} {
		t.Run(scenario, func(t *testing.T) {
			op, m := operatorMock(t)
			r := &userRepository{client: op.client}
			query := m.ExpectQuery(`SELECT balance, operator_balance_cache_version FROM users WHERE id=\$1 AND deleted_at IS NULL`).WithArgs(int64(2))
			switch scenario {
			case "found":
				query.WillReturnRows(sqlmock.NewRows([]string{"balance", "version"}).AddRow("0.00000001", 17))
			case "missing":
				query.WillReturnRows(sqlmock.NewRows([]string{"balance", "version"}))
			case "db_error":
				query.WillReturnError(errors.New("primary failed"))
			case "scan_error":
				query.WillReturnRows(sqlmock.NewRows([]string{"balance", "version"}).AddRow("invalid", 17))
			}
			balance, version, err := r.GetUserBalanceVersion(context.Background(), 2)
			if scenario == "found" {
				require.NoError(t, err)
				require.Equal(t, 0.00000001, balance)
				require.EqualValues(t, 17, version)
			} else {
				require.Error(t, err)
			}
			if scenario == "missing" {
				require.ErrorIs(t, err, service.ErrUserNotFound)
			}
			require.NoError(t, m.ExpectationsWereMet())
		})
	}
}
