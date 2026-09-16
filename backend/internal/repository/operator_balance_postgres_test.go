package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// These checks use only an explicitly named local, disposable test database.
// Each check creates its own synthetic schema and preserves it for inspection.
func operatorPostgres(t *testing.T) (*operatorBalanceRepository, context.Context) {
	t.Helper()
	socket := os.Getenv("OPERATOR_TEST_POSTGRES_SOCKET")
	if socket == "" {
		t.Skip("isolated local PostgreSQL is not configured")
	}
	require.True(t, filepath.Clean(socket) == socket && strings.HasPrefix(socket, "/tmp/sub2api-operator-") && !strings.ContainsAny(socket, " '\"\\\t\r\n"), "refusing a non-test socket")
	dsn := "host=" + socket + " port=55432 user=postgres dbname=sub2api_operator_test sslmode=disable"
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	t.Cleanup(cancel)
	schema := fmt.Sprintf("operator_it_%d", time.Now().UnixNano())
	admin, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	_, err = admin.ExecContext(ctx, "CREATE SCHEMA "+schema)
	require.NoError(t, err)
	require.NoError(t, admin.Close())
	db, err := sql.Open("postgres", dsn+" search_path="+schema)
	require.NoError(t, err)
	db.SetMaxOpenConns(20)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.ExecContext(ctx, `CREATE TABLE users (
		id bigint PRIMARY KEY, role varchar(20) NOT NULL, status varchar(20) NOT NULL,
		balance numeric(20,8) NOT NULL, free_balance numeric(20,8) NOT NULL,
		paid_balance numeric(20,8) NOT NULL, free_balance_issued numeric(20,8) NOT NULL,
		total_recharged numeric(20,8) NOT NULL, operator_balance_cache_version bigint NOT NULL DEFAULT 0, deleted_at timestamptz, updated_at timestamptz,
		email text, username text, created_at timestamptz DEFAULT NOW(), last_active_at timestamptz);
		CREATE TABLE redeem_codes (id bigserial PRIMARY KEY, code varchar(32) NOT NULL UNIQUE,
		type varchar(20) NOT NULL, value numeric(20,8) NOT NULL, balance_source varchar(10) NOT NULL,
		status varchar(20) NOT NULL, used_by bigint REFERENCES users(id), used_at timestamptz,
		notes text, created_at timestamptz NOT NULL, validity_days integer NOT NULL, expires_at timestamptz, group_id bigint);
		INSERT INTO users (id,role,status,balance,free_balance,paid_balance,free_balance_issued,total_recharged,deleted_at,updated_at) VALUES (1,'operator','active',0,0,0,0,0,NULL,NOW()),
		(2,'user','disabled',5,3,2,3,2,NULL,NOW());`)
	require.NoError(t, err)
	return &operatorBalanceRepository{client: dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))}, ctx
}

func TestOperatorPostgresReadScopeAndActualAmounts(t *testing.T) {
	r, ctx := operatorPostgres(t)
	_, err := r.client.ExecContext(ctx, `UPDATE users SET email='synthetic@example.invalid',username='synthetic' WHERE id=2;
		INSERT INTO users(id,role,status,balance,free_balance,paid_balance,free_balance_issued,total_recharged) VALUES(3,'admin','active',0,0,0,0,0);
		CREATE TABLE payment_orders(id bigint PRIMARY KEY,user_id bigint,pay_amount numeric(20,8),provider_snapshot jsonb,status text,created_at timestamptz,paid_at timestamptz);
		CREATE TABLE usage_logs(id bigint PRIMARY KEY,user_id bigint,input_tokens bigint,output_tokens bigint,actual_cost numeric(20,8),created_at timestamptz);
		CREATE TABLE user_affiliate_ledger(id bigint PRIMARY KEY,user_id bigint,action text,amount numeric(20,8),created_at timestamptz);
		CREATE TABLE groups(id bigint PRIMARY KEY,name text,platform text,status text,model_pricing jsonb,updated_at timestamptz,deleted_at timestamptz);
		INSERT INTO payment_orders VALUES(1,2,9.90,'{"currency":"EUR","private_test_field":"canary"}','paid',NOW(),NOW()),(2,2,3.20,'{}','pending',NOW(),NULL),(3,3,999,'{}','paid',NOW(),NOW());
		INSERT INTO usage_logs VALUES(1,2,2,3,1.20,NOW()-INTERVAL '1 minute'),(2,3,999,999,999,NOW()-INTERVAL '1 minute');
		INSERT INTO redeem_codes(code,type,value,balance_source,status,used_by,used_at,notes,created_at,validity_days) VALUES('synthetic','balance',2,'paid','used',2,NOW(),'plain non-JSON notes',NOW(),0);
		INSERT INTO user_affiliate_ledger VALUES(1,2,'transfer',3,NOW()),(2,3,'transfer',999,NOW());
		INSERT INTO groups VALUES(1,'visible','openai','active','[{},{}]',NOW(),NULL),(2,'deleted','openai','active','[]',NOW(),NOW());`)
	require.NoError(t, err)
	read := NewOperatorReadRepository(r.client)
	users, total, err := read.ListUsers(ctx, 1, 20, "")
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, users, 1)
	require.EqualValues(t, 2, users[0].ID)
	users, total, err = read.ListUsers(ctx, 2, 1, "SYNTHETIC")
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Empty(t, users)
	ordinary, err := read.GetUser(ctx, 2)
	require.NoError(t, err)
	require.Equal(t, "synthetic@example.invalid", ordinary.Email)
	_, err = read.GetUser(ctx, 3)
	require.ErrorIs(t, err, service.ErrOperatorUserNotFound)
	orders, total, err := read.ListOrders(ctx, 2, 2, 1)
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, orders, 1)
	require.Equal(t, "9.90000000", orders[0].Amount)
	require.Equal(t, "EUR", orders[0].Currency)
	encoded, err := json.Marshal(orders)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "private_test_field")
	now := time.Now().UTC()
	usage, err := read.Usage(ctx, 2, now.Add(-time.Hour), now)
	require.NoError(t, err)
	require.EqualValues(t, 1, usage.RequestCount)
	require.EqualValues(t, 5, usage.TotalTokens)
	require.Equal(t, "1.20000000", usage.UsageAmount)
	history, total, err := read.ListBalanceHistory(ctx, 2, 1, 20)
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, history, 2)
	for _, item := range history {
		if item.ID < 0 {
			require.Equal(t, "free", item.Source)
		}
	}
	groups, err := read.ListGroups(ctx)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.Equal(t, 2, groups[0].ModelCount)
	_, _, err = read.ListOrders(ctx, 3, 1, 20)
	require.ErrorIs(t, err, service.ErrOperatorUserNotFound)
	_, _, err = read.ListBalanceHistory(ctx, 3, 1, 20)
	require.ErrorIs(t, err, service.ErrOperatorUserNotFound)
	_, err = read.Usage(ctx, 3, now.Add(-time.Hour), now)
	require.ErrorIs(t, err, service.ErrOperatorUserNotFound)
}

func operatorPostgresSnapshot(t *testing.T, r *operatorBalanceRepository, ctx context.Context) (string, int) {
	t.Helper()
	rows, err := r.client.QueryContext(ctx, `SELECT balance::text,(SELECT count(*) FROM redeem_codes) FROM users WHERE id=2`)
	require.NoError(t, err)
	defer rows.Close()
	require.True(t, rows.Next())
	var balance string
	var receipts int
	require.NoError(t, rows.Scan(&balance, &receipts))
	require.NoError(t, rows.Err())
	return balance, receipts
}

func TestOperatorPostgresConcurrentReplayAndRevocation(t *testing.T) {
	r, ctx := operatorPostgres(t)
	svc := service.NewOperatorBalanceService(r, nil, nil)
	req := service.OperatorBalanceAdjustmentRequest{Operation: "add", Amount: "1.25", Source: "paid", Reason: "synthetic concurrency check", IdempotencyKey: "same-operation-123"}
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	results := make(chan *service.OperatorBalanceAdjustmentResult, 16)
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := svc.Adjust(ctx, 1, 2, req)
			errs <- err
			results <- v
		}()
	}
	wg.Wait()
	close(errs)
	close(results)
	for err := range errs {
		require.NoError(t, err)
	}
	firstWrites := 0
	for v := range results {
		require.Equal(t, "6.25000000", v.AfterBalance)
		if !v.Replayed {
			firstWrites++
		}
	}
	require.Equal(t, 1, firstWrites)
	balance, receipts := operatorPostgresSnapshot(t, r, ctx)
	require.Equal(t, "6.25000000", balance)
	require.Equal(t, 1, receipts)
	changed := req
	changed.Amount = "2"
	_, err := svc.Adjust(ctx, 1, 2, changed)
	require.ErrorIs(t, err, service.ErrOperatorBalanceConflict)
	_, err = r.client.ExecContext(ctx, `UPDATE users SET role='user' WHERE id=1`)
	require.NoError(t, err)
	_, err = svc.Adjust(ctx, 1, 2, req)
	require.ErrorIs(t, err, service.ErrOperatorBalanceForbidden, "revocation must be checked before replay")
	_, err = r.client.ExecContext(ctx, `UPDATE users SET role=CASE WHEN id=1 THEN 'operator' ELSE 'admin' END WHERE id IN (1,2)`)
	require.NoError(t, err)
	_, err = svc.Adjust(ctx, 1, 2, req)
	require.ErrorIs(t, err, service.ErrOperatorBalanceForbidden, "a promoted target cannot be adjusted or replayed")
	_, err = r.client.ExecContext(ctx, `UPDATE users SET role='user' WHERE id=2; UPDATE users SET status='disabled' WHERE id=1`)
	require.NoError(t, err)
	_, err = svc.Adjust(ctx, 1, 2, req)
	require.ErrorIs(t, err, service.ErrOperatorBalanceForbidden, "a disabled actor cannot replay a committed operation")
}

func TestOperatorPostgresConcurrentSubtractCannotOverdraw(t *testing.T) {
	r, ctx := operatorPostgres(t)
	svc := service.NewOperatorBalanceService(r, nil, nil)
	var wg sync.WaitGroup
	errs := make(chan error, 10)
	for i := range 10 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := svc.Adjust(ctx, 1, 2, service.OperatorBalanceAdjustmentRequest{Operation: "subtract", Amount: "1", Reason: "synthetic subtraction", IdempotencyKey: fmt.Sprintf("subtract-%08d", i)})
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	success := 0
	for err := range errs {
		if err == nil {
			success++
		} else {
			require.ErrorIs(t, err, service.ErrBalanceNegative)
		}
	}
	require.Equal(t, 5, success)
	balance, receipts := operatorPostgresSnapshot(t, r, ctx)
	require.Equal(t, "0.00000000", balance)
	require.Equal(t, 5, receipts)
}

func TestOperatorPostgresReceiptFailureRollsBackMoney(t *testing.T) {
	r, ctx := operatorPostgres(t)
	_, err := r.client.ExecContext(ctx, `CREATE FUNCTION fail_receipt() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'synthetic receipt failure'; END $$;
		CREATE TRIGGER fail_receipt BEFORE INSERT ON redeem_codes FOR EACH ROW EXECUTE FUNCTION fail_receipt()`)
	require.NoError(t, err)
	svc := service.NewOperatorBalanceService(r, nil, nil)
	_, err = svc.Adjust(ctx, 1, 2, service.OperatorBalanceAdjustmentRequest{Operation: "add", Amount: "1", Source: "paid", Reason: "synthetic rollback", IdempotencyKey: "rollback-operation-123"})
	require.Error(t, err)
	balance, receipts := operatorPostgresSnapshot(t, r, ctx)
	require.Equal(t, "5.00000000", balance)
	require.Zero(t, receipts)
}
