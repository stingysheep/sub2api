package service

import (
	"context"
	"errors"
)

// RedeemCodeStats excludes internal adjustment ledgers: those are not redeemable codes.
type RedeemCodeStats struct {
	TotalCodes            int64            `json:"total_codes"`
	ActiveCodes           int64            `json:"active_codes"`
	UsedCodes             int64            `json:"used_codes"`
	ExpiredCodes          int64            `json:"expired_codes"`
	TotalValueDistributed float64          `json:"total_value_distributed"`
	ByType                map[string]int64 `json:"by_type"`
}

type redeemStatsReader interface {
	RedeemStats(context.Context) (*RedeemCodeStats, error)
}

func (s *RedeemService) readStats(ctx context.Context) (*RedeemCodeStats, error) {
	if s == nil {
		return nil, errors.New("redeem statistics service unavailable")
	}
	reader, ok := s.redeemRepo.(redeemStatsReader)
	if !ok {
		return nil, errors.New("redeem statistics repository unavailable")
	}
	stats, err := reader.RedeemStats(ctx)
	if err != nil {
		return nil, err
	}
	if stats == nil {
		return nil, errors.New("redeem statistics repository returned no result")
	}
	return stats, nil
}
