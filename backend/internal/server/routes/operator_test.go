package routes

import (
	"context"
	"encoding/json"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"testing"
)

type operatorRouteSettings struct {
	service.SettingRepository
	ack           bool
	global, heavy int
}

func (s *operatorRouteSettings) GetValue(_ context.Context, k string) (string, error) {
	if k == service.SettingKeyPanelRateLimitSettings {
		b, _ := json.Marshal(service.PanelRateLimitSettings{Enabled: true, UserRPM: s.global, HeavyRPM: s.heavy, ExemptAdmin: true})
		return string(b), nil
	}
	if s.ack {
		b, _ := json.Marshal(service.AdminComplianceAcknowledgement{Version: service.AdminComplianceVersion})
		return string(b), nil
	}
	return "", service.ErrSettingNotFound
}
func operatorTestRouter(t *testing.T, role string, ack bool, global, heavy int) (*gin.Engine, *miniredis.Miniredis) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	settings := service.NewSettingService(&operatorRouteSettings{ack: ack, global: global, heavy: heavy}, &config.Config{})
	r := gin.New()
	jwt := middleware.JWTAuthMiddleware(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
		c.Set(string(middleware.ContextKeyUserRole), role)
		c.Next()
	})
	RegisterOperatorRoutes(r.Group("/api/v1"), &handler.Handlers{Operator: handler.NewOperatorHandler(nil, nil)}, jwt, settings, middleware.NewPanelRateLimiter(client, settings))
	return r, mr
}
func TestOperatorRoutesPreserveAdminCompliance(t *testing.T) {
	for _, tc := range []struct {
		role string
		ack  bool
		want int
	}{{"admin", false, 423}, {"admin", true, 400}, {"operator", false, 400}, {"user", true, 403}} {
		t.Run(tc.role, func(t *testing.T) {
			r, _ := operatorTestRouter(t, tc.role, tc.ack, 100, 100)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/operator/users/2/balance-adjustments?unknown=1", nil))
			require.Equal(t, tc.want, w.Code)
			if tc.want == 423 {
				require.Contains(t, w.Body.String(), "ADMIN_COMPLIANCE_ACK_REQUIRED")
			}
		})
	}
}
func TestOperatorGlobalRateLimitCoversEveryRoute(t *testing.T) {
	for _, tc := range []struct{ method, path string }{{"GET", "/users"}, {"GET", "/users/2"}, {"GET", "/users/2/balance-history"}, {"GET", "/users/2/payment-orders"}, {"GET", "/users/2/usage"}, {"POST", "/users/2/balance-adjustments"}, {"GET", "/groups"}, {"GET", "/channel-status"}} {
		t.Run(tc.path, func(t *testing.T) {
			r, mr := operatorTestRouter(t, "operator", false, 1, 100)
			require.NoError(t, mr.Set("rate_limit:panel:global:user:7", "1"))
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(tc.method, "/api/v1/operator"+tc.path, nil))
			require.Equal(t, 429, w.Code)
			require.NotEmpty(t, w.Header().Get("Retry-After"))
		})
	}
}
func TestOperatorAggregatesConsumeHeavyLimit(t *testing.T) {
	for _, path := range []string{"/users/2/usage", "/channel-status"} {
		t.Run(path, func(t *testing.T) {
			r, mr := operatorTestRouter(t, "operator", false, 100, 1)
			require.NoError(t, mr.Set("rate_limit:panel:heavy:user:7", "1"))
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/operator"+path, nil))
			require.Equal(t, 429, w.Code)
			count, e := mr.Get("rate_limit:panel:global:user:7")
			require.NoError(t, e)
			require.Equal(t, "1", count)
		})
	}
}
