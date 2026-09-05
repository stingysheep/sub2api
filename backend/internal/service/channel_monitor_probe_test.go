//go:build unit

package service

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestIsChannelMonitorProbe(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name   string
		header string
		want   bool
	}{
		{name: "missing", want: false},
		{name: "one", header: "1", want: true},
		{name: "true", header: " TRUE ", want: true},
		{name: "other", header: "0", want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(nil))
			if tc.header != "" {
				c.Request.Header.Set(ChannelMonitorProbeHeader, tc.header)
			}
			require.Equal(t, tc.want, IsChannelMonitorProbe(c))
		})
	}
}

func TestMonitorFailoverMaxSwitchesExceedsOrdinaryCap(t *testing.T) {
	got := MonitorFailoverMaxSwitches()
	require.Greater(t, got, 10)
	require.Equal(t, int(^uint(0)>>1), got)
}

func TestNewChannelMonitorAttemptTimeoutErrorIsFailover(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	account := &Account{ID: 42, Name: "monitor-account", Platform: PlatformOpenAI}

	err := newChannelMonitorAttemptTimeoutError(c, account, time.Now().Add(-channelMonitorAttemptTimeout))

	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusGatewayTimeout, failoverErr.StatusCode)
	require.True(t, failoverErr.SafeToFailoverAfterWrite)
	require.Contains(t, string(failoverErr.ResponseBody), "channel_monitor_timeout")
}
