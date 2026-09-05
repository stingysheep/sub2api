//go:build unit

package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestResolveEndpointColumn(t *testing.T) {
	tests := []struct {
		endpointType string
		want         string
	}{
		{"inbound", "ul.inbound_endpoint"},
		{"upstream", "ul.upstream_endpoint"},
		{"path", "ul.inbound_endpoint || ' -> ' || ul.upstream_endpoint"},
		{"", "ul.inbound_endpoint"},        // default
		{"unknown", "ul.inbound_endpoint"}, // fallback
	}

	for _, tc := range tests {
		t.Run(tc.endpointType, func(t *testing.T) {
			got := resolveEndpointColumn(tc.endpointType)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestResolveModelDimensionExpression(t *testing.T) {
	tests := []struct {
		modelType string
		want      string
	}{
		{usagestats.ModelSourceRequested, "COALESCE(NULLIF(TRIM(requested_model), ''), model)"},
		{usagestats.ModelSourceUpstream, "COALESCE(NULLIF(TRIM(upstream_model), ''), model)"},
		{usagestats.ModelSourceMapping, "(COALESCE(NULLIF(TRIM(requested_model), ''), model) || ' -> ' || COALESCE(NULLIF(TRIM(upstream_model), ''), model))"},
		{"", "COALESCE(NULLIF(TRIM(requested_model), ''), model)"},
		{"invalid", "COALESCE(NULLIF(TRIM(requested_model), ''), model)"},
	}

	for _, tc := range tests {
		t.Run(tc.modelType, func(t *testing.T) {
			got := resolveModelDimensionExpression(tc.modelType)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestGetUserBreakdownStatsRequestTypeIncludesLegacyFallback(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	requestType := int16(service.RequestTypeStream)

	legacyFilter := `(ul.request_type = $3 OR (ul.request_type = 0 AND ul.stream = TRUE AND ul.openai_ws_mode = FALSE))`
	mock.ExpectQuery(regexp.QuoteMeta(legacyFilter)).
		WithArgs(start, end, requestType).
		WillReturnRows(sqlmock.NewRows([]string{
			"user_id", "email", "requests", "input_tokens", "output_tokens",
			"cache_tokens", "total_tokens", "cost", "actual_cost", "account_cost",
		}))

	rows, err := repo.GetUserBreakdownStats(context.Background(), start, end, usagestats.UserBreakdownDimension{
		RequestType: &requestType,
	}, 0)

	require.NoError(t, err)
	require.Empty(t, rows)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUserBreakdownStatsFiltersNativeCompactionV2(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	nativeCompactionV2 := true

	mock.ExpectQuery(regexp.QuoteMeta("AND ul.native_compaction_v2 = $3")).
		WithArgs(start, end, true).
		WillReturnRows(sqlmock.NewRows([]string{
			"user_id", "email", "requests", "input_tokens", "output_tokens",
			"cache_tokens", "total_tokens", "cost", "actual_cost", "account_cost",
		}))

	rows, err := repo.GetUserBreakdownStats(context.Background(), start, end, usagestats.UserBreakdownDimension{
		NativeCompactionV2: &nativeCompactionV2,
	}, 0)

	require.NoError(t, err)
	require.Empty(t, rows)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFillDashboardUsageStatsUsesCurrentAccountRate(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	today := start
	now := start.Add(12 * time.Hour)

	costExpression := regexp.QuoteMeta("COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(a.rate_multiplier, 1) AS account_cost")
	accountJoin := regexp.QuoteMeta("LEFT JOIN accounts a ON a.id = ul.account_id")
	mock.ExpectQuery("(?s)"+costExpression+".*"+accountJoin).
		WithArgs(start, end, today, today.Add(24*time.Hour)).
		WillReturnRows(sqlmock.NewRows([]string{
			"total_requests", "total_input_tokens", "total_output_tokens",
			"total_cache_creation_tokens", "total_cache_read_tokens",
			"total_cost", "total_actual_cost", "total_account_cost", "total_duration_ms",
			"today_requests", "today_input_tokens", "today_output_tokens",
			"today_cache_creation_tokens", "today_cache_read_tokens",
			"today_cost", "today_actual_cost", "today_account_cost",
		}).AddRow(1, 10, 20, 0, 0, 1.0, 0.5, 0.25, 100, 1, 10, 20, 0, 0, 1.0, 0.5, 0.25))

	hourStart := now.UTC().Truncate(time.Hour)
	mock.ExpectQuery(`(?s)COUNT\(DISTINCT CASE.*FROM scoped`).
		WithArgs(today, today.Add(24*time.Hour), hourStart, hourStart.Add(time.Hour)).
		WillReturnRows(sqlmock.NewRows([]string{"active_users", "hourly_active_users"}).AddRow(1, 1))

	stats := &usagestats.DashboardStats{}
	require.NoError(t, repo.fillDashboardUsageStatsFromUsageLogs(context.Background(), stats, start, end, today, now, ""))
	require.InDelta(t, 0.25, stats.TotalAccountCost, 1e-9)
	require.InDelta(t, 0.25, stats.TodayAccountCost, 1e-9)
	require.NoError(t, mock.ExpectationsWereMet())
}
