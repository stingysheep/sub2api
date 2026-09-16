package repository

import (
	"context"
	"encoding/json"
	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func operatorMock(t *testing.T) (*operatorBalanceRepository, sqlmock.Sqlmock) {
	db, m, e := sqlmock.New()
	require.NoError(t, e)
	t.Cleanup(func() { _ = db.Close() })
	return &operatorBalanceRepository{dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))}, m
}
func operatorLock(m sqlmock.Sqlmock, b, f, p string, actorRole, targetRole string) {
	m.ExpectBegin()
	m.ExpectQuery("SELECT id, role, status.*ORDER BY id FOR UPDATE").WithArgs(int64(1), int64(2)).WillReturnRows(sqlmock.NewRows([]string{"id", "role", "status", "balance", "free_balance", "paid_balance", "free_balance_issued", "total_recharged"}).AddRow(1, actorRole, "active", "0", "0", "0", "0", "0").AddRow(2, targetRole, "disabled", b, f, p, "10", "20"))
}
func operatorCommand(op, amount, source string) service.OperatorBalanceCommand {
	return service.OperatorBalanceCommand{ActorID: 1, TargetID: 2, OperationID: "op_abcdefghijklmnopqrstuvwxyz23", Fingerprint: strings.Repeat("a", 64), Request: service.OperatorBalanceAdjustmentRequest{Operation: op, Amount: amount, Source: source, Reason: "test"}}
}
func TestOperatorBalanceTransactionSourceAndDeficit(t *testing.T) {
	for _, tc := range []struct{ b, f, p, op, a, source, nb, nf, np, issued, total string }{
		{"-10", "0", "-10", "add", "3", "paid", "-7.00000000", "0.00000000", "-7.00000000", "10.00000000", "23.00000000"},
		{"-10", "0", "-10", "add", "3", "free", "-7.00000000", "3.00000000", "-10.00000000", "13.00000000", "23.00000000"},
		{"5", "-2", "7", "subtract", "3", "", "2.00000000", "-2.00000000", "4.00000000", "10.00000000", "20.00000000"},
		{"5", "3", "2", "subtract", "4", "", "1.00000000", "0.00000000", "1.00000000", "10.00000000", "20.00000000"},
		{"5", "1", "1", "subtract", "1", "", "4.00000000", "0.00000000", "1.00000000", "10.00000000", "20.00000000"},
	} {
		t.Run(tc.op+tc.b+tc.source+tc.f, func(t *testing.T) {
			r, m := operatorMock(t)
			operatorLock(m, tc.b, tc.f, tc.p, "operator", "user")
			m.ExpectQuery("SELECT notes FROM redeem_codes").WillReturnRows(sqlmock.NewRows([]string{"notes"}))
			m.ExpectExec("UPDATE users SET balance").WithArgs(tc.nb, tc.nf, tc.np, tc.issued, tc.total, int64(2), "user").WillReturnResult(sqlmock.NewResult(0, 1))
			n := 1
			if tc.f == "3" && tc.a == "4" {
				n = 2
			}
			for i := 0; i < n; i++ {
				m.ExpectExec("INSERT INTO redeem_codes").WillReturnResult(sqlmock.NewResult(1, 1))
			}
			m.ExpectCommit()
			got, e := r.AdjustOperatorBalance(context.Background(), operatorCommand(tc.op, tc.a, tc.source))
			require.NoError(t, e)
			require.Equal(t, tc.nb, got.AfterBalance)
			require.NoError(t, m.ExpectationsWereMet())
		})
	}
}
func TestOperatorBalanceRejectsUnauthorizedBeforeReceipt(t *testing.T) {
	r, m := operatorMock(t)
	operatorLock(m, "5", "3", "2", "user", "user")
	m.ExpectRollback()
	_, e := r.AdjustOperatorBalance(context.Background(), operatorCommand("add", "1", "free"))
	require.ErrorIs(t, e, service.ErrOperatorBalanceForbidden)
	require.NoError(t, m.ExpectationsWereMet())
}
func TestOperatorBalanceInsufficientSourceAndOverflow(t *testing.T) {
	for _, tc := range []struct{ b, f, p, op, a, source string }{{"7", "10", "-3", "subtract", "1", ""}, {"5", "1", "1", "subtract", "3", ""}, {"5", "3", "2", "subtract", "6", ""}, {"999999999999.99999999", "0", "0", "add", "0.00000001", "free"}} {
		r, m := operatorMock(t)
		operatorLock(m, tc.b, tc.f, tc.p, "operator", "user")
		m.ExpectQuery("SELECT notes FROM redeem_codes").WillReturnRows(sqlmock.NewRows([]string{"notes"}))
		m.ExpectRollback()
		_, e := r.AdjustOperatorBalance(context.Background(), operatorCommand(tc.op, tc.a, tc.source))
		require.Error(t, e)
		require.NoError(t, m.ExpectationsWereMet())
	}
}
func TestOperatorBalanceLedgerFailureRollsBack(t *testing.T) {
	r, m := operatorMock(t)
	operatorLock(m, "5", "3", "2", "operator", "user")
	m.ExpectQuery("SELECT notes FROM redeem_codes").WillReturnRows(sqlmock.NewRows([]string{"notes"}))
	m.ExpectExec("UPDATE users SET balance").WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec("INSERT INTO redeem_codes").WillReturnError(errors.New("ledger offline"))
	m.ExpectRollback()
	_, e := r.AdjustOperatorBalance(context.Background(), operatorCommand("add", "1", "free"))
	require.Error(t, e)
	require.NoError(t, m.ExpectationsWereMet())
}
func TestOperatorBalancePersistentReplayConflict(t *testing.T) {
	for _, fp := range []string{strings.Repeat("a", 64), "different"} {
		r, m := operatorMock(t)
		operatorLock(m, "500", "500", "0", "operator", "user")
		receipt := operatorBalanceReceipt{Version: 1, ActorID: 1, TargetID: 2, Fingerprint: fp, Result: service.OperatorBalanceAdjustmentResult{AfterBalance: "6.00000000"}}
		notes, _ := json.Marshal(receipt)
		m.ExpectQuery("SELECT notes FROM redeem_codes").WillReturnRows(sqlmock.NewRows([]string{"notes"}).AddRow(string(notes)))
		if fp == strings.Repeat("a", 64) {
			m.ExpectCommit()
		} else {
			m.ExpectRollback()
		}
		got, e := r.AdjustOperatorBalance(context.Background(), operatorCommand("add", "1", "free"))
		if fp == strings.Repeat("a", 64) {
			require.NoError(t, e)
			require.True(t, got.Replayed)
			require.Equal(t, "6.00000000", got.AfterBalance)
		} else {
			require.ErrorIs(t, e, service.ErrOperatorBalanceConflict)
		}
		require.NoError(t, m.ExpectationsWereMet())
	}
}
func TestOperatorReceiptReservedCreation(t *testing.T) {
	r := &redeemCodeRepository{}
	for _, c := range []service.RedeemCode{{Code: "normal", Type: service.AdjustmentTypeOperatorBalance}, {Code: "op_any", Type: "balance"}, {Code: "OP_any", Type: "balance"}} {
		require.ErrorIs(t, r.Create(context.Background(), &c), service.ErrOperatorBalanceForbidden)
		require.ErrorIs(t, r.Update(context.Background(), &c), service.ErrOperatorBalanceForbidden)
		require.ErrorIs(t, r.CreateBatch(context.Background(), []service.RedeemCode{c}), service.ErrOperatorBalanceForbidden)
	}
}

func TestOperatorBalanceZeroAffectedCannotIssueReceipt(t *testing.T) {
	r, m := operatorMock(t)
	operatorLock(m, "5", "3", "2", "operator", "user")
	m.ExpectQuery("SELECT notes FROM redeem_codes").WillReturnRows(sqlmock.NewRows([]string{"notes"}))
	m.ExpectExec("UPDATE users SET balance").WillReturnResult(sqlmock.NewResult(0, 0))
	m.ExpectRollback()
	_, e := r.AdjustOperatorBalance(context.Background(), operatorCommand("add", "1", "free"))
	require.ErrorIs(t, e, service.ErrOperatorBalanceUnavailable)
	require.NoError(t, m.ExpectationsWereMet())
}
func TestOperatorBalanceRepositoryRejectsInvalidCommandBeforeTransaction(t *testing.T) {
	r := &operatorBalanceRepository{}
	for _, modify := range []func(*service.OperatorBalanceCommand){func(c *service.OperatorBalanceCommand) { c.Request.Operation = "set" }, func(c *service.OperatorBalanceCommand) { c.Request.Source = "other" }, func(c *service.OperatorBalanceCommand) { c.OperationID = "op_invalid" }, func(c *service.OperatorBalanceCommand) { c.Fingerprint = "invalid" }, func(c *service.OperatorBalanceCommand) { c.Request.Amount = "NaN" }} {
		c := operatorCommand("add", "1", "free")
		modify(&c)
		_, e := r.AdjustOperatorBalance(context.Background(), c)
		require.Error(t, e)
	}
}
func TestOperatorLedgerGenericDeleteAndUseAreAtomicProtected(t *testing.T) {
	for _, use := range []bool{false, true} {
		db, m, e := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(_ string, actual string) error {
			if !strings.Contains(actual, "type") || !strings.Contains(actual, "code") || !strings.Contains(actual, "NOT") {
				return errors.New("missing immutable receipt predicate")
			}
			return nil
		})))
		require.NoError(t, e)
		r := &redeemCodeRepository{dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))}
		m.ExpectExec("protected").WillReturnResult(sqlmock.NewResult(0, 0))
		if use {
			e = r.Use(context.Background(), 12, 2)
			require.ErrorIs(t, e, service.ErrRedeemCodeUsed)
		} else {
			require.NoError(t, r.Delete(context.Background(), 12))
		}
		require.NoError(t, m.ExpectationsWereMet())
		_ = db.Close()
	}
}
