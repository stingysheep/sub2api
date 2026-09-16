//go:build unit

package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type recoveryPGAckLoss struct {
	*usageBillingRepository
	failBefore, loseAck bool
}

func (r *recoveryPGAckLoss) Apply(ctx context.Context, cmd *service.UsageBillingCommand) (*service.UsageBillingApplyResult, error) {
	if r.failBefore {
		r.failBefore = false
		return nil, errors.New("synthetic Apply failed before transaction")
	}
	result, err := r.usageBillingRepository.Apply(ctx, cmd)
	if err == nil && r.loseAck {
		r.loseAck = false
		return nil, errors.New("synthetic PG commit ACK lost after real commit")
	}
	return result, err
}

type recoveryPGUsage struct {
	service.UsageLogRepository
	db            *sql.DB
	fail, loseAck bool
}

func (r *recoveryPGUsage) Create(ctx context.Context, log *service.UsageLog) (bool, error) {
	if r.fail {
		r.fail = false
		return false, errors.New("synthetic usage write failed")
	}
	result, err := r.db.ExecContext(ctx, `INSERT INTO usage_logs(request_id,api_key_id,actual_cost,free_cost,paid_cost,unfunded_cost,user_id,account_id,total_cost) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(request_id,api_key_id) DO NOTHING`, log.RequestID, log.APIKeyID, log.ActualCost, log.FreeBalanceCost, log.PaidBalanceCost, log.UnfundedBalanceCost, log.UserID, log.AccountID, log.TotalCost)
	if err != nil {
		return false, err
	}
	if r.loseAck {
		r.loseAck = false
		return false, errors.New("synthetic usage ACK lost after real commit")
	}
	affected, err := result.RowsAffected()
	return affected == 1, err
}

type recoveryFailedRedisGuard struct{ *billingCache }

func (r *recoveryFailedRedisGuard) EnsureUserBalanceVersion(context.Context, int64, int64) (bool, error) {
	return false, errors.New("synthetic Redis unavailable")
}

func TestUsageBillingRecoveryPostgresRealMoneyAllocationsAndRedisFence(t *testing.T) {
	if os.Getenv("BILLING_TEST_POSTGRES_LOCAL") == "" {
		t.Skip("isolated local PostgreSQL is not configured")
	}
	require.Equal(t, "127.0.0.1:25433", os.Getenv("BILLING_TEST_POSTGRES_LOCAL"))
	for _, failure := range []string{"apply_failed", "commit_ack_lost", "usage_failed", "usage_ack_lost", "historical_zero_cost"} {
		t.Run(failure, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			admin, err := sql.Open("postgres", "host=127.0.0.1 port=25433 user=migration_test dbname=postgres sslmode=disable")
			require.NoError(t, err)
			defer admin.Close()
			dbname := fmt.Sprintf("billing_recovery_it_%d", time.Now().UnixNano())
			_, err = admin.ExecContext(ctx, "CREATE DATABASE "+dbname)
			require.NoError(t, err)
			db, err := sql.Open("postgres", "host=127.0.0.1 port=25433 user=migration_test dbname="+dbname+" sslmode=disable")
			require.NoError(t, err)
			defer db.Close()
			_, err = db.ExecContext(ctx, `CREATE TABLE users(id bigint PRIMARY KEY,balance numeric(20,8),free_balance numeric(20,8),paid_balance numeric(20,8),deleted_at timestamptz,updated_at timestamptz,operator_balance_cache_version bigint DEFAULT 0 NOT NULL);
			CREATE TABLE api_keys(id bigint PRIMARY KEY,quota numeric(20,8),quota_used numeric(20,8),status text,updated_at timestamptz,deleted_at timestamptz,expires_at timestamptz,user_id bigint NOT NULL);
			CREATE TABLE usage_billing_dedup(id bigserial PRIMARY KEY,request_id text,api_key_id bigint,request_fingerprint text,UNIQUE(request_id,api_key_id));
			CREATE TABLE usage_billing_dedup_archive(request_id text,api_key_id bigint,request_fingerprint text,UNIQUE(request_id,api_key_id));
			CREATE TABLE usage_balance_allocations(request_id text,api_key_id bigint,user_id bigint,free_cost numeric(20,8),paid_cost numeric(20,8),unfunded_cost numeric(20,8),UNIQUE(request_id,api_key_id));
			CREATE TABLE usage_logs(request_id text,api_key_id bigint,actual_cost numeric(20,8),free_cost numeric(20,8),paid_cost numeric(20,8),unfunded_cost numeric(20,8),user_id bigint,account_id bigint,total_cost numeric(20,8),UNIQUE(request_id,api_key_id));
			INSERT INTO users VALUES(2,5,0.25,4.75,NULL,NOW(),0);INSERT INTO api_keys VALUES(3,1,0,'active',NOW(),NULL,NULL,2)`)
			require.NoError(t, err)
			base := &recoveryPGAckLoss{usageBillingRepository: &usageBillingRepository{db: db}}
			usage := &recoveryPGUsage{db: db}
			switch failure {
			case "apply_failed":
				base.failBefore = true
			case "commit_ack_lost":
				base.loseAck = true
			case "usage_failed":
				usage.fail = true
			case "usage_ack_lost":
				usage.loseAck = true
			case "historical_zero_cost":
				_, err = db.ExecContext(ctx, `INSERT INTO usage_logs(request_id,api_key_id,user_id,account_id,actual_cost,total_cost,free_cost,paid_cost,unfunded_cost) VALUES('synthetic-pg-historical_zero_cost',3,2,4,0,0,0,0,0)`)
				require.NoError(t, err)
			}
			path := filepath.Join(recoveryFixtureDir(t), "recovery.sqlite")
			journal, err := openUsageBillingRecovery(base, usage, path, 10, 1024*1024)
			require.NoError(t, err)
			cache, _ := newMiniRedisCache(t)
			defer cache.rdb.Close()
			_, err = cache.EnsureUserBalanceVersion(ctx, 2, 0)
			require.NoError(t, err)
			require.NoError(t, cache.SetUserBalance(ctx, 2, 5))
			cmd, log := recoveryCommand("synthetic-pg-" + failure)
			_, err = journal.ApplyWithUsage(ctx, cmd, log)
			require.ErrorIs(t, err, service.ErrUsageBillingRecoveryRequired)
			journal.Close()
			journal, err = openUsageBillingRecovery(base, usage, path, 10, 1024*1024)
			require.NoError(t, err)
			defer journal.Close()
			recoverNow(t, journal)
			var balance, free, paid, quota string
			var version, count int
			require.NoError(t, db.QueryRowContext(ctx, "SELECT balance::text,free_balance::text,paid_balance::text,operator_balance_cache_version FROM users WHERE id=2").Scan(&balance, &free, &paid, &version))
			require.Equal(t, "4.00000000", balance)
			require.Equal(t, "0.00000000", free)
			require.Equal(t, "4.00000000", paid)
			require.Equal(t, 1, version)
			require.NoError(t, db.QueryRowContext(ctx, "SELECT quota_used::text FROM api_keys WHERE id=3").Scan(&quota))
			require.Equal(t, "1.00000000", quota)
			financial := NewAPIKeyRepository(dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db))), db).(service.APIKeyFinancialStateReader)
			state, stateErr := financial.GetAPIKeyFinancialState(ctx, 3, 2)
			require.NoError(t, stateErr)
			require.Equal(t, 4.0, state.Balance)
			require.Equal(t, 1.0, state.QuotaUsed)
			require.Equal(t, service.StatusAPIKeyQuotaExhausted, state.Status)
			_, stateErr = financial.GetAPIKeyFinancialState(ctx, 3, 999)
			require.ErrorIs(t, stateErr, service.ErrAPIKeyNotFound)

			require.NoError(t, db.QueryRowContext(ctx, "SELECT count(*) FROM usage_billing_dedup").Scan(&count))
			require.Equal(t, 1, count)
			require.NoError(t, db.QueryRowContext(ctx, "SELECT count(*) FROM usage_logs").Scan(&count))
			require.Equal(t, 1, count)
			var cost, freeCost, paidCost string
			require.NoError(t, db.QueryRowContext(ctx, "SELECT actual_cost::text,free_cost::text,paid_cost::text FROM usage_logs").Scan(&cost, &freeCost, &paidCost))
			if failure == "historical_zero_cost" {
				require.Equal(t, "0.00000000", cost)
				require.Equal(t, 1, recoveryCount(t, journal, "quarantine"))
			} else {
				require.Equal(t, "1.00000000", cost)
				require.Equal(t, "0.25000000", freeCost)
				require.Equal(t, "0.75000000", paidCost)
			}
			reader := NewUserRepository(dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db))), db)
			failedRedis := service.NewBillingCacheService(&recoveryFailedRedisGuard{cache}, reader, nil, nil, nil, nil, &config.Config{}, nil)
			defer failedRedis.Stop()
			actual, err := failedRedis.GetUserBalance(ctx, 2)
			require.NoError(t, err)
			require.Equal(t, 4.0, actual, "Redis failure must use authoritative post-recovery balance")
			cached, err := cache.GetUserBalance(ctx, 2)
			require.NoError(t, err)
			require.Equal(t, 5.0, cached, "no recovery Redis invalidation was delivered")
			healthy := service.NewBillingCacheService(cache, reader, nil, nil, nil, nil, &config.Config{}, nil)
			defer healthy.Stop()
			actual, err = healthy.GetUserBalance(ctx, 2)
			require.NoError(t, err)
			require.Equal(t, 4.0, actual, "next instance observes recovery generation and rejects old cache")
			require.Zero(t, recoveryCount(t, journal, "pending"))
			t.Logf("synthetic DB preserved: %s", dbname)
		})
	}
}
