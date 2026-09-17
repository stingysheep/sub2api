package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

type upstreamBalanceHTTPStub struct {
	responses []*http.Response
	requests  []*http.Request
}

type upstreamBalanceSnapshotRepoStub struct {
	AccountRepository
	accountID int64
	updates   map[string]any
}

func (s *upstreamBalanceSnapshotRepoStub) UpdateExtra(_ context.Context, id int64, updates map[string]any) error {
	s.accountID = id
	s.updates = updates
	return nil
}

func (s *upstreamBalanceHTTPStub) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	return s.DoWithTLS(req, "", 0, 0, nil)
}

func (s *upstreamBalanceHTTPStub) DoWithTLS(req *http.Request, _ string, _ int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	s.requests = append(s.requests, req)
	if len(s.responses) == 0 {
		return nil, io.EOF
	}
	response := s.responses[0]
	s.responses = s.responses[1:]
	return response, nil
}

func upstreamBalanceResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func newUpstreamBalanceTestService(upstream HTTPUpstream) *AccountTestService {
	return NewAccountTestService(nil, nil, nil, nil, nil, upstream, &config.Config{
		Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}},
	}, nil)
}

func upstreamBalanceTestAccount(baseURL string) *Account {
	return &Account{ID: 73, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 2, Credentials: map[string]any{
		"base_url": baseURL, "api_key": "do-not-return-this-key",
	}}
}

func TestQueryUpstreamBalanceCCSwitchUsageResponse(t *testing.T) {
	upstream := &upstreamBalanceHTTPStub{responses: []*http.Response{
		upstreamBalanceResponse(http.StatusOK, `{"planName":"pro","isValid":true,"unit":"USD","quota":{"limit":20,"used":5,"remaining":15}}`),
	}}
	result, err := newUpstreamBalanceTestService(upstream).QueryUpstreamBalance(context.Background(), upstreamBalanceTestAccount("https://relay.ccswitch.example/v1"))
	require.NoError(t, err)
	require.Equal(t, "sub2api", result.Provider)
	require.Equal(t, http.StatusOK, result.StatusCode)
	require.Len(t, result.Entries, 1)
	require.Equal(t, "pro", result.Entries[0].PlanName)
	require.Equal(t, 15.0, *result.Entries[0].Remaining)
	require.Equal(t, 20.0, *result.Entries[0].Total)
	require.Equal(t, 5.0, *result.Entries[0].Used)
	require.Equal(t, "https://relay.ccswitch.example/v1/usage", upstream.requests[0].URL.String())
	require.Equal(t, "Bearer do-not-return-this-key", upstream.requests[0].Header.Get("Authorization"))
	require.True(t, HTTPUpstreamRedirectsDisabled(upstream.requests[0].Context()))
	require.True(t, HTTPUpstreamPublicHostsOnly(upstream.requests[0].Context()))
}

func TestQueryUpstreamBalancePersistsOnlySanitizedNumericSnapshot(t *testing.T) {
	repo := &upstreamBalanceSnapshotRepoStub{}
	upstream := &upstreamBalanceHTTPStub{responses: []*http.Response{
		upstreamBalanceResponse(http.StatusOK, `{"planName":"pro","isValid":true,"unit":"usd","quota":{"limit":20,"used":5,"remaining":15}}`),
	}}
	service := newUpstreamBalanceTestService(upstream)
	service.accountRepo = repo
	result, err := service.QueryUpstreamBalance(context.Background(), upstreamBalanceTestAccount("https://relay.ccswitch.example/v1"))
	require.NoError(t, err)
	require.Len(t, result.Entries, 1)
	require.Equal(t, int64(73), repo.accountID)
	snapshot, ok := repo.updates[upstreamBalanceSnapshotExtraKey].(upstreamBalanceSnapshot)
	require.True(t, ok)
	require.Equal(t, upstreamBalanceSnapshotVersion, snapshot.SchemaVersion)
	require.Equal(t, "sub2api", snapshot.Provider)
	require.Equal(t, http.StatusOK, snapshot.StatusCode)
	require.Len(t, snapshot.Entries, 1)
	require.Equal(t, "USD", snapshot.Entries[0].Unit)
	require.Equal(t, 15.0, *snapshot.Entries[0].Remaining)
	encoded, err := json.Marshal(snapshot)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "api_key")
	require.NotContains(t, string(encoded), "base_url")
	require.NotContains(t, string(encoded), "do-not-return-this-key")
}

func TestQueryUpstreamBalanceDoesNotReplaceSnapshotOnAuthenticationFailure(t *testing.T) {
	repo := &upstreamBalanceSnapshotRepoStub{}
	upstream := &upstreamBalanceHTTPStub{responses: []*http.Response{
		upstreamBalanceResponse(http.StatusUnauthorized, `{"error":"credential rejected"}`),
	}}
	service := newUpstreamBalanceTestService(upstream)
	service.accountRepo = repo
	_, err := service.QueryUpstreamBalance(context.Background(), upstreamBalanceTestAccount("https://relay.example/v1"))
	require.NoError(t, err)
	require.Nil(t, repo.updates)
}

func TestQueryUpstreamBalanceRetriesOnlyForSchemaMismatch(t *testing.T) {
	upstream := &upstreamBalanceHTTPStub{responses: []*http.Response{
		upstreamBalanceResponse(http.StatusOK, `{"unexpected":true}`),
		upstreamBalanceResponse(http.StatusOK, `{"data":{"balance":"12.5","currency":"USD"}}`),
	}}
	result, err := newUpstreamBalanceTestService(upstream).QueryUpstreamBalance(context.Background(), upstreamBalanceTestAccount("https://relay.example/v1"))
	require.NoError(t, err)
	require.Equal(t, "generic", result.Provider)
	require.Len(t, upstream.requests, 2)
	require.Equal(t, "/v1/usage", upstream.requests[0].URL.Path)
	require.Equal(t, "/user/balance", upstream.requests[1].URL.Path)
	require.Equal(t, 12.5, *result.Entries[0].Remaining)
}

func TestQueryUpstreamBalanceAuthenticationFailureDoesNotFallback(t *testing.T) {
	upstream := &upstreamBalanceHTTPStub{responses: []*http.Response{
		upstreamBalanceResponse(http.StatusUnauthorized, `{"error":"do not expose upstream text"}`),
	}}
	result, err := newUpstreamBalanceTestService(upstream).QueryUpstreamBalance(context.Background(), upstreamBalanceTestAccount("https://relay.example/v1"))
	require.NoError(t, err)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, http.StatusUnauthorized, result.StatusCode)
	require.False(t, result.Entries[0].IsValid)
	require.NotContains(t, result.Entries[0].InvalidMessage, "upstream text")
}

func TestQueryUpstreamBalanceDeepSeekMultipleCurrencies(t *testing.T) {
	upstream := &upstreamBalanceHTTPStub{responses: []*http.Response{
		upstreamBalanceResponse(http.StatusOK, `{"is_available":true,"balance_infos":[{"currency":"CNY","total_balance":"7.5"},{"currency":"USD","total_balance":2}]}`),
	}}
	result, err := newUpstreamBalanceTestService(upstream).QueryUpstreamBalance(context.Background(), upstreamBalanceTestAccount("https://api.deepseek.com/v1"))
	require.NoError(t, err)
	require.Equal(t, "deepseek", result.Provider)
	require.Len(t, result.Entries, 2)
	require.Equal(t, "CNY", result.Entries[0].Unit)
	require.Equal(t, 7.5, *result.Entries[0].Remaining)
	require.Equal(t, "USD", result.Entries[1].Unit)
	require.Equal(t, "/user/balance", upstream.requests[0].URL.Path)
}

func TestQueryUpstreamBalanceMoonshotUsesOfficialTemplate(t *testing.T) {
	upstream := &upstreamBalanceHTTPStub{responses: []*http.Response{
		upstreamBalanceResponse(http.StatusOK, `{"data":{"available_balance":18}}`),
	}}
	result, err := newUpstreamBalanceTestService(upstream).QueryUpstreamBalance(context.Background(), upstreamBalanceTestAccount("https://api.moonshot.cn/v1"))
	require.NoError(t, err)
	require.Equal(t, "moonshot", result.Provider)
	require.Equal(t, "/v1/users/me/balance", upstream.requests[0].URL.Path)
	require.Equal(t, 18.0, *result.Entries[0].Remaining)
	require.Equal(t, "CNY", result.Entries[0].Unit)
}

func TestParseUpstreamBalanceOfficialTemplates(t *testing.T) {
	tests := []struct {
		name, provider, body string
		remaining            float64
	}{
		{"stepfun", "stepfun", `{"data":{"balance":10}}`, 10},
		{"siliconflow", "siliconflow", `{"data":{"totalBalance":"11.5"}}`, 11.5},
		{"openrouter", "openrouter", `{"data":{"total_credits":20,"total_usage":4}}`, 16},
		{"novita", "novita", `{"data":{"availableBalance":30000}}`, 3},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entries, ok := parseUpstreamBalance(test.provider, []byte(test.body))
			require.True(t, ok)
			require.Equal(t, test.remaining, *entries[0].Remaining)
		})
	}
}

func TestQueryUpstreamBalanceSiliconFlowInternationalUsesUSD(t *testing.T) {
	upstream := &upstreamBalanceHTTPStub{responses: []*http.Response{
		upstreamBalanceResponse(http.StatusOK, `{"data":{"totalBalance":"11.5"}}`),
	}}
	result, err := newUpstreamBalanceTestService(upstream).QueryUpstreamBalance(context.Background(), upstreamBalanceTestAccount("https://api.siliconflow.com/v1"))
	require.NoError(t, err)
	require.Equal(t, "siliconflow-en", result.Provider)
	require.Equal(t, "USD", result.Entries[0].Unit)
}

func TestParseUpstreamBalanceMarksExhaustedCreditInvalid(t *testing.T) {
	for _, test := range []struct {
		provider string
		body     string
	}{
		{"openrouter", `{"data":{"total_credits":5,"total_usage":5}}`},
		{"novita", `{"availableBalance":0}`},
	} {
		entries, ok := parseUpstreamBalance(test.provider, []byte(test.body))
		require.True(t, ok)
		require.False(t, entries[0].IsValid)
		require.NotEmpty(t, entries[0].InvalidMessage)
	}
}

func TestParseUpstreamBalanceRejectsNonFiniteStrings(t *testing.T) {
	for _, value := range []string{"NaN", "+Inf", "-Inf"} {
		entries, ok := parseUpstreamBalance("generic", []byte(`{"balance":"`+value+`"}`))
		require.False(t, ok)
		require.Nil(t, entries)
	}
}
