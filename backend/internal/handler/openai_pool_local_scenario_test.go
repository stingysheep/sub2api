//go:build unit

package handler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// No network: the actual Responses handler and gateway call this scripted transport.
type poolScenarioUpstream struct {
	service.HTTPUpstream
	mu       sync.Mutex
	hits     []int64
	status   int
	scenario string
	cancel   context.CancelFunc
}

func (u *poolScenarioUpstream) Do(_ *http.Request, _ string, id int64, _ int) (*http.Response, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.hits = append(u.hits, id)
	call := len(u.hits)
	if u.scenario == "cancel" {
		u.cancel()
	}
	if u.scenario == "partial" {
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader("data: {\"type\":\"response.output_text.delta\",\"delta\":\"unique-partial\"}\n\ndata: {\"type\":\"response.failed\",\"response\":{\"id\":\"resp_test\",\"status\":\"failed\",\"error\":{\"code\":\"server_error\",\"message\":\"upstream failed\"}}}\n\n"))}, nil
	}
	success := ((u.scenario == "recover" || u.scenario == "stream_recover") && call > 1) || (u.scenario == "switch" && id == 2)
	if success {
		if u.scenario == "stream_recover" {
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader("data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_pool_recovered\",\"status\":\"completed\",\"output\":[],\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}}\n\n"))}, nil
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"id":"resp_pool_recovered","object":"response","model":"gpt-5.1","status":"completed","output":[],"usage":{"input_tokens":1,"output_tokens":1}}`))}, nil
	}
	return &http.Response{StatusCode: u.status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"error":{"message":"temporary upstream failure","type":"upstream_error"}}`))}, nil
}

func newPoolScenarioHandler(t *testing.T, upstream *poolScenarioUpstream, configured bool) *OpenAIGatewayHandler {
	t.Helper()
	accounts := make([]service.Account, 2)
	for i := range accounts {
		credentials := map[string]any{"api_key": "local-fixture-only", "base_url": "https://upstream.invalid", "pool_mode": true, "pool_mode_retry_count": float64(1)}
		if configured {
			credentials["pool_mode_retry_status_codes"] = []any{float64(401), float64(403), float64(429), float64(502), float64(504)}
		}
		accounts[i] = service.Account{ID: int64(i + 1), Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Priority: i, GroupIDs: []int64{3131}, Credentials: credentials, Extra: map[string]any{"openai_passthrough": true}}
	}
	repo := &grokCredentialHandlerRepo{accounts: accounts}
	cfg := &config.Config{RunMode: config.RunModeSimple}
	cfg.Gateway.MaxAccountSwitches = 1
	billing := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billing.Stop)
	gateway := service.NewOpenAIGatewayService(repo, nil, nil, nil, nil, nil, nil, cfg, nil, nil, service.NewBillingService(cfg, nil), service.NewRateLimitService(repo, nil, cfg, nil, nil), billing, upstream, &service.DeferredService{}, nil, nil, nil, nil, nil, nil, nil)
	h := NewOpenAIGatewayHandler(gateway, service.NewConcurrencyService(nil), billing, service.NewAPIKeyService(nil, nil, nil, nil, nil, nil, cfg), nil, nil, nil, nil, cfg)
	h.maxAccountSwitches = 1
	return h
}

func TestPoolLocalResponsesScenarios(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, status := range []int{502, 504} {
		for _, scenario := range []string{"recover", "stream_recover", "switch", "exhaust", "partial", "cancel", "default_codes"} {
			t.Run(fmt.Sprintf("%d_%s", status, scenario), func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
				defer cancel()
				upstream := &poolScenarioUpstream{status: status, scenario: scenario, cancel: cancel}
				h := newPoolScenarioHandler(t, upstream, scenario != "default_codes")
				c, rec := newOpenAIResponsesFailoverTestContext(t, ctx)
				if scenario == "partial" || scenario == "stream_recover" {
					c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-5.1","input":"hello","stream":true}`)).WithContext(ctx)
					c.Request.Header.Set("Content-Type", "application/json")
				}
				h.Responses(c)
				upstream.mu.Lock()
				hits := append([]int64(nil), upstream.hits...)
				upstream.mu.Unlock()
				switch scenario {
				case "recover", "stream_recover":
					require.Equal(t, []int64{1, 1}, hits)
					require.Equal(t, 200, rec.Code, rec.Body.String())
					require.Contains(t, rec.Body.String(), "resp_pool_recovered")
				case "switch":
					require.Equal(t, []int64{1, 1, 2}, hits)
					require.Equal(t, 200, rec.Code, rec.Body.String())
				case "exhaust":
					require.Equal(t, []int64{1, 1, 2, 2}, hits)
					require.GreaterOrEqual(t, rec.Code, 400)
				case "partial":
					require.Equal(t, []int64{1}, hits)
					require.Equal(t, 1, strings.Count(rec.Body.String(), "unique-partial"))
					require.Contains(t, rec.Body.String(), "response.failed")
					require.NotContains(t, rec.Body.String(), "response.completed")
				case "cancel":
					require.Equal(t, []int64{1}, hits)
				case "default_codes":
					require.Equal(t, []int64{1, 2}, hits)
					require.GreaterOrEqual(t, rec.Code, 400)
				}
			})
		}
	}
}
