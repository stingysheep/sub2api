package repository

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"
)

// CI uses the harness-owned synthetic container and a new schema per test. Other
// integration suites' administrator fixtures cannot alter last-admin assertions.
func adminGuardPostgresSchemaRepo(t *testing.T, sourceDB *sql.DB, sourceDSN string) (*userRepository, *dbent.Client) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	schema := fmt.Sprintf("permissions_guard_%d", time.Now().UnixNano())
	_, err := sourceDB.ExecContext(ctx, "CREATE SCHEMA "+schema)
	require.NoError(t, err)
	dsn, err := url.Parse(sourceDSN)
	require.NoError(t, err)
	require.True(t, dsn.Scheme == "postgres" || dsn.Scheme == "postgresql")
	params := dsn.Query()
	params.Set("search_path", schema)
	dsn.RawQuery = params.Encode()
	db, err := sql.Open("postgres", dsn.String())
	require.NoError(t, err)
	db.SetMaxOpenConns(8)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.PingContext(ctx))
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	require.NoError(t, client.Schema.Create(ctx))
	// Schemas leave with the harness container; no independent bulk cleanup.
	return newUserRepositoryWithSQL(client, db), client
}
