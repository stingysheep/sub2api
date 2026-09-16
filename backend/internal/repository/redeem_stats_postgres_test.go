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
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestRedeemStatsPostgresExpirationAndInternalLedgerExclusion(t *testing.T) {
	if os.Getenv("SUB2API_BACKLOG_LOCAL_TEST") != "1" {
		t.Skip("requires this task's synthetic localhost:25433 cluster")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	const dsn = "host=127.0.0.1 port=25433 user=migration_test dbname=postgres sslmode=disable connect_timeout=5"
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	schema := fmt.Sprintf("redeem_stats_%d", time.Now().UnixNano())
	_, err = db.ExecContext(ctx, "CREATE SCHEMA "+schema)
	require.NoError(t, err)
	testdb, err := sql.Open("postgres", dsn+" search_path="+schema)
	require.NoError(t, err)
	defer testdb.Close()
	_, err = testdb.ExecContext(ctx, `CREATE TABLE redeem_codes (id bigserial PRIMARY KEY, type text, status text, expires_at timestamptz, value numeric(20,8));
INSERT INTO redeem_codes(type,status,expires_at,value) VALUES
('balance','unused',NULL,100),('balance','unused',NOW()+INTERVAL '1 day',100),
('balance','unused',NOW()-INTERVAL '1 day',100),('balance','expired',NULL,100),
('balance','used',NOW()-INTERVAL '1 day',12.34567891),('balance','used',NULL,-3),
('subscription','used',NULL,999),('invitation','unused',NULL,0),
('operator_balance','used',NULL,500),('admin_balance','used',NULL,500);`)
	require.NoError(t, err)
	r := &redeemCodeRepository{client: dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, testdb)))}
	stats, err := r.RedeemStats(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 8, stats.TotalCodes)
	require.EqualValues(t, 3, stats.ActiveCodes)
	require.EqualValues(t, 3, stats.UsedCodes)
	require.EqualValues(t, 2, stats.ExpiredCodes)
	require.Equal(t, 12.34567891, stats.TotalValueDistributed)
	require.EqualValues(t, 6, stats.ByType["balance"])
	require.NotContains(t, stats.ByType, "operator_balance")
}
