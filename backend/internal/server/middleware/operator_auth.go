package middleware

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// OperatorAuth intentionally wraps JWT-only authentication; admin API keys never reach it.
func OperatorRoleGate() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := GetUserRoleFromContext(c)
		if role != service.RoleOperator && role != service.RoleAdmin {
			response.Forbidden(c, "Operator access required")
			c.Abort()
			return
		}
		c.Next()
	}
}

// Admin callers retain the existing acknowledgement requirement even through
// the narrower operator API. Operator callers do not need an admin acknowledgement.
func OperatorAdminComplianceGuard(settings *service.SettingService) gin.HandlerFunc {
	guard := AdminComplianceGuard(settings)
	return func(c *gin.Context) {
		role, _ := GetUserRoleFromContext(c)
		if role == service.RoleAdmin {
			guard(c)
			return
		}
		c.Next()
	}
}
