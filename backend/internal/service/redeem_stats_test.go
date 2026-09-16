package service

import (
	"context"
	"errors"
	"github.com/stretchr/testify/require"
	"testing"
)

type statsRepoStub struct {
	RedeemCodeRepository
	err error
}

func (s statsRepoStub) RedeemStats(context.Context) (*RedeemCodeStats, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &RedeemCodeStats{TotalCodes: 4, ActiveCodes: 1, UsedCodes: 2, ExpiredCodes: 1, TotalValueDistributed: 10, ByType: map[string]int64{"balance": 4}}, nil
}
func TestRedeemStatsServiceUsesRealRepositoryAndPropagatesErrors(t *testing.T) {
	s := &RedeemService{redeemRepo: statsRepoStub{}}
	stats, err := s.GetStats(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 4, stats["total_codes"])
	require.EqualValues(t, 1, stats["active_codes"])
	require.Equal(t, stats["total_value"], stats["total_value_distributed"])
	s.redeemRepo = statsRepoStub{err: errors.New("offline")}
	_, err = s.GetStats(context.Background())
	require.Error(t, err)
	s.redeemRepo = nil
	_, err = s.GetStats(context.Background())
	require.Error(t, err)
}
