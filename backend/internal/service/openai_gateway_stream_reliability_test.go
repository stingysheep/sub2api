package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCCScanRejectsMissingTerminal(t *testing.T) {
	for _, terminal := range []string{"", `{"choices":[{"finish_reason":"stop"}]}`, `{"usage":{"prompt_tokens":1}}`, "[DONE]"} {
		t.Run(terminal, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
			payload := "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n"
			if terminal != "" {
				payload += "data: " + terminal + "\n\n"
			}
			svc := &OpenAIGatewayService{cfg: &config.Config{}}
			resp := &http.Response{Body: io.NopCloser(strings.NewReader(payload))}
			scan := svc.scanCCStream(c, resp, "test", "test", time.Now(), func(*apicompat.ChatCompletionsChunk) {})
			if terminal == "" {
				require.ErrorIs(t, scan.Err, ErrOpenAIUpstreamStreamTruncated)
			} else {
				require.NoError(t, scan.Err)
			}
		})
	}
}

func TestChatStreamPreambleReadFailureCanFailover(t *testing.T) {
	for _, readErr := range []error{io.EOF, errors.New("connection reset by peer")} {
		t.Run(readErr.Error(), func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			resp := &http.Response{Header: http.Header{}, Body: &openAIChatStreamReadErrorCloser{
				payload: []byte("data: {\"type\":\"response.created\",\"response\":{\"id\":\"r\",\"model\":\"gpt-5\"}}\n\n"), err: readErr,
			}}
			svc := &OpenAIGatewayService{cfg: &config.Config{}}
			result, err := svc.handleChatStreamingResponse(resp, c, &Account{ID: 1, Platform: PlatformOpenAI}, "gpt-5", "gpt-5", "gpt-5", time.Now(), 0)
			var failover *UpstreamFailoverError
			require.ErrorAs(t, err, &failover)
			require.Empty(t, rec.Body.String())
			require.Nil(t, result.FirstTokenMs)
		})
	}
}

func TestChatChunkEmptyContentDoesNotStartOutput(t *testing.T) {
	for _, payload := range []string{
		`{"choices":[{"delta":{"role":"assistant"}}]}`,
		`{"choices":[{"delta":{"content":"","reasoning_content":""}}]}`,
	} {
		var chunk apicompat.ChatCompletionsChunk
		require.NoError(t, json.Unmarshal([]byte(payload), &chunk))
		require.False(t, chatChunkStartsResponsesOutput(&chunk))
	}
}

func TestResponsesChatBridgeDoesNotCompleteTruncatedOutput(t *testing.T) {
	c, rec := newReliabilityContext()
	resp := &http.Response{Header: http.Header{}, Body: io.NopCloser(strings.NewReader("data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n"))}
	svc := &OpenAIGatewayService{cfg: &config.Config{}}
	result, err := svc.streamChatCompletionsAsResponses(c, resp, &Account{ID: 1, Platform: PlatformOpenAI}, "test", nil, nil, false, nil, "test", "test", nil, nil, time.Now())
	require.ErrorIs(t, err, ErrOpenAIUpstreamStreamTruncated)
	require.NotNil(t, result)
	require.Contains(t, rec.Body.String(), "partial")
	require.NotContains(t, rec.Body.String(), "response.completed")
	require.NotContains(t, rec.Body.String(), "[DONE]")
}

func TestCCFirstOutputBudgetIncludesHeaders(t *testing.T) {
	c, _ := newReliabilityContext()
	svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{OpenAIFirstOutputTimeoutSeconds: 1}},
		httpUpstream: &blockingOpenAIResponseHeaderUpstream{canceled: make(chan struct{})}}
	_, err := svc.sendCCUpstreamRequest(context.Background(), c, &Account{ID: 1, Platform: PlatformOpenAI}, "http://upstream.example", []byte(`{"model":"test"}`), true, "test", "", "")
	var failover *UpstreamFailoverError
	require.ErrorAs(t, err, &failover)
	require.Equal(t, http.StatusGatewayTimeout, failover.StatusCode)
}

func TestCCFirstOutputBudgetInterruptsRoleOnlyBody(t *testing.T) {
	c, _ := newReliabilityContext()
	reader, writer := io.Pipe()
	defer writer.Close()
	go func() {
		_, _ = io.WriteString(writer, "data: {\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\n\n")
	}()
	svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{OpenAIFirstOutputTimeoutSeconds: 1}},
		httpUpstream: &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: reader}}}
	resp, err := svc.sendCCUpstreamRequest(context.Background(), c, &Account{ID: 1, Platform: PlatformOpenAI}, "http://upstream.example", []byte(`{"model":"test"}`), true, "test", "", "")
	require.NoError(t, err)
	defer resp.Body.Close()
	emitted := 0
	scan := svc.scanCCStream(c, resp, "test", "test", time.Now(), func(*apicompat.ChatCompletionsChunk) { emitted++ })
	require.ErrorIs(t, scan.Err, errOpenAICompatFirstOutputTimeout)
	require.Zero(t, emitted)
	require.Nil(t, scan.FirstTokenMs)
}

func newReliabilityContext() (*gin.Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	return c, rec
}
