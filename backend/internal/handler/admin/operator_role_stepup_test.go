package admin

import (
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http"
	"testing"
)

func TestOperatorPromotionAndCreationRequireExistingStepUp(t *testing.T) {
	r, _ := setupRoleStepUpRouter(t)
	rec := doJSON(t, r, http.MethodPut, "/api/v1/admin/users/1", map[string]any{"role": "operator"})
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	rec = doJSON(t, r, http.MethodPost, "/api/v1/admin/users", map[string]any{"email": "operator@example.com", "password": "pass123", "role": "operator"})
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
func TestOperatorSelfDemotionIsRejected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 2})
		c.Next()
	})
	svc := newStubAdminService()
	svc.users = append(svc.users, service.User{ID: 2, Email: "admin@example.com", Role: service.RoleAdmin, Status: service.StatusActive})
	h := NewUserHandler(svc, nil, nil, nil, nil, nil, nil)
	r.PUT("/api/v1/admin/users/:id", h.Update)
	rec := doJSON(t, r, http.MethodPut, "/api/v1/admin/users/2", map[string]any{"role": "operator"})
	require.Equal(t, http.StatusBadRequest, rec.Code)
}
