package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestBuildOpenAIEndpointURLImages(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		endpoint string
		want     string
	}{
		{name: "official root adds v1 endpoint", base: "https://api.openai.com", endpoint: "/v1/images/generations", want: "https://api.openai.com/v1/images/generations"},
		{name: "official v1 keeps v1", base: "https://api.openai.com/v1", endpoint: "/v1/images/edits", want: "https://api.openai.com/v1/images/edits"},
		{name: "custom root follows responses compatibility", base: "https://example.com", endpoint: "/v1/images/generations", want: "https://example.com/images/generations"},
		{name: "custom v1 appends endpoint", base: "https://example.com/v1", endpoint: "/v1/images/generations", want: "https://example.com/v1/images/generations"},
		{name: "custom explicit non v1 endpoint stays stable", base: "https://example.com/images/generations", endpoint: "/v1/images/generations", want: "https://example.com/images/generations"},
		{name: "explicit endpoint stays stable", base: "https://example.com/v1/images/generations", endpoint: "/v1/images/generations", want: "https://example.com/v1/images/generations"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, buildOpenAIEndpointURL(tt.base, tt.endpoint))
		})
	}
}

func TestParseOpenAIImagesUsage(t *testing.T) {
	body := []byte(`{
		"created": 123,
		"data": [{"b64_json": "abc"}],
		"usage": {
			"total_tokens": 120,
			"input_tokens": 45,
			"output_tokens": 75,
			"input_tokens_details": {"text_tokens": 10, "image_tokens": 35},
			"output_tokens_details": {"image_tokens": 75}
		}
	}`)

	got := parseOpenAIImagesUsage(body)
	require.Equal(t, 45, got.InputTokens)
	require.Equal(t, 75, got.OutputTokens)
	require.Equal(t, 35, got.ImageInputTokens)
	require.Equal(t, 75, got.ImageOutputTokens)
}

func TestOpenAIGatewayService_ForwardAsImageGeneration_NonStream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(nil))
	c.Request.Header.Set("Authorization", "Bearer inbound")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"img-req"}},
		Body: io.NopCloser(strings.NewReader(`{
			"created": 123,
			"data": [{"b64_json": "abc"}],
			"usage": {
				"total_tokens": 8,
				"input_tokens": 3,
				"output_tokens": 5,
				"output_tokens_details": {"image_tokens": 5}
			}
		}`)),
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	account := &Account{
		ID:          7,
		Name:        "openai-apikey",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{"api_key": "sk-upstream", "base_url": "https://api.openai.com"},
	}

	result, err := svc.ForwardAsImageGeneration(
		context.Background(),
		c,
		account,
		[]byte(`{"model":"gpt-image-2","prompt":"draw a cat","n":1}`),
		"gpt-image-2",
		"",
		"",
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"b64_json"`)
	require.Equal(t, "https://api.openai.com/v1/images/generations", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer sk-upstream", upstream.lastReq.Header.Get("authorization"))
	require.Equal(t, "gpt-image-2", result.Model)
	require.Equal(t, "gpt-image-2", result.BillingModel)
	require.Equal(t, 3, result.Usage.InputTokens)
	require.Equal(t, 5, result.Usage.OutputTokens)
	require.Equal(t, 5, result.Usage.ImageOutputTokens)
	require.Equal(t, 1, result.ImageCount)
	require.Equal(t, "1K", result.ImageSize)
}

func TestOpenAIGatewayService_BuildOpenAIImagesRequest_OAuthExperimental(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	c.Request.Header.Set("Accept-Language", "zh-CN")
	c.Request.Header.Set("User-Agent", "Mozilla/5.0")

	svc := &OpenAIGatewayService{
		cfg: &config.Config{
			Gateway: config.GatewayConfig{
				OpenAIOAuthImagesExperimentalEnabled: true,
			},
			Security: config.SecurityConfig{
				URLAllowlist: config.URLAllowlistConfig{Enabled: false},
			},
		},
	}
	account := &Account{
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"chatgpt_account_id": "acct_123"},
	}
	req, err := svc.buildOpenAIImagesRequest(
		context.Background(),
		c,
		account,
		openaiImagesGenerationsEndpoint,
		bytes.NewReader([]byte(`{}`)),
		"oauth-token",
		"application/json",
		&OpenAIOAuthImagesCapability{Supported: true, Strategy: OpenAIOAuthImagesStrategyAPIPlatformImagesWithOAuth},
	)
	require.NoError(t, err)
	require.Equal(t, "https://api.openai.com/v1/images/generations", req.URL.String())
	require.Equal(t, "Bearer oauth-token", req.Header.Get("authorization"))
	require.Equal(t, "acct_123", req.Header.Get("chatgpt-account-id"))
	require.Equal(t, "codex_cli_rs", req.Header.Get("originator"))
	require.Equal(t, "zh-CN", req.Header.Get("accept-language"))
	require.Equal(t, codexCLIUserAgent, req.Header.Get("user-agent"))
}
