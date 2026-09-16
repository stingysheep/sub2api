package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
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

func TestOperatorBalanceVersionPostgresMigrationAtomicReplayAndRollback(t *testing.T) {
	if os.Getenv("BILLING_TEST_POSTGRES_LOCAL") == "" {
		t.Skip("synthetic local PostgreSQL is not configured")
	}
	require.Equal(t, "127.0.0.1:25433", os.Getenv("BILLING_TEST_POSTGRES_LOCAL"), "only the isolated loopback test cluster is allowed")
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	admin, err := sql.Open("postgres", "host=127.0.0.1 port=25433 user=migration_test dbname=postgres sslmode=disable")
	require.NoError(t, err)
	defer admin.Close()
	dbname := fmt.Sprintf("billing_version_it_%d", time.Now().UnixNano())
	_, err = admin.ExecContext(ctx, "CREATE DATABASE "+dbname)
	require.NoError(t, err)
	db, err := sql.Open("postgres", "host=127.0.0.1 port=25433 user=migration_test dbname="+dbname+" sslmode=disable")
	require.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(12)
	_, err = db.ExecContext(ctx, `CREATE TABLE users(id bigint PRIMARY KEY, role text NOT NULL, status text NOT NULL, balance numeric(20,8) NOT NULL, free_balance numeric(20,8) NOT NULL, paid_balance numeric(20,8) NOT NULL, free_balance_issued numeric(20,8) NOT NULL, total_recharged numeric(20,8) NOT NULL, deleted_at timestamptz, updated_at timestamptz);
	CREATE TABLE redeem_codes(id bigserial PRIMARY KEY,code varchar(32) UNIQUE,type text,value numeric(20,8),balance_source text,status text,used_by bigint,used_at timestamptz,notes text,created_at timestamptz,validity_days integer,expires_at timestamptz);
	INSERT INTO users VALUES(1,'operator','active',0,0,0,0,0,NULL,NOW()),(2,'user','active',5,3,2,3,2,NULL,NOW())`)
	require.NoError(t, err)
	migration, err := os.ReadFile(filepath.Join("..", "..", "migrations", "239_operator_balance_cache_version.sql"))
	require.NoError(t, err)
	for range 2 {
		_, err = db.ExecContext(ctx, string(migration))
		require.NoError(t, err)
	}
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	r := &operatorBalanceRepository{client: client}
	reader := NewUserRepository(client, db).(service.UserBalanceVersionReader)
	svc := service.NewOperatorBalanceService(r, nil, nil)
	req := service.OperatorBalanceAdjustmentRequest{Operation: "add", Amount: "0.00000001", Source: "paid", Reason: "synthetic version concurrency", IdempotencyKey: "synthetic-version-key-01"}
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := svc.Adjust(ctx, 1, 2, req); errs <- e }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	balance, version, err := reader.GetUserBalanceVersion(ctx, 2)
	require.NoError(t, err)
	require.Equal(t, 5.00000001, balance)
	require.EqualValues(t, 1, version, "concurrent same-key replay advances generation exactly once")
	var receipts int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT count(*) FROM redeem_codes").Scan(&receipts))
	require.Equal(t, 1, receipts)
	// Ledger failure after the UPDATE must roll back both amount and generation.
	_, err = db.ExecContext(ctx, `ALTER TABLE redeem_codes ADD CONSTRAINT reject_next_receipt CHECK (notes NOT LIKE '%synthetic rollback%')`)
	require.NoError(t, err)
	req.IdempotencyKey, req.Reason = "synthetic-version-key-02", "synthetic rollback"
	_, err = svc.Adjust(ctx, 1, 2, req)
	require.Error(t, err)
	newBalance, newVersion, err := reader.GetUserBalanceVersion(ctx, 2)
	require.NoError(t, err)
	require.Equal(t, balance, newBalance)
	require.Equal(t, version, newVersion)
	// A successful distinct operation advances the same transaction generation.
	req.IdempotencyKey, req.Reason = "synthetic-version-key-03", "synthetic third operation"
	_, err = svc.Adjust(ctx, 1, 2, req)
	require.NoError(t, err)
	_, newVersion, err = reader.GetUserBalanceVersion(ctx, 2)
	require.NoError(t, err)
	require.EqualValues(t, 2, newVersion)
	t.Logf("synthetic database preserved: %s", dbname)
}
