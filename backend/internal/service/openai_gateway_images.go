package service

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/util/responseheaders"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

const (
	openaiImagesGenerationsEndpoint = "/v1/images/generations"
	openaiImagesEditsEndpoint       = "/v1/images/edits"
)

// ForwardAsImageGeneration forwards an OpenAI Images generation request to an
// OpenAI API-key upstream and writes the upstream response back to the client.
func (s *OpenAIGatewayService) ForwardAsImageGeneration(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	originalModel string,
	billingModel string,
	upstreamModel string,
) (*OpenAIForwardResult, error) {
	startTime := time.Now()
	if account == nil || account.Type != AccountTypeAPIKey {
		return nil, errors.New("images API requires an OpenAI API key account")
	}
	requestModel := strings.TrimSpace(gjson.GetBytes(body, "model").String())
	if requestModel == "" {
		requestModel = originalModel
	}
	mappedModel := account.GetMappedModel(requestModel)
	if mappedModel == "" {
		mappedModel = requestModel
	}
	if mappedModel != requestModel {
		body = s.ReplaceModelInBody(body, mappedModel)
	}
	billingModel = mappedModel
	upstreamModel = mappedModel

	token, _, err := s.GetAccessToken(ctx, account)
	if err != nil {
		return nil, fmt.Errorf("get access token: %w", err)
	}

	reqStream := gjson.GetBytes(body, "stream").Bool()
	upstreamReq, err := s.buildOpenAIImagesRequest(ctx, c, account, openaiImagesGenerationsEndpoint, bytes.NewReader(body), token, "application/json")
	if err != nil {
		return nil, fmt.Errorf("build images request: %w", err)
	}

	return s.forwardOpenAIImagesRequest(ctx, c, account, upstreamReq, body, originalModel, billingModel, upstreamModel, reqStream, startTime)
}

// ForwardAsImageEdit rebuilds the inbound multipart/form-data request and
// forwards it to the OpenAI Images edits endpoint.
func (s *OpenAIGatewayService) ForwardAsImageEdit(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	originalModel string,
	billingModel string,
	upstreamModel string,
) (*OpenAIForwardResult, error) {
	startTime := time.Now()
	if account == nil || account.Type != AccountTypeAPIKey {
		return nil, errors.New("images API requires an OpenAI API key account")
	}
	if c.Request.MultipartForm == nil {
		if err := c.Request.ParseMultipartForm(64 << 20); err != nil {
			return nil, fmt.Errorf("parse multipart form: %w", err)
		}
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := copyMultipartForm(writer, c.Request.MultipartForm); err != nil {
		_ = writer.Close()
		return nil, err
	}
	requestModel := strings.TrimSpace(upstreamModel)
	if requestModel == "" {
		requestModel = originalModel
	}
	mappedModel := account.GetMappedModel(requestModel)
	if mappedModel == "" {
		mappedModel = requestModel
	}
	billingModel = mappedModel
	upstreamModel = mappedModel
	if err := overwriteMultipartField(writer, "model", mappedModel); err != nil {
		_ = writer.Close()
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	token, _, err := s.GetAccessToken(ctx, account)
	if err != nil {
		return nil, fmt.Errorf("get access token: %w", err)
	}
	upstreamReq, err := s.buildOpenAIImagesRequest(ctx, c, account, openaiImagesEditsEndpoint, bytes.NewReader(body.Bytes()), token, writer.FormDataContentType())
	if err != nil {
		return nil, fmt.Errorf("build image edit request: %w", err)
	}

	reqStream := strings.EqualFold(strings.TrimSpace(c.Request.FormValue("stream")), "true")
	return s.forwardOpenAIImagesRequest(ctx, c, account, upstreamReq, body.Bytes(), originalModel, billingModel, upstreamModel, reqStream, startTime)
}

func copyMultipartForm(writer *multipart.Writer, form *multipart.Form) error {
	if form == nil {
		return nil
	}
	for key, values := range form.Value {
		if key == "model" {
			continue
		}
		for _, value := range values {
			if err := writer.WriteField(key, value); err != nil {
				return fmt.Errorf("write multipart field %q: %w", key, err)
			}
		}
	}
	for key, files := range form.File {
		for _, fileHeader := range files {
			if fileHeader == nil {
				continue
			}
			src, err := fileHeader.Open()
			if err != nil {
				return fmt.Errorf("open multipart file %q: %w", key, err)
			}
			part, err := writer.CreateFormFile(key, fileHeader.Filename)
			if err != nil {
				_ = src.Close()
				return fmt.Errorf("create multipart file %q: %w", key, err)
			}
			if _, err := io.Copy(part, src); err != nil {
				_ = src.Close()
				return fmt.Errorf("copy multipart file %q: %w", key, err)
			}
			_ = src.Close()
		}
	}
	return nil
}

func overwriteMultipartField(writer *multipart.Writer, key, value string) error {
	if strings.TrimSpace(key) == "" {
		return nil
	}
	return writer.WriteField(key, value)
}

func (s *OpenAIGatewayService) buildOpenAIImagesRequest(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	endpoint string,
	body io.Reader,
	token string,
	contentType string,
) (*http.Request, error) {
	targetURL, err := s.buildOpenAIImagesURL(account, endpoint)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("authorization", "Bearer "+token)
	if contentType != "" {
		req.Header.Set("content-type", contentType)
	}
	for key, values := range c.Request.Header {
		lowerKey := strings.ToLower(key)
		switch lowerKey {
		case "accept", "accept-language", "openai-beta", "user-agent":
			for _, v := range values {
				req.Header.Add(key, v)
			}
		}
	}
	if contentType != "" {
		req.Header.Set("content-type", contentType)
	}
	if customUA := account.GetOpenAIUserAgent(); customUA != "" {
		req.Header.Set("user-agent", customUA)
	}
	return req, nil
}

func (s *OpenAIGatewayService) buildOpenAIImagesURL(account *Account, endpoint string) (string, error) {
	baseURL := "https://api.openai.com"
	if account != nil {
		baseURL = account.GetOpenAIBaseURL()
	}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://api.openai.com"
	}
	validatedURL, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return "", err
	}
	return buildOpenAIEndpointURL(validatedURL, endpoint), nil
}

func buildOpenAIEndpointURL(base, endpoint string) string {
	normalized := strings.TrimRight(strings.TrimSpace(base), "/")
	cleanEndpoint := "/" + strings.TrimLeft(strings.TrimSpace(endpoint), "/")
	if normalized == "" {
		return cleanEndpoint
	}
	if strings.HasSuffix(normalized, cleanEndpoint) {
		return normalized
	}
	customEndpoint := strings.TrimPrefix(cleanEndpoint, "/v1")
	if customEndpoint != cleanEndpoint && strings.HasSuffix(normalized, customEndpoint) {
		return normalized
	}
	if parsed, err := url.Parse(normalized); err == nil && strings.EqualFold(parsed.Hostname(), "api.openai.com") && (parsed.Path == "" || parsed.Path == "/") {
		return normalized + cleanEndpoint
	}
	if strings.HasSuffix(normalized, "/v1") {
		return normalized + customEndpoint
	}
	if strings.Contains(path.Clean(cleanEndpoint), "/v1/") {
		return normalized + customEndpoint
	}
	return normalized + cleanEndpoint
}

func (s *OpenAIGatewayService) forwardOpenAIImagesRequest(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	upstreamReq *http.Request,
	requestBody []byte,
	originalModel string,
	billingModel string,
	upstreamModel string,
	reqStream bool,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	proxyURL := ""
	if account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	resp, err := s.httpUpstream.Do(upstreamReq, proxyURL, account.ID, account.Concurrency)
	if err != nil {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		safeErr := sanitizeUpstreamErrorMessage(err.Error())
		setOpsUpstreamError(c, 0, safeErr, "")
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			Platform:  account.Platform,
			AccountID: account.ID,
			Kind:      "request_error",
			Message:   safeErr,
		})
		return nil, newUpstreamRequestFailoverError(safeErr)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		_ = resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(respBody))
		upstreamMsg := sanitizeUpstreamErrorMessage(strings.TrimSpace(extractUpstreamErrorMessage(respBody)))
		if s.shouldFailoverOpenAIUpstreamResponse(resp.StatusCode, upstreamMsg, respBody) {
			if s.rateLimitService != nil {
				s.rateLimitService.HandleUpstreamError(ctx, account, resp.StatusCode, resp.Header, respBody)
			}
			appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
				Platform:           account.Platform,
				AccountID:          account.ID,
				AccountName:        account.Name,
				UpstreamStatusCode: resp.StatusCode,
				UpstreamRequestID:  resp.Header.Get("x-request-id"),
				Kind:               "failover",
				Message:            upstreamMsg,
			})
			return nil, &UpstreamFailoverError{
				StatusCode:             resp.StatusCode,
				ResponseBody:           respBody,
				RetryableOnSameAccount: account.IsPoolMode() && (isPoolModeRetryableStatus(resp.StatusCode) || isOpenAITransientProcessingError(resp.StatusCode, upstreamMsg, respBody)),
			}
		}
		return s.handleCompatErrorResponse(resp, c, account, writeImagesError)
	}

	if reqStream {
		return s.handleImagesStreamingResponse(resp, c, originalModel, billingModel, upstreamModel, startTime)
	}
	return s.handleImagesBufferedResponse(resp, c, requestBody, originalModel, billingModel, upstreamModel, startTime)
}

func (s *OpenAIGatewayService) handleImagesBufferedResponse(
	resp *http.Response,
	c *gin.Context,
	requestBody []byte,
	originalModel string,
	billingModel string,
	upstreamModel string,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read images response: %w", err)
	}
	if s.responseHeaderFilter != nil {
		responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	}
	contentType := resp.Header.Get("content-type")
	if contentType == "" {
		contentType = "application/json"
	}
	c.Data(resp.StatusCode, contentType, respBody)

	return &OpenAIForwardResult{
		RequestID:     resp.Header.Get("x-request-id"),
		Usage:         parseOpenAIImagesUsage(respBody),
		Model:         originalModel,
		BillingModel:  billingModel,
		UpstreamModel: upstreamModel,
		Stream:        false,
		Duration:      time.Since(startTime),
	}, nil
}

func (s *OpenAIGatewayService) handleImagesStreamingResponse(
	resp *http.Response,
	c *gin.Context,
	originalModel string,
	billingModel string,
	upstreamModel string,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	if s.responseHeaderFilter != nil {
		responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	if v := resp.Header.Get("x-request-id"); v != "" {
		c.Header("x-request-id", v)
	}
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return nil, errors.New("streaming not supported")
	}

	scanner := bufio.NewScanner(resp.Body)
	maxLineSize := defaultMaxLineSize
	if s.cfg != nil && s.cfg.Gateway.MaxLineSize > 0 {
		maxLineSize = s.cfg.Gateway.MaxLineSize
	}
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)
	usage := OpenAIUsage{}
	var firstTokenMs *int
	for scanner.Scan() {
		line := scanner.Text()
		if data, ok := extractOpenAISSEDataLine(line); ok {
			if data != "" && data != "[DONE]" {
				usage = mergeOpenAIUsage(usage, parseOpenAIImagesUsage([]byte(data)))
				if firstTokenMs == nil {
					ms := int(time.Since(startTime).Milliseconds())
					firstTokenMs = &ms
				}
			}
		}
		if _, err := fmt.Fprintln(c.Writer, line); err != nil {
			logger.L().Debug("openai images stream: client write failed", zap.Error(err))
			break
		}
		if line == "" {
			flusher.Flush()
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read images stream: %w", err)
	}
	flusher.Flush()
	return &OpenAIForwardResult{
		RequestID:     resp.Header.Get("x-request-id"),
		Usage:         usage,
		Model:         originalModel,
		BillingModel:  billingModel,
		UpstreamModel: upstreamModel,
		Stream:        true,
		Duration:      time.Since(startTime),
		FirstTokenMs:  firstTokenMs,
	}, nil
}

func parseOpenAIImagesUsage(body []byte) OpenAIUsage {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return OpenAIUsage{}
	}
	usage := gjson.GetBytes(body, "usage")
	if !usage.Exists() {
		return OpenAIUsage{}
	}
	inputTokens := int(usage.Get("input_tokens").Int())
	outputTokens := int(usage.Get("output_tokens").Int())
	if outputTokens == 0 {
		totalTokens := int(usage.Get("total_tokens").Int())
		if totalTokens > inputTokens {
			outputTokens = totalTokens - inputTokens
		}
	}
	imageInputTokens := int(usage.Get("input_tokens_details.image_tokens").Int())
	imageOutputTokens := int(usage.Get("output_tokens_details.image_tokens").Int())
	if imageOutputTokens == 0 {
		imageOutputTokens = outputTokens
	}
	return OpenAIUsage{
		InputTokens:       inputTokens,
		OutputTokens:      outputTokens,
		ImageInputTokens:  imageInputTokens,
		ImageOutputTokens: imageOutputTokens,
	}
}

func mergeOpenAIUsage(dst, src OpenAIUsage) OpenAIUsage {
	if src.InputTokens > 0 {
		dst.InputTokens = src.InputTokens
	}
	if src.OutputTokens > 0 {
		dst.OutputTokens = src.OutputTokens
	}
	if src.CacheCreationInputTokens > 0 {
		dst.CacheCreationInputTokens = src.CacheCreationInputTokens
	}
	if src.CacheReadInputTokens > 0 {
		dst.CacheReadInputTokens = src.CacheReadInputTokens
	}
	if src.ImageInputTokens > 0 {
		dst.ImageInputTokens = src.ImageInputTokens
	}
	if src.ImageOutputTokens > 0 {
		dst.ImageOutputTokens = src.ImageOutputTokens
	}
	return dst
}

func writeImagesError(c *gin.Context, status int, errType, message string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"type":    errType,
			"message": message,
		},
	})
}
