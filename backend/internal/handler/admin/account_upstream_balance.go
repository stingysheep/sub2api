package admin

import (
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// QueryUpstreamBalance fetches a live, non-persisted balance from an API-key upstream.
// POST /api/v1/admin/accounts/:id/upstream-balance
func (h *AccountHandler) QueryUpstreamBalance(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || accountID <= 0 {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	if h == nil || h.adminService == nil || h.accountTestService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Upstream balance query is unavailable")
		return
	}
	account, err := h.adminService.GetAccount(c.Request.Context(), accountID)
	if err != nil || account == nil {
		response.NotFound(c, "Account not found")
		return
	}
	result, err := h.accountTestService.QueryUpstreamBalance(c.Request.Context(), account)
	if err != nil {
		response.BadRequest(c, "Unable to query upstream balance")
		return
	}
	response.Success(c, result)
}
