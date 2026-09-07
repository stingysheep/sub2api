package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type upstreamProviderProfilesRequest struct {
	Profiles         *[]service.UpstreamProviderProfile `json:"profiles"`
	ExpectedProfiles *[]service.UpstreamProviderProfile `json:"expected_profiles"`
}

// GetUpstreamProviderProfiles returns administrator-only account naming and
// endpoint templates.
func (h *SettingHandler) GetUpstreamProviderProfiles(c *gin.Context) {
	profiles, err := h.settingService.GetUpstreamProviderProfiles(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, profiles)
}

// UpdateUpstreamProviderProfiles replaces the small, administrator-owned
// template collection atomically at the settings row level.
func (h *SettingHandler) UpdateUpstreamProviderProfiles(c *gin.Context) {
	var req upstreamProviderProfilesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if req.Profiles == nil || req.ExpectedProfiles == nil {
		response.BadRequest(c, "profiles and expected_profiles are required; reload before saving")
		return
	}
	profiles, err := h.settingService.SetUpstreamProviderProfiles(c.Request.Context(), *req.Profiles, *req.ExpectedProfiles)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, profiles)
}
