//go:build unit

package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type financialAuthorityAuthRepo struct {
	*stubApiKeyRepo
	state *service.APIKeyFinancialState
	err   error
}

func (r *financialAuthorityAuthRepo) GetAPIKeyFinancialState(context.Context, int64, int64) (*service.APIKeyFinancialState, error) {
	return r.state, r.err
}

func TestAPIKeyAuthEnforcesAuthoritativeFinancialStateAfterCachedSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	past := time.Now().Add(-time.Hour)
	for _, tc := range []struct {
		name   string
		state  service.APIKeyFinancialState
		err    error
		status int
	}{
		{"quota_amount", service.APIKeyFinancialState{Balance: 10, Quota: 1, QuotaUsed: 1, Status: service.StatusActive}, nil, http.StatusTooManyRequests},
		{"quota_status", service.APIKeyFinancialState{Balance: 10, Status: service.StatusAPIKeyQuotaExhausted}, nil, http.StatusTooManyRequests},
		{"disabled", service.APIKeyFinancialState{Balance: 10, Status: "disabled"}, nil, http.StatusUnauthorized},
		{"expired", service.APIKeyFinancialState{Balance: 10, Status: service.StatusActive, ExpiresAt: &past}, nil, http.StatusForbidden},
		{"primary_failure", service.APIKeyFinancialState{}, errors.New("synthetic primary unavailable"), http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := &service.APIKey{ID: 100, UserID: 7, Key: "synthetic-financial-key", Status: service.StatusActive,
				User:  &service.User{ID: 7, Role: service.RoleUser, Status: service.StatusActive, Balance: 10},
				Group: &service.Group{ID: 42, Status: service.StatusActive, Hydrated: true}}
			repo := &financialAuthorityAuthRepo{stubApiKeyRepo: &stubApiKeyRepo{getByKey: func(context.Context, string) (*service.APIKey, error) {
				return original, nil
			}}, state: &service.APIKeyFinancialState{Balance: 10, Quota: 1, Status: service.StatusActive}}
			cfg := &config.Config{RunMode: config.RunModeStandard, APIKeyAuth: config.APIKeyAuthCacheConfig{L1Size: 1000, L1TTLSeconds: 60}}
			svc := service.NewAPIKeyService(repo, nil, nil, nil, nil, nil, cfg)
			router := newAuthTestRouter(svc, nil, cfg)
			request := func() int {
				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
				req.Header.Set("Authorization", "Bearer synthetic-financial-key")
				router.ServeHTTP(w, req)
				return w.Code
			}
			require.Equal(t, http.StatusOK, request())
			repo.state, repo.err = &tc.state, tc.err // No invalidation or PubSub delivery.
			require.Equal(t, tc.status, request())
			require.Equal(t, service.StatusActive, original.Status)
			require.Zero(t, original.QuotaUsed)
			require.Nil(t, original.ExpiresAt)
		})
	}
}
