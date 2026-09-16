//go:build unit

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestBillingCacheAuthorityPostgres_CommitWithoutFinalizeOrGenerationChange(t *testing.T) {
	if os.Getenv("BILLING_TEST_POSTGRES_LOCAL") == "" {
		t.Skip("isolated synthetic PostgreSQL is not configured")
	}
	require.Equal(t, "127.0.0.1:25433", os.Getenv("BILLING_TEST_POSTGRES_LOCAL"))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	admin, err := sql.Open("postgres", "host=127.0.0.1 port=25433 user=migration_test dbname=postgres sslmode=disable")
	require.NoError(t, err)
	t.Cleanup(func() { _ = admin.Close() })
	name := fmt.Sprintf("billing_authority_it_%d", time.Now().UnixNano())
	_, err = admin.ExecContext(ctx, "CREATE DATABASE "+name)
	require.NoError(t, err)
	db, err := sql.Open("postgres", "host=127.0.0.1 port=25433 user=migration_test dbname="+name+" sslmode=disable")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.ExecContext(ctx, `CREATE TABLE users(id bigint PRIMARY KEY,balance numeric(20,8) NOT NULL,operator_balance_cache_version bigint NOT NULL,deleted_at timestamptz); INSERT INTO users VALUES(7,100,7,NULL)`)
	require.NoError(t, err)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	userRepo := NewUserRepository(client, db)
	reader := userRepo.(service.UserBalanceVersionReader)
	cache, _ := newMiniRedisCache(t)
	t.Cleanup(func() { _ = cache.rdb.Close() })
	matched, err := cache.EnsureUserBalanceVersion(ctx, 7, 7)
	require.NoError(t, err)
	require.True(t, matched)
	require.NoError(t, cache.SetUserBalance(ctx, 7, 100))
	newService := func() *service.BillingCacheService {
		s := service.NewBillingCacheService(cache, userRepo, nil, nil, nil, nil, &config.Config{}, nil)
		t.Cleanup(s.Stop)
		return s
	}
	first, second := newService(), newService()
	// The database commit survives; post-billing cache callbacks never execute.
	_, err = db.ExecContext(ctx, "UPDATE users SET balance=0 WHERE id=7")
	require.NoError(t, err)
	amount, generation, err := reader.GetUserBalanceVersion(ctx, 7)
	require.NoError(t, err)
	require.Zero(t, amount)
	require.EqualValues(t, 7, generation)
	for _, s := range []*service.BillingCacheService{first, second, newService()} {
		amount, err = s.GetUserBalance(ctx, 7)
		require.NoError(t, err)
		require.Zero(t, amount)
		require.ErrorIs(t, s.CheckBillingEligibility(ctx, &service.User{ID: 7}, nil, nil, nil, ""), service.ErrInsufficientBalance)
	}
	// A later/duplicate queued cache decrement must not hide real available funds.
	_, err = db.ExecContext(ctx, "UPDATE users SET balance=25 WHERE id=7")
	require.NoError(t, err)
	require.NoError(t, cache.DeductUserBalance(ctx, 7, 100))
	amount, err = second.GetUserBalance(ctx, 7)
	require.NoError(t, err)
	require.Equal(t, float64(25), amount)
	require.NoError(t, second.CheckBillingEligibility(ctx, &service.User{ID: 7}, nil, nil, nil, ""))
	t.Logf("synthetic database preserved: %s", name)
}
