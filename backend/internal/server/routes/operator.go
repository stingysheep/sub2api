package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func RegisterOperatorRoutes(v1 *gin.RouterGroup, h *handler.Handlers, jwt middleware.JWTAuthMiddleware, settings *service.SettingService, panelRateLimiter *middleware.PanelRateLimiter) {
	op := v1.Group("/operator")
	op.Use(gin.HandlerFunc(jwt), middleware.OperatorRoleGate(), panelRateLimiter.Global(), middleware.OperatorAdminComplianceGuard(settings))
	op.GET("/users", h.Operator.ListUsers)
	op.GET("/users/:id", h.Operator.GetUser)
	op.POST("/users/:id/balance-adjustments", h.Operator.AdjustBalance)
	op.GET("/users/:id/payment-orders", h.Operator.ListOrders)
	op.GET("/users/:id/usage", panelRateLimiter.Heavy(), h.Operator.Usage)
	op.GET("/groups", h.Operator.ListGroups)
	op.GET("/users/:id/balance-history", h.Operator.BalanceHistory)
	op.GET("/channel-status", panelRateLimiter.Heavy(), h.Operator.ChannelStatus)
}
