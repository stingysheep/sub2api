package repository

import (
	"context"
	"database/sql"
	"errors"
	"github.com/shopspring/decimal"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type usageBillingRecoveryState interface {
	CommittedUsageBilling(context.Context, *service.UsageBillingCommand) (*service.UsageBillingApplyResult, error)
	FenceRecoveredBalance(context.Context, int64) error
	VerifyCommittedUsage(context.Context, *service.UsageBillingCommand, *service.UsageLog) error
}

var errUsageRecoveryConflict = errors.New("existing usage row conflicts with recovered billing")

func (r *usageBillingRepository) VerifyCommittedUsage(ctx context.Context, cmd *service.UsageBillingCommand, log *service.UsageLog) error {
	var userID, accountID int64
	var actual, total, free, paid, unfunded string
	err := r.db.QueryRowContext(ctx, `SELECT ul.user_id,ul.account_id,ul.actual_cost::text,ul.total_cost::text,COALESCE(uba.free_cost,0)::text,COALESCE(uba.paid_cost,0)::text,COALESCE(uba.unfunded_cost,0)::text FROM usage_logs ul LEFT JOIN usage_balance_allocations uba ON uba.request_id=ul.request_id AND uba.api_key_id=ul.api_key_id AND uba.user_id=ul.user_id WHERE ul.request_id=$1 AND ul.api_key_id=$2`, cmd.RequestID, cmd.APIKeyID).Scan(&userID, &accountID, &actual, &total, &free, &paid, &unfunded)
	if err != nil {
		return err
	}
	if userID != log.UserID || accountID != log.AccountID {
		return errUsageRecoveryConflict
	}
	for _, pair := range []struct {
		stored   string
		expected float64
	}{{actual, log.ActualCost}, {total, log.TotalCost}, {free, log.FreeBalanceCost}, {paid, log.PaidBalanceCost}, {unfunded, log.UnfundedBalanceCost}} {
		stored, err := decimal.NewFromString(pair.stored)
		if err != nil || !stored.Equal(decimal.NewFromFloat(pair.expected).Round(service.UsageBillingMonetaryScale)) {
			return errUsageRecoveryConflict
		}
	}
	return nil
}

func (r *usageBillingRepository) CommittedUsageBilling(ctx context.Context, cmd *service.UsageBillingCommand) (*service.UsageBillingApplyResult, error) {
	var fingerprint string
	err := r.db.QueryRowContext(ctx, `SELECT request_fingerprint FROM usage_billing_dedup WHERE request_id=$1 AND api_key_id=$2 UNION ALL SELECT request_fingerprint FROM usage_billing_dedup_archive WHERE request_id=$1 AND api_key_id=$2 LIMIT 1`, cmd.RequestID, cmd.APIKeyID).Scan(&fingerprint)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(fingerprint) != strings.TrimSpace(cmd.RequestFingerprint) {
		return nil, service.ErrUsageBillingRequestConflict
	}
	result := &service.UsageBillingApplyResult{}
	if cmd.BalanceCost > 0 {
		err = r.db.QueryRowContext(ctx, `SELECT free_cost,paid_cost,unfunded_cost FROM usage_balance_allocations WHERE request_id=$1 AND api_key_id=$2 AND user_id=$3`, cmd.RequestID, cmd.APIKeyID, cmd.UserID).Scan(&result.FreeBalanceCost, &result.PaidBalanceCost, &result.UnfundedBalanceCost)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (r *usageBillingRepository) FenceRecoveredBalance(ctx context.Context, userID int64) error {
	result, err := r.db.ExecContext(ctx, `UPDATE users SET operator_balance_cache_version=operator_balance_cache_version+1 WHERE id=$1 AND deleted_at IS NULL`, userID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return service.ErrUserNotFound
	}
	return nil
}
