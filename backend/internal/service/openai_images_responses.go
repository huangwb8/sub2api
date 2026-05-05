package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/util/responseheaders"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const openaiImagesOAuthResponsesModel = "gpt-5.4-mini"

type openAIImagesParsedRequest struct {
	Model          string
	Prompt         string
	Stream         bool
	N              int
	Size           string
	Quality        string
	ResponseFormat string
	OutputFormat   string
	Body           []byte
}

func parseOpenAIImagesGenerationRequest(body []byte) (openAIImagesParsedRequest, error) {
	req := openAIImagesParsedRequest{Body: body}
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return req, fmt.Errorf("invalid images request body")
	}
	req.Model = strings.TrimSpace(gjson.GetBytes(body, "model").String())
	if req.Model == "" {
		req.Model = "gpt-image-2"
	}
	if err := validateOpenAIImagesModel(req.Model); err != nil {
		return req, err
	}
	req.Prompt = strings.TrimSpace(gjson.GetBytes(body, "prompt").String())
	req.Stream = gjson.GetBytes(body, "stream").Bool()
	req.N = int(gjson.GetBytes(body, "n").Int())
	if req.N <= 0 {
		req.N = 1
	}
	req.Size = strings.TrimSpace(gjson.GetBytes(body, "size").String())
	req.Quality = strings.TrimSpace(gjson.GetBytes(body, "quality").String())
	req.ResponseFormat = strings.TrimSpace(gjson.GetBytes(body, "response_format").String())
	req.OutputFormat = strings.TrimSpace(gjson.GetBytes(body, "output_format").String())
	return req, nil
}

func validateOpenAIImagesModel(model string) error {
	trimmed := strings.TrimSpace(model)
	if strings.HasPrefix(strings.ToLower(trimmed), "gpt-image-") {
		return nil
	}
	return fmt.Errorf("images endpoint requires an image model, got %q", trimmed)
}

func buildOpenAIImagesResponsesRequest(parsed openAIImagesParsedRequest) ([]byte, error) {
	tool := map[string]any{
		"type":  "image_generation",
		"model": parsed.Model,
	}
	if parsed.Quality != "" {
		tool["quality"] = parsed.Quality
	}
	if parsed.Size != "" {
		tool["size"] = parsed.Size
	}
	if parsed.OutputFormat != "" {
		tool["output_format"] = parsed.OutputFormat
	}
	body := map[string]any{
		"model":       openaiImagesOAuthResponsesModel,
		"stream":      true,
		"store":       false,
		"tool_choice": map[string]any{"type": "image_generation"},
		"tools":       []any{tool},
		"input": []any{
			map[string]any{
				"type": "message",
				"role": "user",
				"content": []any{
					map[string]any{"type": "input_text", "text": parsed.Prompt},
				},
			},
		},
	}
	return json.Marshal(body)
}

func (s *OpenAIGatewayService) buildOpenAIImagesResponsesBridgeRequest(ctxReq *http.Request, account *Account, token string, body []byte) (*http.Request, error) {
	ctx := context.Background()
	if ctxReq != nil {
		ctx = ctxReq.Context()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, chatgptCodexURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Host = "chatgpt.com"
	req.Header.Set("authorization", "Bearer "+token)
	req.Header.Set("content-type", "application/json")
	req.Header.Set("accept", "text/event-stream")
	req.Header.Set("OpenAI-Beta", "responses=experimental")
	req.Header.Set("originator", "codex_cli_rs")
	if account != nil {
		if chatgptAccountID := account.GetChatGPTAccountID(); chatgptAccountID != "" {
			req.Header.Set("chatgpt-account-id", chatgptAccountID)
		}
		if ua := account.GetOpenAIUserAgent(); ua != "" {
			req.Header.Set("user-agent", ua)
		} else {
			req.Header.Set("user-agent", codexCLIUserAgent)
		}
	}
	if ctxReq != nil {
		if v := ctxReq.Header.Get("accept-language"); v != "" {
			req.Header.Set("accept-language", v)
		}
	}
	return req, nil
}

func (s *OpenAIGatewayService) handleImagesResponsesBridge(
	resp *http.Response,
	c *gin.Context,
	parsed openAIImagesParsedRequest,
	requestBytes int64,
	responseBytes int64,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	if parsed.Stream {
		return s.handleImagesResponsesBridgeStreaming(resp, c, parsed, requestBytes, responseBytes, startTime)
	}
	return s.handleImagesResponsesBridgeBuffered(resp, c, parsed, requestBytes, responseBytes, startTime)
}

func (s *OpenAIGatewayService) handleImagesResponsesBridgeBuffered(
	resp *http.Response,
	c *gin.Context,
	parsed openAIImagesParsedRequest,
	requestBytes int64,
	responseBytes int64,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), defaultMaxLineSize)
	usage := OpenAIUsage{}
	images := make([]map[string]any, 0, parsed.N)
	var firstTokenMs *int
	for scanner.Scan() {
		data, ok := extractOpenAISSEDataLine(scanner.Text())
		if !ok || data == "" || data == "[DONE]" {
			continue
		}
		if firstTokenMs == nil {
			ms := int(time.Since(startTime).Milliseconds())
			firstTokenMs = &ms
		}
		eventBytes := []byte(data)
		usage = mergeOpenAIUsage(usage, parseOpenAIImagesUsage(eventBytes))
		appendImagesFromResponsesEvent(&images, eventBytes, parsed.ResponseFormat)
		responseBytes += int64(len(data))
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read responses image bridge stream: %w", err)
	}
	if len(images) == 0 {
		return nil, newUpstreamRequestFailoverError("responses image bridge completed without image_generation result")
	}
	out := map[string]any{
		"created": time.Now().Unix(),
		"model":   parsed.Model,
		"data":    images,
	}
	if usage != (OpenAIUsage{}) {
		out["usage"] = usage
	}
	respBody, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	if s.responseHeaderFilter != nil {
		responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	}
	c.Data(http.StatusOK, "application/json", respBody)
	imageCount := len(images)
	return &OpenAIForwardResult{
		RequestID:          resp.Header.Get("x-request-id"),
		Usage:              usage,
		Model:              parsed.Model,
		BillingModel:       parsed.Model,
		UpstreamModel:      parsed.Model,
		ImageCount:         imageCount,
		ImageSize:          normalizeOpenAIImageSize(parsed.Size),
		Stream:             false,
		Duration:           time.Since(startTime),
		FirstTokenMs:       firstTokenMs,
		ProxyRequestBytes:  requestBytes,
		ProxyResponseBytes: responseBytes + int64(len(respBody)),
		UpstreamEndpoint:   "/v1/responses",
	}, nil
}

func (s *OpenAIGatewayService) handleImagesResponsesBridgeStreaming(
	resp *http.Response,
	c *gin.Context,
	parsed openAIImagesParsedRequest,
	requestBytes int64,
	responseBytes int64,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	flusher, _ := c.Writer.(http.Flusher)
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), defaultMaxLineSize)
	usage := OpenAIUsage{}
	imageCount := 0
	var firstTokenMs *int
	for scanner.Scan() {
		data, ok := extractOpenAISSEDataLine(scanner.Text())
		if !ok || data == "" {
			continue
		}
		if data == "[DONE]" {
			continue
		}
		if firstTokenMs == nil {
			ms := int(time.Since(startTime).Milliseconds())
			firstTokenMs = &ms
		}
		eventBytes := []byte(data)
		usage = mergeOpenAIUsage(usage, parseOpenAIImagesUsage(eventBytes))
		responseBytes += int64(len(data))
		events := imagesResponsesBridgeSSEEvents(eventBytes, parsed.ResponseFormat)
		for _, event := range events {
			if strings.Contains(event, `"type":"image_generation.completed"`) {
				imageCount++
			}
			if _, err := fmt.Fprintf(c.Writer, "data: %s\n\n", event); err != nil {
				return nil, err
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		return nil, fmt.Errorf("read responses image bridge stream: %w", err)
	}
	if imageCount == 0 {
		return nil, newUpstreamRequestFailoverError("responses image bridge completed without image_generation result")
	}
	_, _ = fmt.Fprint(c.Writer, "data: [DONE]\n\n")
	if flusher != nil {
		flusher.Flush()
	}
	return &OpenAIForwardResult{
		RequestID:          resp.Header.Get("x-request-id"),
		Usage:              usage,
		Model:              parsed.Model,
		BillingModel:       parsed.Model,
		UpstreamModel:      parsed.Model,
		ImageCount:         imageCount,
		ImageSize:          normalizeOpenAIImageSize(parsed.Size),
		Stream:             true,
		Duration:           time.Since(startTime),
		FirstTokenMs:       firstTokenMs,
		ProxyRequestBytes:  requestBytes,
		ProxyResponseBytes: responseBytes,
		UpstreamEndpoint:   "/v1/responses",
	}, nil
}

func appendImagesFromResponsesEvent(images *[]map[string]any, event []byte, responseFormat string) {
	for _, item := range extractResponsesImageResults(event) {
		image := map[string]any{}
		if wantsURL(responseFormat) {
			image["url"] = "data:image/png;base64," + item.B64
		} else {
			image["b64_json"] = item.B64
		}
		if item.RevisedPrompt != "" {
			image["revised_prompt"] = item.RevisedPrompt
		}
		*images = append(*images, image)
	}
}

func imagesResponsesBridgeSSEEvents(event []byte, responseFormat string) []string {
	results := extractResponsesImageResults(event)
	if len(results) == 0 {
		if partial := firstGJSONValue(event, "partial_image", "partial_image_b64", "b64_json"); partial != "" {
			payload, _ := json.Marshal(map[string]any{
				"type":          "image_generation.partial_image",
				"partial_image": partial,
			})
			return []string{string(payload)}
		}
		return nil
	}
	out := make([]string, 0, len(results))
	for _, item := range results {
		payload := map[string]any{"type": "image_generation.completed"}
		if wantsURL(responseFormat) {
			payload["url"] = "data:image/png;base64," + item.B64
		} else {
			payload["b64_json"] = item.B64
		}
		if item.RevisedPrompt != "" {
			payload["revised_prompt"] = item.RevisedPrompt
		}
		encoded, _ := json.Marshal(payload)
		out = append(out, string(encoded))
	}
	return out
}

type responsesImageResult struct {
	B64           string
	RevisedPrompt string
}

func extractResponsesImageResults(event []byte) []responsesImageResult {
	if len(event) == 0 || !gjson.ValidBytes(event) {
		return nil
	}
	var out []responsesImageResult
	gjson.GetBytes(event, "response.output").ForEach(func(_, value gjson.Result) bool {
		if strings.TrimSpace(value.Get("type").String()) == "image_generation_call" {
			if b64 := firstGJSONValue([]byte(value.Raw), "result", "b64_json"); b64 != "" {
				out = append(out, responsesImageResult{B64: b64, RevisedPrompt: value.Get("revised_prompt").String()})
			}
		}
		return true
	})
	gjson.GetBytes(event, "output").ForEach(func(_, value gjson.Result) bool {
		if strings.TrimSpace(value.Get("type").String()) == "image_generation_call" {
			if b64 := firstGJSONValue([]byte(value.Raw), "result", "b64_json"); b64 != "" {
				out = append(out, responsesImageResult{B64: b64, RevisedPrompt: value.Get("revised_prompt").String()})
			}
		}
		return true
	})
	if len(out) == 0 && strings.TrimSpace(gjson.GetBytes(event, "type").String()) == "image_generation_call" {
		if b64 := firstGJSONValue(event, "result", "b64_json"); b64 != "" {
			out = append(out, responsesImageResult{B64: b64, RevisedPrompt: gjson.GetBytes(event, "revised_prompt").String()})
		}
	}
	if len(out) == 0 && strings.TrimSpace(gjson.GetBytes(event, "item.type").String()) == "image_generation_call" {
		item := gjson.GetBytes(event, "item")
		if b64 := firstGJSONValue([]byte(item.Raw), "result", "b64_json"); b64 != "" {
			out = append(out, responsesImageResult{B64: b64, RevisedPrompt: item.Get("revised_prompt").String()})
		}
	}
	return out
}

func firstGJSONValue(body []byte, paths ...string) string {
	for _, path := range paths {
		if value := strings.TrimSpace(gjson.GetBytes(body, path).String()); value != "" {
			return value
		}
	}
	return ""
}

func wantsURL(responseFormat string) bool {
	return strings.EqualFold(strings.TrimSpace(responseFormat), "url")
}
