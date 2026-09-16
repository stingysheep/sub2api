package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOperatorRoleGateRunsBeforeHandlerAfterJWTNext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, role := range []string{"user", "", "operator", "admin"} {
		t.Run(role, func(t *testing.T) {
			r := gin.New()
			called := false
			r.Use(func(c *gin.Context) { c.Set(string(ContextKeyUserRole), role); c.Next() }, OperatorRoleGate())
			r.GET("/", func(c *gin.Context) { called = true; c.Status(http.StatusNoContent) })
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
			allowed := role == "operator" || role == "admin"
			require.Equal(t, allowed, called)
			if allowed {
				require.Equal(t, 204, w.Code)
			} else {
				require.Equal(t, 403, w.Code)
			}
		})
	}
}
