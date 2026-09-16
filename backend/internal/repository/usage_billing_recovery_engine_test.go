package repository

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// Recovery workers may open the same WAL from separate processes. The engine
// must include SQLite's WAL-reset corruption fix, not merely limit connections
// on one database/sql handle.
func TestRecoverySQLiteEngineIncludesWALResetFix(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "engine.sqlite"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	var version string
	require.NoError(t, db.QueryRow("SELECT sqlite_version()").Scan(&version))
	var major, minor, patch int
	n, err := fmt.Sscanf(version, "%d.%d.%d", &major, &minor, &patch)
	require.NoError(t, err)
	require.Equal(t, 3, n)
	code := major*1000000 + minor*1000 + patch
	patched := code >= 3051003 || (major == 3 && minor == 50 && patch >= 7) || (major == 3 && minor == 44 && patch >= 6)
	require.True(t, patched, "SQLite %s lacks the multi-connection WAL-reset corruption fix", version)
}
