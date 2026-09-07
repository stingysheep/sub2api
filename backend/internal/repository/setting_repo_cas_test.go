package repository

import (
	"context"
	"database/sql"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestSettingCompareAndSwap(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`CREATE TABLE settings (id INTEGER PRIMARY KEY AUTOINCREMENT, key TEXT NOT NULL UNIQUE, value TEXT NOT NULL, updated_at DATETIME NOT NULL)`)
	require.NoError(t, err)
	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.SQLite, db)))
	repo := &settingRepository{client: client}
	ctx := context.Background()
	ok, err := repo.CompareAndSwap(ctx, "profiles", nil, "original")
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = repo.CompareAndSwap(ctx, "profiles", nil, "overwrite")
	require.NoError(t, err)
	require.False(t, ok)
	previous := "original"
	ok, err = repo.CompareAndSwap(ctx, "profiles", &previous, "new")
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = repo.CompareAndSwap(ctx, "profiles", &previous, "stale")
	require.NoError(t, err)
	require.False(t, ok)
	got, err := repo.GetValue(ctx, "profiles")
	require.NoError(t, err)
	require.Equal(t, "new", got)
}
