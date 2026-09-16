package repository

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestOperatorRelatedReadsUseScopedSnapshot(t *testing.T) {
	for _, kind := range []string{"orders", "history", "usage"} {
		t.Run(kind, func(t *testing.T) {
			b, m := operatorMock(t)
			r := &operatorReadRepository{client: b.client}
			m.ExpectQuery(`WITH target AS \(SELECT id FROM users WHERE id=\$1 AND role='user' AND deleted_at IS NULL\)`).WillReturnRows(sqlmock.NewRows([]string{"empty"}))
			var e error
			switch kind {
			case "orders":
				_, _, e = r.ListOrders(context.Background(), 2, 1, 20)
			case "history":
				_, _, e = r.ListBalanceHistory(context.Background(), 2, 1, 20)
			case "usage":
				_, e = r.Usage(context.Background(), 2, time.Now().Add(-time.Hour), time.Now())
			}
			require.ErrorIs(t, e, service.ErrOperatorUserNotFound)
			require.NoError(t, m.ExpectationsWereMet())
		})
	}
}
func TestOperatorUsageReturnsActualAggregatedCost(t *testing.T) {
	b, m := operatorMock(t)
	r := &operatorReadRepository{client: b.client}
	start, end := time.Now().Add(-time.Hour), time.Now()
	m.ExpectQuery(`SUM\(l.actual_cost\)`).WithArgs(int64(2), start, end).WillReturnRows(sqlmock.NewRows([]string{"count", "input", "output", "amount"}).AddRow(3, 123, 456, "1.25000001"))
	v, e := r.Usage(context.Background(), 2, start, end)
	require.NoError(t, e)
	require.Equal(t, "1.25000001", v.UsageAmount)
	require.EqualValues(t, 579, v.TotalTokens)
	require.NoError(t, m.ExpectationsWereMet())
}
func TestOperatorOrdersReturnActualPaymentAndCurrency(t *testing.T) {
	b, m := operatorMock(t)
	r := &operatorReadRepository{client: b.client}
	now := time.Now()
	m.ExpectQuery(`o.pay_amount::text`).WithArgs(int64(2), 20, 20).WillReturnRows(sqlmock.NewRows([]string{"count", "id", "amount", "currency", "status", "created", "paid"}).AddRow(21, 9, "99.50", "CNY", "paid", now, now))
	v, total, e := r.ListOrders(context.Background(), 2, 2, 20)
	require.NoError(t, e)
	require.EqualValues(t, 21, total)
	require.Equal(t, "99.50", v[0].Amount)
	require.Equal(t, "CNY", v[0].Currency)
	require.NoError(t, m.ExpectationsWereMet())
}
