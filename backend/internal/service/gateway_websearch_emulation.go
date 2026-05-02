package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/websearch"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
)

const (
	defaultWebSearchMaxResults = 5
	defaultWebSearchModel      = "claude-sonnet-4-6"
	webSearchMsgIDPrefix       = "msg_ws_"
	webSearchToolUseIDPrefix   = "srvtoolu_ws_"
	tokenEstimateDivisor       = 4
)

var webSearchManagerPtr atomic.Pointer[websearch.Manager]

func SetWebSearchManager(m *websearch.Manager) {
	webSearchManagerPtr.Store(m)
}

func getWebSearchManager() *websearch.Manager {
	return webSearchManagerPtr.Load()
}

func (s *GatewayService) shouldEmulateWebSearch(ctx context.Context, account *Account, body []byte) bool {
	if account == nil || account.Platform != PlatformAnthropic || account.Type != AccountTypeAPIKey {
		return false
	}
	if getWebSearchManager() == nil {
		return false
	}
	if !isOnlyWebSearchToolInBody(body) {
		return false
	}
	if !s.settingService.IsWebSearchEmulationEnabled(ctx) {
		return false
	}

	switch account.GetWebSearchEmulationMode() {
	case WebSearchModeDisabled:
		return false
	case WebSearchModeEnabled, WebSearchModeDefault:
		return true
	default:
		return false
	}
}

func isOnlyWebSearchToolInBody(body []byte) bool {
	tools := gjson.GetBytes(body, "tools")
	if !tools.IsArray() {
		return false
	}
	arr := tools.Array()
	if len(arr) != 1 {
		return false
	}
	return isWebSearchToolJSON(arr[0])
}

func isWebSearchToolJSON(tool gjson.Result) bool {
	toolType := tool.Get("type").String()
	if strings.HasPrefix(toolType, "web_search") || toolType == "google_search" {
		return true
	}
	switch tool.Get("name").String() {
	case "web_search", "google_search", "web_search_20250305":
		return true
	}
	return false
}

func extractSearchQueryFromBody(body []byte) string {
	messages := gjson.GetBytes(body, "messages")
	if !messages.IsArray() {
		return ""
	}
	arr := messages.Array()
	if len(arr) == 0 {
		return ""
	}
	lastMsg := arr[len(arr)-1]
	if lastMsg.Get("role").String() != "user" {
		return ""
	}
	return extractWebSearchTextFromContent(lastMsg.Get("content"))
}

func extractWebSearchTextFromContent(content gjson.Result) string {
	if content.Type == gjson.String {
		return content.String()
	}
	if content.IsArray() {
		for _, block := range content.Array() {
			if block.Get("type").String() == "text" {
				if text := block.Get("text").String(); text != "" {
					return text
				}
			}
		}
	}
	return ""
}

func (s *GatewayService) handleWebSearchEmulation(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	parsed *ParsedRequest,
) (*ForwardResult, error) {
	startTime := time.Now()
	if parsed.OnUpstreamAccepted != nil {
		parsed.OnUpstreamAccepted()
	}

	query := extractSearchQueryFromBody(parsed.Body)
	if query == "" {
		return nil, fmt.Errorf("web search emulation: no query found in messages")
	}

	resp, err := doWebSearch(ctx, account, query)
	if err != nil {
		if errors.Is(err, websearch.ErrProxyUnavailable) {
			return nil, &UpstreamFailoverError{
				StatusCode:   http.StatusBadGateway,
				ResponseBody: []byte(err.Error()),
			}
		}
		return nil, err
	}

	model := parsed.Model
	if model == "" {
		model = defaultWebSearchModel
	}
	if parsed.Stream {
		return writeWebSearchStreamResponse(c, query, resp, model, startTime)
	}
	return writeWebSearchNonStreamResponse(c, query, resp, model, startTime)
}

func doWebSearch(ctx context.Context, account *Account, query string) (*websearch.SearchResponse, error) {
	proxyURL := resolveAccountProxyURL(account)
	mgr := getWebSearchManager()
	if mgr == nil {
		return nil, fmt.Errorf("web search emulation: manager not initialized")
	}
	resp, _, err := mgr.SearchWithBestProvider(ctx, websearch.SearchRequest{
		Query: query, MaxResults: defaultWebSearchMaxResults, ProxyURL: proxyURL,
	})
	if err != nil {
		return nil, fmt.Errorf("web search emulation: %w", err)
	}
	return resp, nil
}

func resolveAccountProxyURL(account *Account) string {
	if account != nil && account.ProxyID != nil && account.Proxy != nil {
		return account.Proxy.URL()
	}
	return ""
}

func writeWebSearchStreamResponse(c *gin.Context, query string, resp *websearch.SearchResponse, model string, startTime time.Time) (*ForwardResult, error) {
	msgID := webSearchMsgIDPrefix + uuid.New().String()
	toolUseID := webSearchToolUseIDPrefix + uuid.New().String()[:16]
	textSummary := buildTextSummary(query, resp.Results)

	setSSEHeaders(c)
	writer := c.Writer
	for _, fn := range []func() error{
		func() error { return writeSSEMessageStart(writer, msgID, model) },
		func() error { return writeSSEServerToolUse(writer, toolUseID, query, 0) },
		func() error { return writeSSEToolResult(writer, toolUseID, resp.Results, 1) },
		func() error { return writeSSETextBlock(writer, textSummary, 2) },
		func() error { return writeSSEMessageEnd(writer, len(textSummary)/tokenEstimateDivisor) },
	} {
		if err := fn(); err != nil {
			break
		}
	}
	writer.Flush()
	return &ForwardResult{Model: model, Duration: time.Since(startTime), Usage: ClaudeUsage{}}, nil
}

func setSSEHeaders(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)
}

func writeSSEMessageStart(w http.ResponseWriter, msgID, model string) error {
	evt := map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id": msgID, "type": "message", "role": "assistant", "model": model,
			"content": []any{}, "stop_reason": nil, "stop_sequence": nil,
			"usage": map[string]int{"input_tokens": 0, "output_tokens": 0},
		},
	}
	return flushSSEJSON(w, "message_start", evt)
}

func writeSSEServerToolUse(w http.ResponseWriter, toolUseID, query string, index int) error {
	start := map[string]any{
		"type": "content_block_start", "index": index,
		"content_block": map[string]any{
			"type": "server_tool_use", "id": toolUseID, "name": "web_search", "input": map[string]any{"query": query},
		},
	}
	if err := flushSSEJSON(w, "content_block_start", start); err != nil {
		return err
	}
	stop := map[string]any{"type": "content_block_stop", "index": index}
	return flushSSEJSON(w, "content_block_stop", stop)
}

func writeSSEToolResult(w http.ResponseWriter, toolUseID string, results []websearch.SearchResult, index int) error {
	start := map[string]any{
		"type": "content_block_start", "index": index,
		"content_block": map[string]any{
			"type": "web_search_tool_result", "tool_use_id": toolUseID, "content": results,
		},
	}
	if err := flushSSEJSON(w, "content_block_start", start); err != nil {
		return err
	}
	stop := map[string]any{"type": "content_block_stop", "index": index}
	return flushSSEJSON(w, "content_block_stop", stop)
}

func writeSSETextBlock(w http.ResponseWriter, text string, index int) error {
	start := map[string]any{
		"type": "content_block_start", "index": index,
		"content_block": map[string]any{"type": "text", "text": ""},
	}
	if err := flushSSEJSON(w, "content_block_start", start); err != nil {
		return err
	}
	delta := map[string]any{
		"type": "content_block_delta", "index": index,
		"delta": map[string]any{"type": "text_delta", "text": text},
	}
	if err := flushSSEJSON(w, "content_block_delta", delta); err != nil {
		return err
	}
	stop := map[string]any{"type": "content_block_stop", "index": index}
	return flushSSEJSON(w, "content_block_stop", stop)
}

func writeSSEMessageEnd(w http.ResponseWriter, outputTokens int) error {
	delta := map[string]any{
		"type":  "message_delta",
		"delta": map[string]any{"stop_reason": "tool_use", "stop_sequence": nil},
		"usage": map[string]int{"output_tokens": outputTokens},
	}
	if err := flushSSEJSON(w, "message_delta", delta); err != nil {
		return err
	}
	return flushSSEJSON(w, "message_stop", map[string]any{"type": "message_stop"})
}

func flushSSEJSON(w http.ResponseWriter, event string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, body); err != nil {
		return err
	}
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	return nil
}

func writeWebSearchNonStreamResponse(c *gin.Context, query string, resp *websearch.SearchResponse, model string, startTime time.Time) (*ForwardResult, error) {
	msgID := webSearchMsgIDPrefix + uuid.New().String()
	toolUseID := webSearchToolUseIDPrefix + uuid.New().String()[:16]
	textSummary := buildTextSummary(query, resp.Results)

	payload := map[string]any{
		"id":    msgID,
		"type":  "message",
		"role":  "assistant",
		"model": model,
		"content": []map[string]any{
			{
				"type":  "server_tool_use",
				"id":    toolUseID,
				"name":  "web_search",
				"input": map[string]any{"query": query},
			},
			{
				"type":        "web_search_tool_result",
				"tool_use_id": toolUseID,
				"content":     resp.Results,
			},
			{
				"type": "text",
				"text": textSummary,
			},
		},
		"stop_reason":   "tool_use",
		"stop_sequence": nil,
		"usage":         map[string]int{"input_tokens": 0, "output_tokens": len(textSummary) / tokenEstimateDivisor},
	}
	c.JSON(http.StatusOK, payload)
	return &ForwardResult{Model: model, Duration: time.Since(startTime), Usage: ClaudeUsage{}}, nil
}

func buildTextSummary(query string, results []websearch.SearchResult) string {
	if len(results) == 0 {
		return "未检索到可用的网页结果。"
	}
	var sb strings.Builder
	_, _ = sb.WriteString("以下是关于“")
	_, _ = sb.WriteString(query)
	_, _ = sb.WriteString("”的网页搜索结果摘要：\n")
	for i, result := range results {
		_, _ = sb.WriteString(fmt.Sprintf("%d. %s\n%s\n%s\n", i+1, strings.TrimSpace(result.Title), strings.TrimSpace(result.Snippet), strings.TrimSpace(result.URL)))
	}
	return strings.TrimSpace(sb.String())
}
