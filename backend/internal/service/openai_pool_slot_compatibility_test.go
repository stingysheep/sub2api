package service

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// Verify pool policy against the final admission gate, not only the transient map.
func TestPool502504SlotCompatibility(t *testing.T) {
	for _, status := range []int{http.StatusBadGateway, http.StatusGatewayTimeout} {
		for _, configured := range []bool{false, true} {
			t.Run(fmt.Sprintf("status_%d_configured_%t", status, configured), func(t *testing.T) {
				svc, repo := newSlotAdmissionFixture(t, nil)
				svc.rateLimitService = NewRateLimitService(transientCooldownAccountRepo{}, nil, &config.Config{}, nil, nil)
				account := &repo.accounts[0]
				account.Credentials = map[string]any{"pool_mode": true, "pool_mode_retry_count": float64(1)}
				if configured {
					account.Credentials["pool_mode_retry_status_codes"] = []any{float64(401), float64(403), float64(429), float64(502), float64(504)}
				}
				for i := 0; i < 3; i++ {
					require.False(t, svc.handleOpenAIAccountUpstreamError(context.Background(), account, status, http.Header{}, []byte(`{"error":{"message":"temporary upstream failure"}}`), "gpt-6-astra"))
				}
				require.Equal(t, !configured, svc.isOpenAIAccountRequestRuntimeBlocked(account, "gpt-6-astra"))
				for _, waited := range []bool{false, true} {
					_, vetoed, reason := svc.AccountSlotVetoLatest(context.Background(), account, OpenAIAccountSlotRequirements{Platform: PlatformOpenAI, RequestedModel: "gpt-6-astra"}, waited)
					require.Equal(t, !configured, vetoed, reason)
					if !configured {
						require.Equal(t, "runtime_blocked", reason)
					}
				}
				require.False(t, svc.isOpenAIAccountRequestRuntimeBlocked(account, "gpt-5.6-sol"))
				// A retry policy must not make a disabled account eligible.
				account.Schedulable = false
				_, vetoed, _ := svc.AccountSlotVetoLatest(context.Background(), account, OpenAIAccountSlotRequirements{Platform: PlatformOpenAI, RequestedModel: "gpt-6-astra"}, false)
				require.True(t, vetoed)
			})
		}
	}
}

// Enabling pool mode is not a command to clear an already recorded cooldown.
func TestPoolEnablePreservesExistingCooldown(t *testing.T) {
	svc := &OpenAIGatewayService{}
	account := &Account{ID: 5109, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	for range 3 {
		svc.recordOpenAIAccountModelTransientFailure(account, "gpt-6-astra", time.Now())
	}
	require.True(t, svc.isOpenAIAccountRequestRuntimeBlocked(account, "gpt-6-astra"))
	account.Credentials = map[string]any{"pool_mode": true, "pool_mode_retry_status_codes": []any{float64(502), float64(504)}}
	require.True(t, svc.isOpenAIAccountRequestRuntimeBlocked(account, "gpt-6-astra"), "configuration alone must not promise immediate recovery")
	svc.ReportOpenAIAccountScheduleResult(account, "gpt-6-astra", true, nil)
	require.False(t, svc.isOpenAIAccountRequestRuntimeBlocked(account, "gpt-6-astra"))
}
