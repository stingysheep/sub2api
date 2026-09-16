//go:build unit

package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"testing"
)

func TestOperatorBackendModeIdentityOnly(t *testing.T) {
	for _, tc := range []struct {
		role, method, path string
		status             int
	}{{"operator", "GET", "/api/v1/auth/me", 200}, {"operator", "POST", "/api/v1/auth/me", 403}, {"operator", "GET", "/api/v1/user/profile", 403}, {"user", "GET", "/api/v1/auth/me", 403}, {"admin", "GET", "/api/v1/user/profile", 200}} {
		r := gin.New()
		r.Use(func(c *gin.Context) { c.Set(string(ContextKeyUserRole), tc.role); c.Next() }, BackendModeUserGuard(newBackendModeSettingService(t, "true")))
		r.Handle(tc.method, tc.path, func(c *gin.Context) { c.Status(200) })
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		require.Equal(t, tc.status, w.Code, tc.role+tc.method+tc.path)
	}
}
