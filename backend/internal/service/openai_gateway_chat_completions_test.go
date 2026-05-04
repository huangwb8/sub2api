package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestForwardAsChatCompletionsDirectStreamPreservesBillingMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(nil))
	c.Request.Header.Set("User-Agent", "openai-test-client")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
			"x-request-id": []string{"cc-direct-stream"},
		},
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			`data: {"id":"chatcmpl_1","choices":[{"delta":{"content":"hi"}}]}`,
			`data: {"id":"chatcmpl_1","choices":[],"usage":{"prompt_tokens":11,"completion_tokens":7,"total_tokens":18,"prompt_tokens_details":{"cached_tokens":3}}}`,
			`data: [DONE]`,
			``,
		}, "\n"))),
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	account := &Account{
		ID:          42,
		Name:        "chatapi-direct",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeChatAPI,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-upstream",
			"base_url": "https://chat.example.com/v1",
		},
	}

	result, err := svc.ForwardAsChatCompletionsDirect(
		context.Background(),
		c,
		account,
		[]byte(`{"model":"gpt-5.1","stream":true,"service_tier":"flex","reasoning":{"effort":"high"},"messages":[{"role":"user","content":"hi"}]}`),
		"",
		"",
	)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "https://chat.example.com/v1/chat/completions", upstream.lastReq.URL.String())
	require.Equal(t, "openai-test-client", upstream.lastReq.Header.Get("User-Agent"))
	require.True(t, gjson.GetBytes(upstream.lastBody, "stream_options.include_usage").Bool())
	require.Equal(t, 11, result.Usage.InputTokens)
	require.Equal(t, 7, result.Usage.OutputTokens)
	require.Equal(t, 3, result.Usage.CacheReadInputTokens)
	require.NotNil(t, result.ReasoningEffort)
	require.Equal(t, "high", *result.ReasoningEffort)
	require.NotNil(t, result.ServiceTier)
	require.Equal(t, "flex", *result.ServiceTier)
}
