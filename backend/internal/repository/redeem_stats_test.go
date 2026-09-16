package repository

import (
	"context"
	"errors"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"
)

func TestRedeemStatsAggregatesOnlyRedeemableTypes(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	r := &redeemCodeRepository{client: dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))}
	mock.ExpectQuery(`SELECT .*COUNT\(\*\).*FILTER.*FROM "redeem_codes".*"type" IN.*GROUP BY`).
		WithArgs("balance", "concurrency", "subscription", "invitation").
		WillReturnRows(sqlmock.NewRows([]string{"type", "total", "active", "used", "expired", "distributed"}).
			AddRow("balance", 5, 1, 2, 2, 12.34567891).AddRow("subscription", 2, 1, 1, 0, 0))
	stats, err := r.RedeemStats(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 7, stats.TotalCodes)
	require.EqualValues(t, 2, stats.ActiveCodes)
	require.EqualValues(t, 3, stats.UsedCodes)
	require.EqualValues(t, 2, stats.ExpiredCodes)
	require.Equal(t, 12.34567891, stats.TotalValueDistributed)
	require.EqualValues(t, 5, stats.ByType["balance"])
	require.EqualValues(t, 0, stats.ByType["invitation"])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRedeemStatsEmptyAndQueryFailure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "empty", true: "failure"}[fail], func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			r := &redeemCodeRepository{client: dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))}
			q := mock.ExpectQuery(`SELECT .*FROM "redeem_codes"`).WithArgs("balance", "concurrency", "subscription", "invitation")
			if fail {
				q.WillReturnError(errors.New("offline"))
			} else {
				q.WillReturnRows(sqlmock.NewRows([]string{"type", "total", "active", "used", "expired", "distributed"}))
			}
			stats, err := r.RedeemStats(context.Background())
			if fail {
				require.Error(t, err)
				require.Nil(t, stats)
			} else {
				require.NoError(t, err)
				require.Zero(t, stats.TotalCodes)
				require.Len(t, stats.ByType, 4)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
