//go:build !integration

package repository

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"
)

// Fixed, opt-in, empty synthetic database; cannot be redirected to production.
func adminGuardPostgresRepo(t *testing.T) (*userRepository, *dbent.Client) {
	t.Helper()
	if os.Getenv("SUB2API_PERMISSIONS_POSTGRES_TEST") != "1" {
		t.Skip("set SUB2API_PERMISSIONS_POSTGRES_TEST=1 for the isolated loopback synthetic database")
	}
	db, err := sql.Open("postgres", "host=127.0.0.1 port=25433 user=migration_test dbname=permissions_autonomous_20260916 sslmode=disable")
	require.NoError(t, err)
	db.SetMaxOpenConns(8)
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	require.NoError(t, db.PingContext(ctx))
	return adminGuardPostgresSchemaRepo(t, db, "postgres://migration_test@127.0.0.1:25433/permissions_autonomous_20260916?sslmode=disable")
}
