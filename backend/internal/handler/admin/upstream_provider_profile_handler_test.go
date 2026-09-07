package admin

import (
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUpstreamProfilesRejectsMissingSnapshotBeforeWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, payload := range []string{`{}`, `{"profiles":[]}`, `{"expected_profiles":[]}`, `{"profiles":null,"expected_profiles":[]}`} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("PUT", "/api/v1/admin/settings/upstream-providers", strings.NewReader(payload))
		c.Request.Header.Set("Content-Type", "application/json")
		(&SettingHandler{}).UpdateUpstreamProviderProfiles(c)
		require.Equal(t, 400, c.Writer.Status())
	}
}
