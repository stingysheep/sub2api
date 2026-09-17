package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
)

// UpstreamBalanceEntry is one plan, quota, or wallet value reported by an upstream.
// Pointer amounts distinguish an omitted upstream field from a real zero balance.
type UpstreamBalanceEntry struct {
	PlanName       string   `json:"plan_name"`
	Remaining      *float64 `json:"remaining"`
	Total          *float64 `json:"total"`
	Used           *float64 `json:"used"`
	Unit           string   `json:"unit"`
	IsValid        bool     `json:"is_valid"`
	InvalidMessage string   `json:"invalid_message"`
}

// UpstreamBalanceQueryResult is deliberately credential-free. It is suitable for
// direct delivery to the administrator UI and is never persisted or logged.
type UpstreamBalanceQueryResult struct {
	Entries    []UpstreamBalanceEntry `json:"entries"`
	Provider   string                 `json:"provider"`
	FetchedAt  time.Time              `json:"fetched_at"`
	StatusCode int                    `json:"status_code"`
}

const (
	upstreamBalanceQueryTimeout  = 10 * time.Second
	upstreamBalanceQueryMaxBytes = 64 * 1024
)

var errUpstreamBalanceUnavailable = errors.New("upstream balance query is unavailable")

type upstreamBalanceCandidate struct {
	provider string
	path     string
}

// QueryUpstreamBalance fetches a live, read-only balance from an API-key account.
// It deliberately lives on AccountTestService so it shares the same URL allowlist,
// proxy, HTTP transport, TLS profile, redirect, response-size, and deadline guards.
func (s *AccountTestService) QueryUpstreamBalance(ctx context.Context, account *Account) (*UpstreamBalanceQueryResult, error) {
	if s == nil || s.httpUpstream == nil {
		return nil, errUpstreamBalanceUnavailable
	}
	if account == nil || (account.Type != AccountTypeAPIKey && account.Type != AccountTypeUpstream) {
		return nil, errors.New("account is not an API key or upstream account")
	}
	apiKey := strings.TrimSpace(account.GetCredential("api_key"))
	baseURL := strings.TrimSpace(account.GetCredential("base_url"))
	if apiKey == "" {
		return nil, errors.New("account API key is empty")
	}
	if baseURL == "" {
		return nil, errors.New("account base URL is empty")
	}
	// One administrator click has one wall-clock budget, including a compatible
	// relay's fallback endpoint. Candidate requests inherit this deadline.
	ctx, cancel := context.WithTimeout(ctx, upstreamBalanceQueryTimeout)
	defer cancel()
	normalizedBaseURL, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	proxyURL := ""
	if account.ProxyID != nil {
		if account.Proxy == nil || account.Proxy.ID != *account.ProxyID {
			return nil, errors.New("account proxy is unavailable")
		}
		proxyURL = account.Proxy.URL()
	}
	var tlsProfile *tlsfingerprint.Profile
	if s.tlsFPProfileService != nil {
		tlsProfile = s.tlsFPProfileService.ResolveTLSProfile(account)
	}

	var lastResult *UpstreamBalanceQueryResult
	for _, candidate := range upstreamBalanceCandidates(normalizedBaseURL) {
		result, retry, err := s.queryUpstreamBalanceCandidate(ctx, account, normalizedBaseURL, apiKey, proxyURL, tlsProfile, candidate)
		if err != nil {
			return nil, err
		}
		if !retry {
			return result, nil
		}
		lastResult = result
	}
	if lastResult != nil {
		lastResult.Entries = []UpstreamBalanceEntry{{IsValid: false, InvalidMessage: "Upstream balance response is not recognized"}}
		return lastResult, nil
	}
	return &UpstreamBalanceQueryResult{
		Entries:   []UpstreamBalanceEntry{{IsValid: false, InvalidMessage: "Upstream balance response is not recognized"}},
		Provider:  "generic",
		FetchedAt: time.Now().UTC(),
	}, nil
}

func (s *AccountTestService) queryUpstreamBalanceCandidate(
	ctx context.Context,
	account *Account,
	normalizedBaseURL, apiKey, proxyURL string,
	tlsProfile *tlsfingerprint.Profile,
	candidate upstreamBalanceCandidate,
) (*UpstreamBalanceQueryResult, bool, error) {
	requestURL, err := upstreamBalanceURL(normalizedBaseURL, candidate.path)
	if err != nil {
		return nil, false, fmt.Errorf("build upstream balance URL: %w", err)
	}
	requestCtx, cancel := context.WithTimeout(ctx, upstreamBalanceQueryTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, false, errors.New("build upstream balance request")
	}
	requestContext := WithHTTPUpstreamRedirectsDisabled(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileDefault))
	// The default URL policy validates syntax only. Balance probes are an
	// additional credential-bearing entry point, so default installations must
	// also reject loopback, private, link-local, and metadata destinations. A
	// deliberately configured allowlist remains authoritative and can opt into
	// private upstreams through AllowPrivateHosts.
	if s.cfg == nil || !s.cfg.Security.URLAllowlist.Enabled {
		requestContext = WithHTTPUpstreamPublicHostsOnly(requestContext)
	}
	req = req.WithContext(requestContext)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	account.ApplyHeaderOverrides(req.Header)
	// Header overrides are compatibility settings, but must never change the
	// credential this server intentionally selected for the account.
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := s.httpUpstream.DoWithTLS(req, proxyURL, account.ID, account.Concurrency, tlsProfile)
	if err != nil {
		return nil, false, errors.New("upstream balance request failed")
	}
	if resp == nil || resp.Body == nil {
		return nil, false, errors.New("upstream balance response is empty")
	}
	defer func() { _ = resp.Body.Close() }()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, upstreamBalanceQueryMaxBytes+1))
	if readErr != nil {
		return nil, false, errors.New("upstream balance response could not be read")
	}
	result := &UpstreamBalanceQueryResult{Provider: candidate.provider, FetchedAt: time.Now().UTC(), StatusCode: resp.StatusCode}
	if len(body) > upstreamBalanceQueryMaxBytes {
		result.Entries = []UpstreamBalanceEntry{{IsValid: false, InvalidMessage: "Upstream balance response is too large"}}
		return result, false, nil
	}
	// Authentication failures must not trigger fallback paths. In particular this
	// prevents a rejected vendor key from being tried against unrelated endpoints.
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		result.Entries = []UpstreamBalanceEntry{{IsValid: false, InvalidMessage: "Upstream authentication failed"}}
		return result, false, nil
	}
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed {
		return result, true, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		result.Entries = []UpstreamBalanceEntry{{IsValid: false, InvalidMessage: "Upstream balance request failed"}}
		return result, false, nil
	}
	entries, ok := parseUpstreamBalance(candidate.provider, body)
	if !ok {
		return result, true, nil
	}
	result.Entries = entries
	return result, false, nil
}

func upstreamBalanceCandidates(baseURL string) []upstreamBalanceCandidate {
	host := strings.ToLower(strings.TrimSuffix(upstreamBalanceHost(baseURL), "."))
	var candidates []upstreamBalanceCandidate
	switch {
	case strings.Contains(host, "moonshot") || strings.Contains(host, "kimi"):
		candidates = append(candidates, upstreamBalanceCandidate{"moonshot", "/v1/users/me/balance"})
	case strings.Contains(host, "deepseek"):
		candidates = append(candidates, upstreamBalanceCandidate{"deepseek", "/user/balance"})
	case strings.Contains(host, "stepfun"):
		candidates = append(candidates, upstreamBalanceCandidate{"stepfun", "/v1/accounts"})
	case strings.Contains(host, "siliconflow.com"):
		candidates = append(candidates, upstreamBalanceCandidate{"siliconflow-en", "/v1/user/info"})
	case strings.Contains(host, "siliconflow"):
		candidates = append(candidates, upstreamBalanceCandidate{"siliconflow", "/v1/user/info"})
	case strings.Contains(host, "openrouter"):
		candidates = append(candidates, upstreamBalanceCandidate{"openrouter", "/api/v1/credits"})
	case strings.Contains(host, "novita"):
		candidates = append(candidates, upstreamBalanceCandidate{"novita", "/v3/user/balance"})
	}
	// CCSwitch/Sub2API and unknown compatible relays advertise /v1/usage first.
	candidates = append(candidates,
		upstreamBalanceCandidate{"sub2api", "/v1/usage"},
		upstreamBalanceCandidate{"generic", "/user/balance"},
	)
	return uniqueUpstreamBalanceCandidates(candidates)
}

func uniqueUpstreamBalanceCandidates(in []upstreamBalanceCandidate) []upstreamBalanceCandidate {
	seen := make(map[string]struct{}, len(in))
	out := make([]upstreamBalanceCandidate, 0, len(in))
	for _, item := range in {
		if _, exists := seen[item.path]; exists {
			continue
		}
		seen[item.path] = struct{}{}
		out = append(out, item)
	}
	return out
}

func upstreamBalanceHost(baseURL string) string {
	u, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// upstreamBalanceURL always uses the origin root. Account base_url commonly ends
// in /v1, while the vendor endpoints below are specified from their API root.
func upstreamBalanceURL(baseURL, endpoint string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", errors.New("invalid base URL")
	}
	u.Path = "/" + strings.TrimLeft(endpoint, "/")
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

func parseUpstreamBalance(provider string, body []byte) ([]UpstreamBalanceEntry, bool) {
	var payload any
	if json.Unmarshal(body, &payload) != nil {
		return nil, false
	}
	root, ok := payload.(map[string]any)
	if !ok {
		return nil, false
	}
	if provider == "deepseek" {
		if entries, found := parseDeepSeekBalance(root); found {
			return entries, true
		}
	}
	entry, found := parseGenericUpstreamBalance(root)
	if !found {
		return nil, false
	}
	normalizeUpstreamBalanceEntry(provider, &entry)
	return []UpstreamBalanceEntry{entry}, true
}

func parseDeepSeekBalance(root map[string]any) ([]UpstreamBalanceEntry, bool) {
	raw, ok := root["balance_infos"].([]any)
	if !ok || len(raw) == 0 {
		return nil, false
	}
	valid := boolAt(root, "is_available", true)
	entries := make([]UpstreamBalanceEntry, 0, len(raw))
	for _, value := range raw {
		item, ok := value.(map[string]any)
		if !ok {
			continue
		}
		balance, found := numberAt(item, "total_balance")
		if !found {
			continue
		}
		unit, _ := stringAt(item, "currency")
		entries = append(entries, UpstreamBalanceEntry{PlanName: "payg", Remaining: upstreamBalanceFloatPtr(balance), Unit: strings.ToUpper(unit), IsValid: valid, InvalidMessage: invalidMessage(valid)})
	}
	return entries, len(entries) > 0
}

func parseGenericUpstreamBalance(root map[string]any) (UpstreamBalanceEntry, bool) {
	data := root
	if nested, ok := root["data"].(map[string]any); ok {
		data = nested
	}
	planName, _ := firstString(data, root, "planName", "plan_name", "plan", "name")
	unit, _ := firstString(data, root, "unit", "currency")
	valid := firstBool(data, root, "isValid", "is_valid", "available", "is_available")
	if valid == nil {
		v := true
		valid = &v
	}
	remaining, hasRemaining := firstNumber(data, root, "remaining", "balance", "available_balance", "availableBalance", "total_balance", "totalBalance", "credit")
	total, hasTotal := firstNumber(data, root, "total", "limit", "total_credits", "quota")
	used, hasUsed := firstNumber(data, root, "used", "usage", "total_usage", "consumed")
	if quota, ok := data["quota"].(map[string]any); ok {
		if !hasRemaining {
			remaining, hasRemaining = numberAt(quota, "remaining")
		}
		if !hasTotal {
			total, hasTotal = firstNumber(quota, quota, "limit", "total", "quota")
		}
		if !hasUsed {
			used, hasUsed = numberAt(quota, "used")
		}
	}
	if !hasRemaining && !hasTotal && !hasUsed {
		return UpstreamBalanceEntry{}, false
	}
	if !hasRemaining && hasTotal && hasUsed {
		remaining, hasRemaining = total-used, true
	}
	entry := UpstreamBalanceEntry{PlanName: planName, Unit: unit, IsValid: *valid, InvalidMessage: invalidMessage(*valid)}
	if hasRemaining {
		entry.Remaining = upstreamBalanceFloatPtr(remaining)
	}
	if hasTotal {
		entry.Total = upstreamBalanceFloatPtr(total)
	}
	if hasUsed {
		entry.Used = upstreamBalanceFloatPtr(used)
	}
	return entry, true
}

func normalizeUpstreamBalanceEntry(provider string, entry *UpstreamBalanceEntry) {
	if entry == nil {
		return
	}
	if provider == "novita" {
		entry.Unit = "USD"
		entry.Remaining = scaleUpstreamBalance(entry.Remaining, 10000)
		entry.Total = scaleUpstreamBalance(entry.Total, 10000)
		entry.Used = scaleUpstreamBalance(entry.Used, 10000)
		markUpstreamBalanceUnavailableWhenEmpty(entry)
		return
	}
	if entry.Unit != "" {
		return
	}
	switch provider {
	case "moonshot", "stepfun", "siliconflow":
		entry.Unit = "CNY"
	case "siliconflow-en":
		entry.Unit = "USD"
	case "openrouter":
		entry.Unit = "USD"
	}
	if provider == "openrouter" {
		markUpstreamBalanceUnavailableWhenEmpty(entry)
	}
}

func markUpstreamBalanceUnavailableWhenEmpty(entry *UpstreamBalanceEntry) {
	if entry == nil || entry.Remaining == nil || *entry.Remaining > 0 {
		return
	}
	entry.IsValid = false
	entry.InvalidMessage = "Upstream reports no balance remaining"
}

func scaleUpstreamBalance(value *float64, divisor float64) *float64 {
	if value == nil || divisor == 0 {
		return value
	}
	scaled := *value / divisor
	return &scaled
}

func firstNumber(primary, fallback map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		if value, ok := numberAt(primary, key); ok {
			return value, true
		}
		if value, ok := numberAt(fallback, key); ok {
			return value, true
		}
	}
	return 0, false
}

func numberAt(values map[string]any, key string) (float64, bool) {
	v, ok := values[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return finiteUpstreamBalanceNumber(n)
	case json.Number:
		value, err := n.Float64()
		if err != nil {
			return 0, false
		}
		return finiteUpstreamBalanceNumber(value)
	case string:
		value, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		if err != nil {
			return 0, false
		}
		return finiteUpstreamBalanceNumber(value)
	default:
		return 0, false
	}
}

func finiteUpstreamBalanceNumber(value float64) (float64, bool) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	return value, true
}

func firstString(primary, fallback map[string]any, keys ...string) (string, bool) {
	for _, key := range keys {
		if value, ok := stringAt(primary, key); ok {
			return value, true
		}
		if value, ok := stringAt(fallback, key); ok {
			return value, true
		}
	}
	return "", false
}

func stringAt(values map[string]any, key string) (string, bool) {
	v, ok := values[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return strings.TrimSpace(s), ok
}

func firstBool(primary, fallback map[string]any, keys ...string) *bool {
	for _, key := range keys {
		if value, ok := primary[key].(bool); ok {
			return &value
		}
		if value, ok := fallback[key].(bool); ok {
			return &value
		}
	}
	return nil
}

func boolAt(values map[string]any, key string, defaultValue bool) bool {
	if value, ok := values[key].(bool); ok {
		return value
	}
	return defaultValue
}

func upstreamBalanceFloatPtr(value float64) *float64 { return &value }

func invalidMessage(valid bool) string {
	if valid {
		return ""
	}
	return "Upstream reports this balance is unavailable"
}
