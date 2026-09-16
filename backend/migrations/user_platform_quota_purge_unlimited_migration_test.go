package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestUserPlatformQuotasPurgeUnlimitedMigration 校验 238 号迁移只删除三档限额全为 NULL 的活跃行：
// 这类活跃行等价于"不存在"；任何一档非 NULL 的记录和所有软删历史都必须保留。
func TestUserPlatformQuotasPurgeUnlimitedMigration(t *testing.T) {
	content, err := FS.ReadFile("238_purge_unlimited_user_platform_quotas.sql")
	require.NoError(t, err)

	// 去掉注释行后只允许这一条 DELETE。
	var stmts []string
	for _, line := range strings.Split(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}
		stmts = append(stmts, trimmed)
	}
	sql := strings.Join(stmts, " ")
	require.Equal(t, "DELETE FROM user_platform_quotas WHERE daily_limit_usd IS NULL AND weekly_limit_usd IS NULL AND monthly_limit_usd IS NULL AND deleted_at IS NULL;", sql)
	require.Contains(t, sql, "deleted_at IS NULL", "migration must preserve soft-deleted quota history")
}
