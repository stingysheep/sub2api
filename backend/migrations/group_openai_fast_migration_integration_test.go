//go:build integration

package migrations_test

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/Wei-Shaw/sub2api/ent"
	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/migrations"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

const fastPolicyMigration = "238_group_openai_fast_and_reasoning_limit_policy.sql"

// Explicit opt-in targets only this task's newly initialized synthetic cluster.
// Each test creates a new database and preserves it for inspection; no preexisting
// database, snapshot or credentials are accepted by this helper.
func newMigrationDatabase(t *testing.T) (*sql.DB, context.Context) {
	t.Helper()
	if os.Getenv("SUB2API_MIGRATIONS_LOCAL_TEST") != "1" {
		t.Skip("set SUB2API_MIGRATIONS_LOCAL_TEST=1 for the synthetic localhost:25433 cluster")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	const base = "host=127.0.0.1 port=25433 user=migration_test sslmode=disable connect_timeout=5 dbname="
	admin, err := sql.Open("postgres", base+"postgres")
	require.NoError(t, err)
	defer admin.Close()
	require.NoError(t, admin.PingContext(ctx))
	name := fmt.Sprintf("migration_bug02_%d", time.Now().UnixNano())
	_, err = admin.ExecContext(ctx, "CREATE DATABASE "+name)
	require.NoError(t, err)
	db, err := sql.Open("postgres", base+name)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db, ctx
}

func TestGroupFastMigration_FullEmptyDatabaseAndEnt(t *testing.T) {
	db, ctx := newMigrationDatabase(t)
	var tables int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public'").Scan(&tables))
	require.Zero(t, tables, "the migration input must be a genuinely empty database")
	require.NoError(t, repository.ApplyMigrations(ctx, db))
	names, err := fs.Glob(migrations.FS, "*.sql")
	require.NoError(t, err)
	var applied int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations").Scan(&applied))
	require.Equal(t, len(names), applied)
	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	group, err := client.Group.Create().SetName("synthetic-ent-defaults").SetPlatform("openai").Save(ctx)
	require.NoError(t, err)
	require.False(t, group.ForceOpenaiFast)
	require.False(t, group.FreeOpenaiFast)
	require.Equal(t, "downgrade", group.MaxReasoningEffortOverLimit)
	configured, err := client.Group.Create().SetName("synthetic-ent-custom").SetPlatform("openai").SetForceOpenaiFast(true).SetFreeOpenaiFast(true).SetMaxReasoningEffortOverLimit("deny").Save(ctx)
	require.NoError(t, err)
	require.NoError(t, repository.ApplyMigrations(ctx, db), "the full runner must replay safely")
	configured, err = client.Group.Get(ctx, configured.ID)
	require.NoError(t, err)
	require.True(t, configured.ForceOpenaiFast)
	require.True(t, configured.FreeOpenaiFast)
	require.Equal(t, "deny", configured.MaxReasoningEffortOverLimit)
	requireGroupFastColumnShape(t, ctx, db)

	// Simulate the historical fully migrated database with manually added columns,
	// before 238 was recorded. Only this newly created synthetic database changes.
	_, err = db.ExecContext(ctx, "DELETE FROM schema_migrations WHERE filename = $1", fastPolicyMigration)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `ALTER TABLE groups
        ALTER COLUMN force_openai_fast DROP NOT NULL, ALTER COLUMN force_openai_fast DROP DEFAULT,
        ALTER COLUMN free_openai_fast DROP NOT NULL, ALTER COLUMN free_openai_fast DROP DEFAULT,
        ALTER COLUMN max_reasoning_effort_over_limit DROP NOT NULL, ALTER COLUMN max_reasoning_effort_over_limit DROP DEFAULT;
        INSERT INTO groups (name, platform, force_openai_fast, free_openai_fast, max_reasoning_effort_over_limit)
        VALUES ('synthetic-legacy-null', 'openai', NULL, NULL, NULL)`)
	require.NoError(t, err)
	require.NoError(t, repository.ApplyMigrations(ctx, db))
	require.NoError(t, repository.ApplyMigrations(ctx, db))
	configured, err = client.Group.Get(ctx, configured.ID)
	require.NoError(t, err)
	require.True(t, configured.ForceOpenaiFast)
	require.True(t, configured.FreeOpenaiFast)
	require.Equal(t, "deny", configured.MaxReasoningEffortOverLimit)
	requireGroupFastColumnShape(t, ctx, db)
	var force, free bool
	var policy string
	require.NoError(t, db.QueryRowContext(ctx, "SELECT force_openai_fast, free_openai_fast, max_reasoning_effort_over_limit FROM groups WHERE name = 'synthetic-legacy-null'").Scan(&force, &free, &policy))
	require.False(t, force)
	require.False(t, free)
	require.Equal(t, "downgrade", policy)
}

func TestGroupFastMigration_LegacyMissingAndPartialColumns(t *testing.T) {
	for _, columns := range []string{
		"",
		", force_openai_fast BOOLEAN",
		", free_openai_fast BOOLEAN",
		", max_reasoning_effort_over_limit VARCHAR(20)",
		", force_openai_fast BOOLEAN, free_openai_fast BOOLEAN, max_reasoning_effort_over_limit VARCHAR(20)",
	} {
		t.Run(columns, func(t *testing.T) {
			db, ctx := newMigrationDatabase(t)
			_, err := db.ExecContext(ctx, "CREATE TABLE groups (id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL"+columns+"); INSERT INTO groups (name) VALUES ('synthetic-existing-row')")
			require.NoError(t, err)
			content, err := migrations.FS.ReadFile(fastPolicyMigration)
			require.NoError(t, err)
			for i := 0; i < 2; i++ {
				_, err = db.ExecContext(ctx, string(content))
				require.NoError(t, err)
			}
			requireGroupFastColumnShape(t, ctx, db)
			_, err = db.ExecContext(ctx, "INSERT INTO groups (name) VALUES ('synthetic-default-row')")
			require.NoError(t, err)
			rows, err := db.QueryContext(ctx, "SELECT force_openai_fast, free_openai_fast, max_reasoning_effort_over_limit FROM groups")
			require.NoError(t, err)
			defer rows.Close()
			count := 0
			for rows.Next() {
				var force, free bool
				var policy string
				require.NoError(t, rows.Scan(&force, &free, &policy))
				require.False(t, force)
				require.False(t, free)
				require.Equal(t, "downgrade", policy)
				count++
			}
			require.NoError(t, rows.Err())
			require.Equal(t, 2, count)
		})
	}
}

func TestGroupFastMigration_PreservesExistingConfiguration(t *testing.T) {
	db, ctx := newMigrationDatabase(t)
	_, err := db.ExecContext(ctx, `CREATE TABLE groups (
		id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL,
		force_openai_fast BOOLEAN DEFAULT TRUE, free_openai_fast BOOLEAN DEFAULT TRUE,
		max_reasoning_effort_over_limit VARCHAR(20) DEFAULT 'deny')`)
	require.NoError(t, err)
	for i, values := range []struct {
		force, free bool
		policy      string
	}{{true, false, "deny"}, {false, true, "downgrade"}, {true, true, ""}, {false, false, "deny"}} {
		name := fmt.Sprintf("synthetic-preserved-%d", i)
		_, err = db.ExecContext(ctx, `INSERT INTO groups (name, force_openai_fast, free_openai_fast, max_reasoning_effort_over_limit)
			VALUES ($1, $2, $3, $4)`, name, values.force, values.free, values.policy)
		require.NoError(t, err)
	}
	content, err := migrations.FS.ReadFile(fastPolicyMigration)
	require.NoError(t, err)
	for i := 0; i < 2; i++ {
		_, err = db.ExecContext(ctx, string(content))
		require.NoError(t, err)
	}
	requireGroupFastColumnShape(t, ctx, db)
	var preserved int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM groups WHERE
		(name = 'synthetic-preserved-0' AND force_openai_fast = TRUE AND free_openai_fast = FALSE AND max_reasoning_effort_over_limit = 'deny') OR
		(name = 'synthetic-preserved-1' AND force_openai_fast = FALSE AND free_openai_fast = TRUE AND max_reasoning_effort_over_limit = 'downgrade') OR
		(name = 'synthetic-preserved-2' AND force_openai_fast = TRUE AND free_openai_fast = TRUE AND max_reasoning_effort_over_limit = '') OR
		(name = 'synthetic-preserved-3' AND force_openai_fast = FALSE AND free_openai_fast = FALSE AND max_reasoning_effort_over_limit = 'deny')`).Scan(&preserved))
	require.Equal(t, 4, preserved)
}

func requireGroupFastColumnShape(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	for _, column := range []string{"force_openai_fast", "free_openai_fast", "max_reasoning_effort_over_limit"} {
		var nullable, defaultValue, dataType string
		var length sql.NullInt64
		require.NoError(t, db.QueryRowContext(ctx, `SELECT is_nullable, COALESCE(column_default, ''), data_type, character_maximum_length
            FROM information_schema.columns WHERE table_schema = 'public' AND table_name = 'groups' AND column_name = $1`, column).Scan(&nullable, &defaultValue, &dataType, &length))
		require.Equal(t, "NO", nullable)
		if column == "max_reasoning_effort_over_limit" {
			require.Contains(t, defaultValue, "'downgrade'")
			require.Equal(t, "character varying", dataType)
			require.Equal(t, int64(20), length.Int64)
		} else {
			require.Equal(t, "false", defaultValue)
			require.Equal(t, "boolean", dataType)
		}
	}
}
