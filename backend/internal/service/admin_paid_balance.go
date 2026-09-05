package service

import (
	"context"
	"fmt"
)

// GetPaidBalanceSummary returns the current paid-balance report without changing
// the existing AdminService interface used by unrelated handlers and tests.
func (s *adminServiceImpl) GetPaidBalanceSummary(ctx context.Context, limit int) (*PaidBalanceSummary, error) {
	reader, ok := s.userRepo.(PaidBalanceSummaryReader)
	if !ok {
		return nil, fmt.Errorf("paid balance summary is not available")
	}
	return reader.GetPaidBalanceSummary(ctx, limit)
}
