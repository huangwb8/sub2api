//go:build unit

package service

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"syscall"
	"testing"
	"testing/iotest"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type sseErrAfterReader struct {
	payload []byte
	sent    bool
	err     error
}

func (r *sseErrAfterReader) Read(p []byte) (int, error) {
	if !r.sent {
		r.sent = true
		return copy(p, r.payload), nil
	}
	if r.err != nil {
		return 0, r.err
	}
	return 0, io.EOF
}

func (r *sseErrAfterReader) Close() error {
	return nil
}

// --- parseSSEUsage 测试 ---

func newMinimalGatewayService() *GatewayService {
	return &GatewayService{
		cfg: &config.Config{
			Gateway: config.GatewayConfig{
				StreamDataIntervalTimeout: 0,
				MaxLineSize:               defaultMaxLineSize,
			},
		},
		rateLimitService: &RateLimitService{},
	}
}

func TestParseSSEUsage_MessageStart(t *testing.T) {
	svc := newMinimalGatewayService()
	usage := &ClaudeUsage{}

	data := `{"type":"message_start","message":{"usage":{"input_tokens":100,"cache_creation_input_tokens":50,"cache_read_input_tokens":200}}}`
	svc.parseSSEUsage(data, usage)

	require.Equal(t, 100, usage.InputTokens)
	require.Equal(t, 50, usage.CacheCreationInputTokens)
	require.Equal(t, 200, usage.CacheReadInputTokens)
	require.Equal(t, 0, usage.OutputTokens, "message_start 不应设置 output_tokens")
}

func TestParseSSEUsage_MessageDelta(t *testing.T) {
	svc := newMinimalGatewayService()
	usage := &ClaudeUsage{}

	data := `{"type":"message_delta","usage":{"output_tokens":42}}`
	svc.parseSSEUsage(data, usage)

	require.Equal(t, 42, usage.OutputTokens)
	require.Equal(t, 0, usage.InputTokens, "message_delta 的 output_tokens 不应影响已有的 input_tokens")
}

func TestParseSSEUsage_DeltaDoesNotOverwriteStartValues(t *testing.T) {
	svc := newMinimalGatewayService()
	usage := &ClaudeUsage{}

	// 先处理 message_start
	svc.parseSSEUsage(`{"type":"message_start","message":{"usage":{"input_tokens":100}}}`, usage)
	require.Equal(t, 100, usage.InputTokens)

	// 再处理 message_delta（output_tokens > 0, input_tokens = 0）
	svc.parseSSEUsage(`{"type":"message_delta","usage":{"output_tokens":50}}`, usage)
	require.Equal(t, 100, usage.InputTokens, "delta 中 input_tokens=0 不应覆盖 start 中的值")
	require.Equal(t, 50, usage.OutputTokens)
}

func TestParseSSEUsage_DeltaOverwritesWithNonZero(t *testing.T) {
	svc := newMinimalGatewayService()
	usage := &ClaudeUsage{}

	// GLM 等 API 会在 delta 中包含所有 usage 信息
	svc.parseSSEUsage(`{"type":"message_delta","usage":{"input_tokens":200,"output_tokens":100,"cache_creation_input_tokens":30,"cache_read_input_tokens":60}}`, usage)
	require.Equal(t, 200, usage.InputTokens)
	require.Equal(t, 100, usage.OutputTokens)
	require.Equal(t, 30, usage.CacheCreationInputTokens)
	require.Equal(t, 60, usage.CacheReadInputTokens)
}

func TestParseSSEUsage_DeltaDoesNotResetCacheCreationBreakdown(t *testing.T) {
	svc := newMinimalGatewayService()
	usage := &ClaudeUsage{}

	// 先在 message_start 中写入非零 5m/1h 明细
	svc.parseSSEUsage(`{"type":"message_start","message":{"usage":{"input_tokens":100,"cache_creation":{"ephemeral_5m_input_tokens":30,"ephemeral_1h_input_tokens":70}}}}`, usage)
	require.Equal(t, 30, usage.CacheCreation5mTokens)
	require.Equal(t, 70, usage.CacheCreation1hTokens)

	// 后续 delta 带默认 0，不应覆盖已有非零值
	svc.parseSSEUsage(`{"type":"message_delta","usage":{"output_tokens":12,"cache_creation":{"ephemeral_5m_input_tokens":0,"ephemeral_1h_input_tokens":0}}}`, usage)
	require.Equal(t, 30, usage.CacheCreation5mTokens, "delta 的 0 值不应重置 5m 明细")
	require.Equal(t, 70, usage.CacheCreation1hTokens, "delta 的 0 值不应重置 1h 明细")
	require.Equal(t, 12, usage.OutputTokens)
}

func TestParseSSEUsage_InvalidJSON(t *testing.T) {
	svc := newMinimalGatewayService()
	usage := &ClaudeUsage{}

	// 无效 JSON 不应 panic
	svc.parseSSEUsage("not json", usage)
	require.Equal(t, 0, usage.InputTokens)
	require.Equal(t, 0, usage.OutputTokens)
}

func TestParseSSEUsage_UnknownType(t *testing.T) {
	svc := newMinimalGatewayService()
	usage := &ClaudeUsage{}

	// 不是 message_start 或 message_delta 的类型
	svc.parseSSEUsage(`{"type":"content_block_delta","delta":{"text":"hello"}}`, usage)
	require.Equal(t, 0, usage.InputTokens)
	require.Equal(t, 0, usage.OutputTokens)
}

func TestParseSSEUsage_EmptyString(t *testing.T) {
	svc := newMinimalGatewayService()
	usage := &ClaudeUsage{}

	svc.parseSSEUsage("", usage)
	require.Equal(t, 0, usage.InputTokens)
}

func TestParseSSEUsage_DoneEvent(t *testing.T) {
	svc := newMinimalGatewayService()
	usage := &ClaudeUsage{}

	// [DONE] 事件不应影响 usage
	svc.parseSSEUsage("[DONE]", usage)
	require.Equal(t, 0, usage.InputTokens)
}

// --- 流式响应端到端测试 ---

func TestHandleStreamingResponse_CacheTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newMinimalGatewayService()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	pr, pw := io.Pipe()
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: pr}

	go func() {
		defer func() { _ = pw.Close() }()
		_, _ = pw.Write([]byte("data: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":10,\"cache_creation_input_tokens\":20,\"cache_read_input_tokens\":30}}}\n\n"))
		_, _ = pw.Write([]byte("data: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":15}}\n\n"))
		_, _ = pw.Write([]byte("data: [DONE]\n\n"))
	}()

	result, err := svc.handleStreamingResponse(context.Background(), resp, c, &Account{ID: 1}, time.Now(), "model", "model", false)
	_ = pr.Close()
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.usage)
	require.Equal(t, 10, result.usage.InputTokens)
	require.Equal(t, 15, result.usage.OutputTokens)
	require.Equal(t, 20, result.usage.CacheCreationInputTokens)
	require.Equal(t, 30, result.usage.CacheReadInputTokens)
}

func TestHandleStreamingResponse_EmptyStream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newMinimalGatewayService()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	pr, pw := io.Pipe()
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: pr}

	go func() {
		// 直接关闭，不发送任何事件
		_ = pw.Close()
	}()

	result, err := svc.handleStreamingResponse(context.Background(), resp, c, &Account{ID: 1}, time.Now(), "model", "model", false)
	_ = pr.Close()
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing terminal event")
	require.NotNil(t, result)
}

func TestHandleStreamingResponse_SpecialCharactersInJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newMinimalGatewayService()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	pr, pw := io.Pipe()
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: pr}

	go func() {
		defer func() { _ = pw.Close() }()
		// 包含特殊字符的 content_block_delta（引号、换行、Unicode）
		_, _ = pw.Write([]byte("data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello \\\"world\\\"\\n你好\"}}\n\n"))
		_, _ = pw.Write([]byte("data: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":5}}}\n\n"))
		_, _ = pw.Write([]byte("data: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":3}}\n\n"))
		_, _ = pw.Write([]byte("data: [DONE]\n\n"))
	}()

	result, err := svc.handleStreamingResponse(context.Background(), resp, c, &Account{ID: 1}, time.Now(), "model", "model", false)
	_ = pr.Close()
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.usage)
	require.Equal(t, 5, result.usage.InputTokens)
	require.Equal(t, 3, result.usage.OutputTokens)

	// 验证响应中包含转发的数据
	body := rec.Body.String()
	require.Contains(t, body, "content_block_delta", "响应应包含转发的 SSE 事件")
}

func TestHandleStreamingResponse_StreamReadErrorBeforeOutput_TriggersFailover(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newMinimalGatewayService()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(iotest.ErrReader(io.ErrUnexpectedEOF)),
	}

	result, err := svc.handleStreamingResponse(context.Background(), resp, c, &Account{ID: 1}, time.Now(), "model", "model", false)

	require.Error(t, err)
	require.Nil(t, result)

	var failoverErr *UpstreamFailoverError
	require.True(t, errors.As(err, &failoverErr))
	require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
	require.True(t, failoverErr.RetryableOnSameAccount)
	require.Contains(t, ExtractUpstreamErrorMessage(failoverErr.ResponseBody), "upstream stream disconnected")
	require.NotContains(t, rec.Body.String(), "stream_read_error")
}

func TestHandleStreamingResponse_StreamReadErrorAfterOutput_PassesThrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newMinimalGatewayService()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: &sseErrAfterReader{
			payload: []byte("data: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":5}}}\n\n"),
			err:     io.ErrUnexpectedEOF,
		},
	}

	result, err := svc.handleStreamingResponse(context.Background(), resp, c, &Account{ID: 1}, time.Now(), "model", "model", false)

	require.Error(t, err)
	require.NotNil(t, result)
	var failoverErr *UpstreamFailoverError
	require.False(t, errors.As(err, &failoverErr))

	body := rec.Body.String()
	require.Contains(t, body, "event: error\n")
	require.Contains(t, body, `"type":"error"`)
	require.Contains(t, body, `"stream_read_error"`)
	require.Contains(t, body, "upstream stream disconnected")
}

func TestSanitizeStreamError_StripsNetworkAddresses(t *testing.T) {
	src, err := net.ResolveTCPAddr("tcp", "10.0.0.1:54321")
	require.NoError(t, err)
	dst, err := net.ResolveTCPAddr("tcp", "52.1.2.3:443")
	require.NoError(t, err)

	raw := &net.OpError{
		Op:     "read",
		Net:    "tcp",
		Source: src,
		Addr:   dst,
		Err:    syscall.ECONNRESET,
	}

	require.Contains(t, raw.Error(), "10.0.0.1")
	require.Contains(t, raw.Error(), "52.1.2.3")

	got := sanitizeStreamError(raw)
	require.Equal(t, "connection reset by peer", got)
	require.NotContains(t, got, "10.0.0.1")
	require.NotContains(t, got, "54321")
	require.NotContains(t, got, "52.1.2.3")
	require.NotContains(t, got, "443")
}

func TestSanitizeStreamError_KnownErrors(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"unexpected EOF", io.ErrUnexpectedEOF, "unexpected EOF"},
		{"EOF", io.EOF, "EOF"},
		{"context canceled", context.Canceled, "canceled"},
		{"deadline exceeded", context.DeadlineExceeded, "deadline exceeded"},
		{"ECONNRESET", syscall.ECONNRESET, "connection reset by peer"},
		{"EPIPE", syscall.EPIPE, "broken pipe"},
		{"ETIMEDOUT", syscall.ETIMEDOUT, "connection timed out"},
		{"unknown", errors.New("read tcp 10.0.0.1:123->52.1.2.3:443: weird"), "upstream connection error"},
		{"nil", nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, sanitizeStreamError(tc.err))
		})
	}
}

func TestHandleStreamingResponse_FailoverBodyDoesNotLeakAddresses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newMinimalGatewayService()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	src, _ := net.ResolveTCPAddr("tcp", "10.0.0.1:54321")
	dst, _ := net.ResolveTCPAddr("tcp", "52.1.2.3:443")
	netErr := &net.OpError{
		Op:     "read",
		Net:    "tcp",
		Source: src,
		Addr:   dst,
		Err:    syscall.ECONNRESET,
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(iotest.ErrReader(netErr)),
	}

	_, err := svc.handleStreamingResponse(context.Background(), resp, c, &Account{ID: 1}, time.Now(), "model", "model", false)
	require.Error(t, err)

	var failoverErr *UpstreamFailoverError
	require.True(t, errors.As(err, &failoverErr))
	body := string(failoverErr.ResponseBody)
	require.Contains(t, body, "connection reset by peer")
	require.NotContains(t, body, "10.0.0.1")
	require.NotContains(t, body, "54321")
	require.NotContains(t, body, "52.1.2.3")
	require.NotContains(t, body, "443")
}
