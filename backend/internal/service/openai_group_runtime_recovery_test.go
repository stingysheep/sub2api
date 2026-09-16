//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestSelectAccountWithSchedulerRecoversWhenAllAccountsAreModelCooldownBlocked(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	t.Cleanup(resetOpenAIAdvancedSchedulerSettingCacheForTest)
	accounts := []Account{
		{ID: 6101, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Credentials: map[string]any{"api_key": "local-test", "model_mapping": map[string]any{"gpt-6-astra": "gpt-6-astra"}}, Extra: map[string]any{"privacy_mode": PrivacyModeTrainingOff}},
		{ID: 6102, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Credentials: map[string]any{"api_key": "local-test", "model_mapping": map[string]any{"gpt-6-astra": "gpt-6-astra"}}, Extra: map[string]any{"privacy_mode": PrivacyModeTrainingOff}},
	}
	cfg := &config.Config{}
	cfg.Gateway.Scheduling.LoadBatchEnabled = false
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: accounts},
		cache:              &schedulerTestGatewayCache{},
		cfg:                cfg,
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
	}
	for _, account := range accounts {
		for range 3 {
			svc.recordOpenAIAccountModelTransientFailure(&account, "gpt-6-astra", time.Now())
		}
	}
	selection, _, err := svc.SelectAccountWithScheduler(context.Background(), nil, "", "", "gpt-6-astra", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.NotNil(t, selection)
	selection.ReleaseFunc()
}

func TestSelectAccountWithLoadBatchRecoversWhenAllAccountsAreModelCooldownBlocked(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	t.Cleanup(resetOpenAIAdvancedSchedulerSettingCacheForTest)
	accounts := []Account{
		{ID: 6201, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Credentials: map[string]any{"api_key": "local-test", "model_mapping": map[string]any{"gpt-6-astra": "gpt-6-astra"}}, Extra: map[string]any{"privacy_mode": PrivacyModeTrainingOff}},
		{ID: 6202, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Credentials: map[string]any{"api_key": "local-test", "model_mapping": map[string]any{"gpt-6-astra": "gpt-6-astra"}}, Extra: map[string]any{"privacy_mode": PrivacyModeTrainingOff}},
	}
	cfg := &config.Config{}
	cfg.Gateway.Scheduling.LoadBatchEnabled = true
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: accounts},
		cache:              &schedulerTestGatewayCache{},
		cfg:                cfg,
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
	}
	for _, account := range accounts {
		for range 3 {
			svc.recordOpenAIAccountModelTransientFailure(&account, "gpt-6-astra", time.Now())
		}
	}

	selection, err := svc.SelectAccountWithLoadAwareness(context.Background(), nil, "", "gpt-6-astra", nil)
	require.NoError(t, err)
	require.NotNil(t, selection)
	selection.ReleaseFunc()
}
