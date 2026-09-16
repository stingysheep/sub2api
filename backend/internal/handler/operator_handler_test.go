package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOperatorAdjustmentRejectsAmbiguousBodies(t *testing.T) {
	for _, body := range []string{`{"operation":"add","operation":"subtract"}`, `{"amount":"1","admin":true}`, `{"amount":null}`, `{"amount":1}`, `[]`, `{"amount":"1"} {}`, `{"reason":"` + strings.Repeat("a", 4096) + `"}`} {
		_, e := decodeOperatorAdjustment(strings.NewReader(body))
		require.Error(t, e, body)
	}
	r, e := decodeOperatorAdjustment(strings.NewReader(`{"operation":"add","amount":"1.00000001","source":"free","reason":"support"}`))
	require.NoError(t, e)
	require.Equal(t, "1.00000001", r.Amount)
}
func TestOperatorQueryRejectsUnknownDuplicateAndMalformed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, raw := range []string{"role=admin", "include_deleted=", "page=1&page=2", "page=%ZZ", "page=1;page_size=100"} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("GET", "/?"+raw, nil)
		require.False(t, operatorQuery(c, "page", "page_size"), raw)
	}
	for _, raw := range []string{"page=0", "page=-1", "page=no", "page_size=101", "page_size=0", "page_size="} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("GET", "/?"+raw, nil)
		_, _, ok := operatorPage(c)
		require.False(t, ok, raw)
	}
}
