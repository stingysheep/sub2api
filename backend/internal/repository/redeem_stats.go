package repository

import (
	"context"
	"fmt"

	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/redeemcode"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *redeemCodeRepository) RedeemStats(ctx context.Context) (*service.RedeemCodeStats, error) {
	// One grouped query gives all counters a consistent snapshot. SQL NOW() is
	// stable throughout the statement, including the expires_at boundary.
	var rows []struct {
		Type        string  `json:"type"`
		Total       int64   `json:"total"`
		Active      int64   `json:"active"`
		Used        int64   `json:"used"`
		Expired     int64   `json:"expired"`
		Distributed float64 `json:"distributed"`
	}
	count := func(condition, alias string) dbent.AggregateFunc {
		return func(s *entsql.Selector) string {
			return fmt.Sprintf("COUNT(*) FILTER (WHERE %s) AS %s", condition, alias)
		}
	}
	err := clientFromContext(ctx, r.client).RedeemCode.Query().
		Where(redeemcode.TypeIn("balance", "concurrency", "subscription", "invitation")).
		GroupBy(redeemcode.FieldType).
		Aggregate(dbent.As(dbent.Count(), "total"),
			count("status = 'unused' AND (expires_at IS NULL OR expires_at > NOW())", "active"),
			count("status = 'used'", "used"),
			count("status = 'expired' OR (status = 'unused' AND expires_at <= NOW())", "expired"),
			func(s *entsql.Selector) string {
				return "COALESCE(SUM(CASE WHEN status = 'used' AND type = 'balance' AND value > 0 THEN value ELSE 0 END), 0) AS distributed"
			}).Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}
	stats := &service.RedeemCodeStats{ByType: map[string]int64{"balance": 0, "concurrency": 0, "subscription": 0, "invitation": 0}}
	for _, row := range rows {
		stats.TotalCodes += row.Total
		stats.ActiveCodes += row.Active
		stats.UsedCodes += row.Used
		stats.ExpiredCodes += row.Expired
		stats.TotalValueDistributed += row.Distributed
		stats.ByType[row.Type] = row.Total
	}
	return stats, nil
}
