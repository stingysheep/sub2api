package repository

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAffiliateUserOverviewSQLIncludesMaturedFrozenQuota(t *testing.T) {
	query := strings.Join(strings.Fields(affiliateUserOverviewSQL), " ")

	require.Contains(t, query, "ua.aff_quota + COALESCE(matured.matured_frozen_quota, 0)")
	require.Contains(t, query, "frozen_until <= NOW()")
}

func TestAffiliateRecordQueriesUseLedgerAuditFields(t *testing.T) {
	source, err := os.ReadFile("affiliate_repo.go")
	require.NoError(t, err)
	content := string(source)

	require.Contains(t, content, "LEFT JOIN payment_orders po ON po.id = ual.source_order_id")
	require.Contains(t, content, "ual.source_order_id IS NOT NULL OR ual.source_usage_request_id IS NOT NULL")
	require.Contains(t, content, "ual.source_usage_request_id")
	require.Contains(t, content, "CASE WHEN ual.source_usage_request_id IS NOT NULL THEN 'usage'")
	require.Contains(t, content, "COALESCE(po.amount, 0)::double precision")
	require.Contains(t, content, "ual.amount::double precision")
	require.Contains(t, content, "ual.balance_after::double precision")
	require.NotContains(t, content, "parseAffiliateRebateAmount")
	require.NotContains(t, content, `"current_balance": "u.balance"`)
}

func TestAccrueUsageQuotaUsesPartialUniqueIndexConflictTarget(t *testing.T) {
	source, err := os.ReadFile("affiliate_repo.go")
	require.NoError(t, err)
	content := string(source)

	require.Contains(t, content, "ON CONFLICT (source_usage_request_id) WHERE source_usage_request_id IS NOT NULL DO NOTHING")
	require.NotContains(t, content, "ON CONFLICT (source_usage_request_id) DO NOTHING")
}
func TestAdminAffiliateAnalyticsQueryKeepsGrowthAndRankingScopes(t *testing.T) {
	source, err := os.ReadFile("affiliate_repo.go")
	require.NoError(t, err)
	content := string(source)

	require.Contains(t, content, "func (r *affiliateRepository) GetAdminAnalytics")
	require.Contains(t, content, "invitee.role = 'user'")
	require.Contains(t, content, "invitee.deleted_at IS NULL")
	require.Contains(t, content, "u.created_at AT TIME ZONE $3")
	require.Contains(t, content, "ua.inviter_id IS NULL")
	require.Contains(t, content, "ua.inviter_id IS NOT NULL")
	require.Contains(t, content, "LIMIT $5")
}
