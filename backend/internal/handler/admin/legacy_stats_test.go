package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGroupStatsRejectsFabricatedStatistics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &GroupHandler{}
	r := gin.New()
	r.GET("/groups/:id/stats", h.GetStats)
	for _, tc := range []struct {
		id     string
		status int
	}{{"1", 501}, {"0", 400}, {"-1", 400}, {"bad", 400}} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/groups/"+tc.id+"/stats", nil))
		require.Equal(t, tc.status, w.Code)
		require.NotContains(t, w.Body.String(), "total_api_keys")
	}
}

type handlerRedeemStatsRepo struct{ service.RedeemCodeRepository }

func (handlerRedeemStatsRepo) RedeemStats(context.Context) (*service.RedeemCodeStats, error) {
	return &service.RedeemCodeStats{TotalCodes: 3, ActiveCodes: 1, UsedCodes: 1, ExpiredCodes: 1,
		TotalValueDistributed: 12.34567891, ByType: map[string]int64{"balance": 3}}, nil
}

func TestRedeemStatsHandlerReturnsRepositoryAggregation(t *testing.T) {
	svc := service.NewRedeemService(handlerRedeemStatsRepo{}, nil, nil, nil, nil, nil, nil, nil)
	h := NewRedeemHandler(nil, svc)
	r := gin.New()
	r.GET("/stats", h.GetStats)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/stats", nil))
	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Data struct {
			Total       int64   `json:"total_codes"`
			Active      int64   `json:"active_codes"`
			Distributed float64 `json:"total_value_distributed"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.EqualValues(t, 3, body.Data.Total)
	require.EqualValues(t, 1, body.Data.Active)
	require.Equal(t, 12.34567891, body.Data.Distributed)
}
func TestRedeemStatsUnavailableIsNotSuccessfulZeroData(t *testing.T) {
	h := &RedeemHandler{}
	r := gin.New()
	r.GET("/stats", h.GetStats)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/stats", nil))
	require.Equal(t, 500, w.Code)
	require.NotContains(t, w.Body.String(), "total_codes")
}
