//go:build unit

package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type slotRefreshUpstream struct {
	service.HTTPUpstream
	authorization  []string
	accountIDs     []int64
	beforeResponse func(int64) *http.Response
}

type slotRefreshSchedulerCache struct {
	fakeSchedulerCache
	repo           *grokCredentialHandlerRepo
	staleHydration map[int64]bool
}

func (s *slotRefreshSchedulerCache) GetAccount(ctx context.Context, id int64) (*service.Account, error) {
	// Delay publication for the scheduler's final hydration, so the handler
	// receives the old object and must replace it during post-slot admission.
	if s.staleHydration[id] {
		s.staleHydration[id] = false
		return s.fakeSchedulerCache.GetAccount(ctx, id)
	}
	return s.repo.GetByID(ctx, id)
}

func (u *slotRefreshUpstream) Do(req *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
	u.authorization = append(u.authorization, req.Header.Get("Authorization"))
	u.accountIDs = append(u.accountIDs, accountID)
	if u.beforeResponse != nil {
		if response := u.beforeResponse(accountID); response != nil {
			return response, nil
		}
	}
	body := `{"id":"test-response","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":0,"completion_tokens":0}}`
	if strings.HasSuffix(req.URL.Path, "/responses") {
		body = `{"id":"test-response","object":"response","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":0,"output_tokens":0}}`
	}
	if strings.HasSuffix(req.URL.Path, "/embeddings") {
		body = `{"object":"list","data":[],"model":"text-embedding-3-small","usage":{"prompt_tokens":0,"total_tokens":0}}`
	}
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
}

func TestChatCompletionsFailoverIgnoresHeartbeatButPreservesBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, semantic := range []bool{false, true} {
		name := "heartbeat_only"
		if semantic {
			name = "semantic_body"
		}
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/openai/v1/chat/completions", strings.NewReader(`{"model":"gpt-5","messages":[{"role":"user","content":"hello"}],"stream":false}`))
			account := service.Account{ID: 91, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true,
				Credentials: map[string]any{"api_key": "test-first", "base_url": "https://example.test"}, Extra: map[string]any{"openai_responses_supported": false}}
			fallback := account
			fallback.ID, fallback.Priority = 92, 10
			repo := &grokCredentialHandlerRepo{accounts: []service.Account{account, fallback}}
			upstream := &slotRefreshUpstream{beforeResponse: func(id int64) *http.Response {
				if id != 91 {
					return nil
				}
				written, err := c.Writer.Write([]byte(": keepalive\n\n"))
				require.NoError(t, err)
				// Same counter contract as service-side stream keepalives.
				c.Set("openai_stream_keepalive_bytes", written)
				if semantic {
					_, err = c.Writer.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"first\"}}]}\n\n"))
					require.NoError(t, err)
				}
				c.Writer.Flush()
				return &http.Response{StatusCode: http.StatusBadGateway, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"error":{"message":"temporary failure"}}`))}
			}}
			cfg := &config.Config{RunMode: config.RunModeSimple}
			cfg.Default.RateMultiplier, cfg.Gateway.MaxAccountSwitches = 1, 2
			billing := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
			t.Cleanup(billing.Stop)
			gw := service.NewOpenAIGatewayService(repo, nil, nil, nil, nil, nil, nil, cfg, nil, nil, service.NewBillingService(cfg, nil), nil, billing, upstream, &service.DeferredService{}, nil, nil, nil, nil, nil, nil, nil)
			h := NewOpenAIGatewayHandler(gw, service.NewConcurrencyService(nil), billing, &service.APIKeyService{}, nil, nil, nil, nil, cfg)
			groupID := int64(90)
			c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{ID: 90, GroupID: &groupID, User: &service.User{ID: 90, Status: service.StatusActive}, Group: &service.Group{ID: groupID, Platform: service.PlatformOpenAI, Status: service.StatusActive}})
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 90, Concurrency: 0})
			h.ChatCompletions(c)
			if semantic {
				require.Equal(t, []int64{91}, upstream.accountIDs)
				require.Contains(t, w.Body.String(), "first")
			} else {
				require.Equal(t, []int64{91, 92}, upstream.accountIDs)
				require.Contains(t, w.Body.String(), `"content":"ok"`)
			}
		})
	}
}

func TestOpenAIHandlersForwardRefreshedSlotAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, endpoint := range []string{"responses", "messages", "chat/completions", "embeddings"} {
		for _, change := range []string{"refresh", "disabled", "model", "capability"} {
			name := endpoint + "/" + change
			t.Run(name, func(t *testing.T) {
				account := service.Account{ID: 91, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Concurrency: 1,
					Credentials: map[string]any{"api_key": "test-old", "base_url": "https://example.test"}}
				if endpoint != "responses" {
					account.Extra = map[string]any{"openai_responses_supported": false}
				}
				fallback := account
				fallback.ID = 92
				fallback.Priority = 10
				fallback.Credentials = map[string]any{"api_key": "test-fallback", "base_url": "https://example.test"}
				repo := &grokCredentialHandlerRepo{accounts: []service.Account{account, fallback}}
				upstream := &slotRefreshUpstream{}
				schedulerCache := &slotRefreshSchedulerCache{fakeSchedulerCache: fakeSchedulerCache{accounts: []*service.Account{&account, &fallback}}, repo: repo, staleHydration: make(map[int64]bool)}
				cfg := &config.Config{RunMode: config.RunModeSimple}
				cfg.Default.RateMultiplier = 1
				cfg.Gateway.MaxAccountSwitches = 2
				billing := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
				t.Cleanup(billing.Stop)
				cache := &concurrencyCacheMock{acquireAccountSlotFn: func(_ context.Context, id int64, _ int, _ string) (bool, error) {
					if id == account.ID {
						repo.mu.Lock()
						repo.accounts[0].Credentials = map[string]any{"api_key": "test-refreshed", "base_url": "https://example.test"}
						repo.accounts[0].Schedulable = change != "disabled"
						if change == "model" {
							repo.accounts[0].Credentials["model_mapping"] = map[string]any{"other-model": "other-model"}
						}
						if change == "capability" {
							repo.accounts[0].Credentials["openai_capabilities"] = map[string]any{"chat_completions": false, "embeddings": false}
						}
						repo.mu.Unlock()
						schedulerCache.staleHydration[id] = true
					}
					return true, nil
				}}
				concurrency := service.NewConcurrencyService(cache)
				snapshot := service.NewSchedulerSnapshotService(schedulerCache, nil, repo, nil, cfg)
				gw := service.NewOpenAIGatewayService(repo, nil, nil, nil, nil, nil, nil, cfg, snapshot, concurrency, service.NewBillingService(cfg, nil), nil, billing, upstream, &service.DeferredService{}, nil, nil, nil, nil, nil, nil, nil)
				h := NewOpenAIGatewayHandler(gw, concurrency, billing, &service.APIKeyService{}, nil, nil, nil, nil, cfg)
				body := `{"model":"gpt-5","messages":[{"role":"user","content":"hello"}],"max_tokens":16,"stream":false}`
				if endpoint == "responses" {
					body = `{"model":"gpt-5","input":"hello","stream":false}`
				}
				if endpoint == "embeddings" {
					body = `{"model":"text-embedding-3-small","input":"hello"}`
				}
				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)
				c.Request = httptest.NewRequest("POST", "/openai/v1/"+endpoint, strings.NewReader(body))
				c.Request.Header.Set("Content-Type", "application/json")
				groupID := int64(90)
				c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{ID: 90, GroupID: &groupID, User: &service.User{ID: 90, Status: service.StatusActive}, Group: &service.Group{ID: groupID, Platform: service.PlatformOpenAI, Status: service.StatusActive, AllowMessagesDispatch: true}})
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 90, Concurrency: 0})
				switch endpoint {
				case "responses":
					h.Responses(c)
				case "messages":
					h.Messages(c)
				case "embeddings":
					h.Embeddings(c)
				default:
					h.ChatCompletions(c)
				}
				require.Equal(t, http.StatusOK, w.Code, w.Body.String())
				if change != "refresh" {
					require.Equal(t, []int64{92}, upstream.accountIDs)
					require.Equal(t, []string{"Bearer test-fallback"}, upstream.authorization)
					require.Equal(t, int32(2), cache.releaseAccountCalled)
				} else {
					require.Equal(t, []int64{91}, upstream.accountIDs)
					require.Equal(t, []string{"Bearer test-refreshed"}, upstream.authorization)
					require.Equal(t, int32(1), cache.releaseAccountCalled)
				}
			})
		}
	}
}
