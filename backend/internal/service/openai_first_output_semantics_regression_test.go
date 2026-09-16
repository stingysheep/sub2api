//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/stretchr/testify/require"
)

func TestFirstOutputSemanticsChatReasoningAlias(t *testing.T) {
	for _, tc := range []struct {
		name  string
		delta string
		want  bool
	}{
		{"alias", `{"reasoning":"thinking"}`, true},
		{"null_primary", `{"reasoning_content":null,"reasoning":"thinking"}`, true},
		{"primary", `{"reasoning_content":"thinking","reasoning":""}`, true},
		{"empty_primary_takes_precedence", `{"reasoning_content":"","reasoning":"ignored"}`, false},
		{"empty_alias", `{"reasoning":""}`, false},
		{"role_only", `{"role":"assistant","content":""}`, false},
		{"tool_name", `{"tool_calls":[{"index":0,"function":{"name":"lookup"}}]}`, true},
		{"tool_arguments", `{"tool_calls":[{"index":0,"function":{"arguments":"{"}}]}`, true},
		{"tool_identity", `{"tool_calls":[{"index":0,"id":"call_1"}]}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var chunk apicompat.ChatCompletionsChunk
			require.NoError(t, json.Unmarshal([]byte(`{"choices":[{"delta":`+tc.delta+`}]}`), &chunk))
			require.Equal(t, tc.want, chatChunkStartsResponsesOutput(&chunk))
		})
	}
}

func TestFirstOutputSemanticsResponsesDelta(t *testing.T) {
	for _, eventType := range []string{
		"response.output_text.delta",
		"response.reasoning_summary_text.delta",
		"response.reasoning_text.delta",
	} {
		for _, delta := range []string{"", " ", "text"} {
			t.Run(eventType+"/"+delta, func(t *testing.T) {
				payload, err := json.Marshal(map[string]string{"type": eventType, "delta": delta})
				require.NoError(t, err)
				require.Equal(t, delta != "", openAIStreamDataStartsClientOutput(string(payload), eventType))
			})
		}
	}
	for _, tc := range []struct {
		name, eventType, payload string
		want                     bool
	}{
		{"function_name_only", "response.output_item.added", `{"item":{"type":"function_call","name":"lookup","arguments":""}}`, false},
		{"function_arguments", "response.output_item.added", `{"item":{"type":"function_call","name":"lookup","arguments":"{}"}}`, true},
		{"custom_name_only", "response.output_item.added", `{"item":{"type":"custom_tool_call","name":"edit","input":""}}`, false},
		{"custom_input", "response.output_item.added", `{"item":{"type":"custom_tool_call","name":"edit","input":"patch"}}`, true},
		{"empty_function_delta_unchanged", "response.function_call_arguments.delta", `{"delta":""}`, true},
		{"empty_custom_delta_unchanged", "response.custom_tool_call_input.delta", `{"delta":""}`, true},
		{"terminal_unchanged", "response.completed", `{"response":{"status":"completed"}}`, true},
		{"malformed_unchanged", "response.output_text.delta", `{`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, openAIStreamDataStartsClientOutput(tc.payload, tc.eventType))
		})
	}
}

func TestFirstOutputSemanticsReasoningAliasForwarding(t *testing.T) {
	for _, protocol := range []string{"raw", "responses", "anthropic"} {
		for _, ending := range []string{"done", "usage", "truncated"} {
			t.Run(protocol+"/"+ending, func(t *testing.T) {
				c, rec := newReliabilityContext()
				payload := "data: {\"choices\":[{\"delta\":{\"reasoning\":\"alias reasoning\"}}]}\n\n"
				if ending == "usage" {
					payload += "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":7,\"completion_tokens\":3}}\n\n"
				}
				if ending != "truncated" {
					payload += "data: [DONE]\n\n"
				}
				_, guard := newOpenAICompatStreamGuard(context.Background(), time.Time{}, 0)
				guard.attach(io.NopCloser(strings.NewReader(payload)))
				defer guard.Close()
				resp := &http.Response{Header: http.Header{}, Body: guard}
				svc := &OpenAIGatewayService{cfg: &config.Config{}}
				account := &Account{ID: 1, Platform: PlatformOpenAI}
				var result *OpenAIForwardResult
				var err error
				switch protocol {
				case "raw":
					result, err = svc.streamRawChatCompletions(c, resp, account, "test", "test", "test", nil, nil, time.Now(), 0)
				case "responses":
					result, err = svc.streamChatCompletionsAsResponses(c, resp, account, "test", nil, nil, false, nil, "test", "test", nil, nil, time.Now())
				case "anthropic":
					result, err = svc.streamChatCompletionsAsAnthropic(c, resp, account, "test", "test", "test", nil, nil, time.Now())
				}
				if ending == "truncated" {
					require.ErrorIs(t, err, ErrOpenAIUpstreamStreamTruncated)
					var failover *UpstreamFailoverError
					require.False(t, errors.As(err, &failover), "emitted reasoning must not be replayed")
					require.NotContains(t, rec.Body.String(), "response.completed")
					require.NotContains(t, rec.Body.String(), "message_stop")
				} else {
					require.NoError(t, err)
				}
				require.NotNil(t, result)
				require.Contains(t, rec.Body.String(), "alias reasoning")
				require.NotNil(t, result.FirstTokenMs)
				if ending == "usage" {
					require.Equal(t, 7, result.Usage.InputTokens)
					require.Equal(t, 3, result.Usage.OutputTokens)
				}
				guard.expire(errOpenAICompatFirstOutputTimeout, true)
				require.NoError(t, guard.Err(), "reasoning must disarm the first-output deadline")
			})
		}
	}
}

type firstOutputSemanticsDeadlineReader struct {
	guard *openAICompatStreamGuard
}

func (r firstOutputSemanticsDeadlineReader) Read([]byte) (int, error) {
	// Fire the deadline after the preceding frames have been processed, without sleeps.
	r.guard.expire(errOpenAICompatFirstOutputTimeout, true)
	return 0, io.EOF
}

func TestFirstOutputSemanticsEmptyDeltaKeepsDeadline(t *testing.T) {
	for _, eventType := range []string{"response.output_text.delta", "response.reasoning_summary_text.delta", "response.reasoning_text.delta"} {
		for _, withOutput := range []bool{false, true} {
			name := eventType + "/before_output"
			if withOutput {
				name = eventType + "/after_output"
			}
			t.Run(name, func(t *testing.T) {
				c, rec := newReliabilityContext()
				payload := "data: {\"type\":\"response.created\",\"response\":{\"id\":\"r\"}}\n\n"
				if withOutput {
					payload += "data: {\"type\":\"response.output_text.delta\",\"delta\":\"visible\"}\n\n"
				}
				payload += "data: {\"type\":\"" + eventType + "\",\"delta\":\"\"}\n\n: heartbeat\n\n"
				_, guard := newOpenAICompatStreamGuard(context.Background(), time.Time{}, 0)
				guard.attach(io.NopCloser(io.MultiReader(strings.NewReader(payload), firstOutputSemanticsDeadlineReader{guard})))
				defer guard.Close()
				resp := &http.Response{Header: http.Header{}, Body: guard}
				svc := &OpenAIGatewayService{cfg: &config.Config{}}
				_, err := svc.handleChatStreamingResponse(resp, c, &Account{ID: 1, Platform: PlatformOpenAI}, "test", "test", "test", time.Now(), 0)
				require.Error(t, err)
				var failover *UpstreamFailoverError
				if withOutput {
					require.NoError(t, guard.Err())
					require.False(t, errors.As(err, &failover))
					require.Contains(t, rec.Body.String(), "visible")
				} else {
					require.ErrorIs(t, guard.Err(), errOpenAICompatFirstOutputTimeout)
					require.ErrorAs(t, err, &failover)
					require.False(t, c.Writer.Written())
					require.Empty(t, rec.Body.String())
				}
			})
		}
	}
}
