package service

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// ChannelMonitorProbeHeader marks requests emitted by the channel monitor.
// The marker is advisory: it only changes timeout/failover behavior and does
// not grant access or bypass authentication.
const ChannelMonitorProbeHeader = "X-Sub2API-Channel-Monitor"

const channelMonitorAttemptTimeout = 30 * time.Second

func IsChannelMonitorProbe(c *gin.Context) bool {
	if c == nil {
		return false
	}
	value := strings.TrimSpace(c.GetHeader(ChannelMonitorProbeHeader))
	return strings.EqualFold(value, "1") || strings.EqualFold(value, "true")
}

// MonitorFailoverMaxSwitches disables the ordinary request switch cap for a
// monitor probe. The probe's request deadline remains the final safety bound;
// account selection exhaustion, rather than an arbitrary switch count, decides
// when the account pool has been fully tried.
func MonitorFailoverMaxSwitches() int {
	return int(^uint(0) >> 1)
}

func newChannelMonitorAttemptTimeoutError(c *gin.Context, account *Account, started time.Time) *UpstreamFailoverError {
	elapsed := time.Since(started)
	if account != nil {
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			Platform:           account.Platform,
			AccountID:          account.ID,
			AccountName:        account.Name,
			UpstreamStatusCode: http.StatusGatewayTimeout,
			Kind:               "channel_monitor_timeout",
			Message:            "channel monitor account attempt timed out",
			Detail:             fmt.Sprintf("elapsed_ms=%d timeout_ms=%d", elapsed.Milliseconds(), channelMonitorAttemptTimeout.Milliseconds()),
		})
	}
	return &UpstreamFailoverError{
		StatusCode:               http.StatusGatewayTimeout,
		ResponseBody:             []byte(`{"error":{"type":"channel_monitor_timeout","message":"account attempt timed out"}}`),
		SafeToFailoverAfterWrite: true,
	}
}
