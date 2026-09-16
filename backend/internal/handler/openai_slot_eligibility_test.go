//go:build unit

package handler

import (
	"context"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type slotLatestAccountRepo struct {
	service.AccountRepository
	latest *service.Account
	reads  int
}

func (r *slotLatestAccountRepo) GetByID(context.Context, int64) (*service.Account, error) {
	r.reads++
	return r.latest, nil
}

func TestAcquireResponsesAccountSlotRefreshBudget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, path := range []string{"acquired", "fast", "wait"} {
		t.Run(path, func(t *testing.T) {
			selected := profitSlotTestAccount(31, 0.3)
			repo := &slotLatestAccountRepo{latest: profitSlotTestAccount(31, 0.3)}
			gw := service.NewOpenAIGatewayService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
			attempts := 0
			cache := &concurrencyCacheMock{acquireAccountSlotFn: func(context.Context, int64, int, string) (bool, error) {
				attempts++
				return path != "wait" || attempts > 1, nil
			}}
			h := &OpenAIGatewayHandler{gatewayService: gw, concurrencyHelper: NewConcurrencyHelper(service.NewConcurrencyService(cache), SSEPingFormatClaude, 0)}
			selection := &service.AccountSelectionResult{Account: selected, Acquired: path == "acquired", ReleaseFunc: func() {}, WaitPlan: &service.AccountWaitPlan{AccountID: selected.ID, MaxConcurrency: 2, MaxWaiting: 2, Timeout: time.Second}}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
			started := false
			release, status := h.acquireResponsesAccountSlot(c, nil, "", selection, false, &started, zap.NewNop())
			require.Equal(t, openAISlotAcquireOK, status)
			release()
			if path == "wait" {
				require.Equal(t, 1, repo.reads)
				require.Same(t, repo.latest, selection.Account)
			} else {
				require.Zero(t, repo.reads, "immediate admission must not add a database read")
				require.Same(t, selected, selection.Account)
			}
		})
	}
}

func TestAcquireResponsesAccountSlotLatestEligibility(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, path := range []string{"acquired", "fast", "wait"} {
		for _, change := range []string{"disabled", "cooldown", "rate_limit", "model", "capability", "continuation", "profit", "refresh", "missing", "stale_snapshot"} {
			t.Run(path+"/"+change, func(t *testing.T) {
				selected := profitSlotTestAccount(31, 0.3)
				latest := profitSlotTestAccount(31, 0.3)
				latest.Name = "refreshed-account"
				until := time.Now().Add(time.Hour)
				requirements := service.OpenAIAccountSlotRequirements{RequestedModel: "gpt-5", RequiredCapability: service.OpenAIEndpointCapabilityChatCompletions}
				switch change {
				case "disabled":
					latest.Schedulable = false
				case "cooldown":
					latest.TempUnschedulableUntil = &until
				case "rate_limit":
					latest.RateLimitResetAt = &until
				case "missing":
					latest = nil
				case "model":
					latest.Credentials = map[string]any{"model_mapping": map[string]any{"other-model": "other-model"}}
				case "capability":
					requirements.RequiredCapability = service.OpenAIEndpointCapabilityEmbeddings
					latest.Type = service.AccountTypeOAuth
				case "continuation":
					requirements.RequireHTTPContinuation = true
					latest.Type = service.AccountTypeOAuth
				case "profit":
					latest = profitSlotTestAccount(31, 0.8)
				case "stale_snapshot":
					selected.UpdatedAt = time.Now()
					latest.UpdatedAt = selected.UpdatedAt.Add(-time.Minute)
					latest.Schedulable = false
				}
				repo := &slotLatestAccountRepo{latest: latest}
				snapshot := service.NewSchedulerSnapshotService(&fakeSchedulerCache{accounts: []*service.Account{latest}}, nil, repo, nil, nil)
				gw := service.NewOpenAIGatewayService(repo, nil, nil, nil, nil, nil, nil, nil, snapshot, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
				attempts := 0
				cache := &concurrencyCacheMock{acquireAccountSlotFn: func(context.Context, int64, int, string) (bool, error) {
					attempts++
					return path != "wait" || attempts > 1, nil
				}}
				h := &OpenAIGatewayHandler{gatewayService: gw, concurrencyHelper: NewConcurrencyHelper(service.NewConcurrencyService(cache), SSEPingFormatClaude, 0)}
				selection := &service.AccountSelectionResult{Account: selected, Acquired: path == "acquired", WaitPlan: &service.AccountWaitPlan{AccountID: selected.ID, MaxConcurrency: 2, MaxWaiting: 2, Timeout: time.Second}}
				selection.ReleaseFunc = func() { atomic.AddInt32(&cache.releaseAccountCalled, 1) }
				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)
				c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
				if change == "profit" {
					c.Request = c.Request.WithContext(profitSlotTestContext(t, gw, 50, false))
				}
				started := false
				release, status := h.acquireResponsesAccountSlot(c, nil, "", selection, false, &started, zap.NewNop(), requirements)
				if change == "refresh" || (change == "stale_snapshot" && path != "wait") {
					require.Equal(t, openAISlotAcquireOK, status)
					if change == "stale_snapshot" {
						require.Same(t, selected, selection.Account)
					} else {
						require.Same(t, latest, selection.Account)
					}
					require.NotNil(t, release)
					release()
				} else {
					require.Equal(t, openAISlotAcquireProfitVetoed, status)
					require.Nil(t, release)
				}
				require.Equal(t, int32(1), atomic.LoadInt32(&cache.releaseAccountCalled))
				require.Empty(t, w.Body.String())
				if path == "wait" {
					require.Equal(t, 1, repo.reads)
				} else if change != "missing" {
					require.Zero(t, repo.reads, "healthy snapshot admission must not read the database")
				}
			})
		}
	}
}
