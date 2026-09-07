package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type groupCategoriesRequest struct {
	Categories         *[]service.GroupCategory `json:"categories"`
	ExpectedCategories *[]service.GroupCategory `json:"expected_categories"`
}

func (h *SettingHandler) GetGroupCategories(c *gin.Context) {
	categories, err := h.settingService.GetGroupCategories(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, categories)
}

func (h *SettingHandler) UpdateGroupCategories(c *gin.Context) {
	var req groupCategoriesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if req.Categories == nil || req.ExpectedCategories == nil {
		response.BadRequest(c, "categories and expected_categories are required; reload before saving")
		return
	}
	categories, err := h.settingService.SetGroupCategories(c.Request.Context(), *req.Categories, *req.ExpectedCategories)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, categories)
}
