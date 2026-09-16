//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type durableBillingGatewayStub struct {
	UsageBillingRepository
	calls   int
	err     error
	lastLog *UsageLog
}

func (r *durableBillingGatewayStub) ApplyWithUsage(ctx context.Context, cmd *UsageBillingCommand, log *UsageLog) (*UsageBillingApplyResult, error) {
	r.calls++
	r.lastLog = log
	if r.err != nil {
		return nil, r.err
	}
	// This stub models the confirmed usage persistence done by the durable repo.
	return &UsageBillingApplyResult{Applied: false}, nil
}

func TestGatewayUsageBillingRecovery_DurableFailureNeverWritesZeroCostPlaceholder(t *testing.T) {
	for _, gateway := range []string{"anthropic", "openai"} {
		for _, failed := range []bool{false, true} {
			t.Run(gateway+map[bool]string{true: "_failed", false: "_persisted"}[failed], func(t *testing.T) {
				usage := &openAIRecordUsageLogRepoStub{}
				durable := &durableBillingGatewayStub{}
				if failed {
					durable.err = ErrUsageBillingRecoveryRequired
				}
				var err error
				if gateway == "anthropic" {
					svc := newGatewayRecordUsageServiceWithBillingRepoForTest(usage, durable, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})
					err = svc.RecordUsage(context.Background(), &RecordUsageInput{Result: &ForwardResult{RequestID: "synthetic-durable", Model: "claude-sonnet-4", Usage: ClaudeUsage{InputTokens: 10, OutputTokens: 6}, Duration: time.Second}, APIKey: &APIKey{ID: 3}, User: &User{ID: 2}, Account: &Account{ID: 4}})
				} else {
					svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(usage, durable, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
					err = svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{Result: &OpenAIForwardResult{RequestID: "synthetic-durable", Model: "gpt-5.4", Usage: OpenAIUsage{InputTokens: 10, OutputTokens: 6}, Duration: time.Second}, APIKey: &APIKey{ID: 3}, User: &User{ID: 2}, Account: &Account{ID: 4}})
				}
				if failed {
					require.ErrorIs(t, err, ErrUsageBillingRecoveryRequired)
				} else {
					require.NoError(t, err)
				}
				require.Equal(t, 1, durable.calls)
				require.Positive(t, durable.lastLog.ActualCost)
				require.Zero(t, usage.calls, "durable intent/ACK must not be overwritten by a zero-cost or duplicate usage writer")
			})
		}
	}
}
