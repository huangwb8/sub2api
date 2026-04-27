# Upstream c056db to c92b88 Compatibility Fixes Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 吸收上游 `Wei-Shaw/sub2api` 在 `(c056db740d56ce008292a7b414c804cc6f308208, c92b88e34abd1b032aa5307d760b8bc0aad49b28]` 区间内对 OpenAI/Anthropic 兼容性的必要修复，同时不改变本项目已有路由与调度行为。

**Architecture:** 本次上游变更分为两条独立修复线：Responses 转 Anthropic 时清理 Claude Code `Read.pages` 空字符串参数，以及 OpenAI Images 请求只用显式会话信号做粘性会话。当前项目已具备 `responses_to_anthropic` 兼容层，建议直接吸收第一条；当前项目未发现上游新增的 `backend/internal/handler/openai_images.go` 图片入口，因此第二条先做路径确认，只有存在或准备引入 `/v1/images/*` OpenAI 图片入口时再落地。

**Tech Stack:** Go 1.26+、Gin、Responses API 兼容层、OpenAI 账号调度与粘性会话、`go test`。

**Minimal Change Scope:** 允许修改 `backend/internal/pkg/apicompat/responses_to_anthropic.go`、`backend/internal/pkg/apicompat/anthropic_responses_test.go`、`backend/internal/service/openai_gateway_service.go`、`backend/internal/service/openai_gateway_service_test.go`，以及在确认存在 OpenAI Images handler 后修改对应 handler 文件。避免引入新接口、迁移、前端改动、调度策略重构和无关格式化。

**Success Criteria:** Claude Code `Read` 工具调用中 `pages:""` 不再透传给 Anthropic；其它工具的空字符串参数仍保持原样；OpenAI 普通对话/Responses 的内容派生粘性会话保持不变；OpenAI Images 在无 `session_id`、`conversation_id`、`prompt_cache_key` 时不会因请求内容被隐式绑定到固定账号；license 与上游区间一致。

**Verification Plan:** 运行 `cd backend && go test -tags=unit ./internal/pkg/apicompat ./internal/service`；若实现 OpenAI Images handler 适配，再补充对应 handler/service 定向测试；执行 `git diff --check`；确认 `git diff c056db740d56ce008292a7b414c804cc6f308208..c92b88e34abd1b032aa5307d760b8bc0aad49b28 -- LICENSE*` 为空。

---

## 上游变更摘要

对比范围：`c056db740d56ce008292a7b414c804cc6f308208..c92b88e34abd1b032aa5307d760b8bc0aad49b28`。

| Commit | 类型 | 影响 |
|--------|------|------|
| `30220903` | `fix(anthropic)` | Responses 转 Anthropic 时，对 Claude Code `Read` 工具的 `pages:""` 做清理，避免 Anthropic 收到不合法的空页码参数。 |
| `c92b88e3` | merge | 合并 `fix/claude-code-read-empty-pages`。 |
| `615557ec` | `fix(openai)` | 新增显式会话 hash 方法，让 OpenAI Images 请求不再使用内容派生 fallback 产生隐式粘性会话。 |
| `ed0c85a1` | merge | 合并 `pr/openai-images-explicit-session`。 |

文件变化共 5 个：`backend/internal/handler/openai_images.go`、`backend/internal/pkg/apicompat/responses_to_anthropic.go`、`backend/internal/pkg/apicompat/anthropic_responses_test.go`、`backend/internal/service/openai_gateway_service.go`、`backend/internal/service/openai_gateway_service_test.go`。

license 检查结果：`LICENSE` 在两个版本中的 SHA-256 均为 `a5681bf9b05db14d86776930017c647ad9e6e56ff6bbcfdf21e5848288dfaf1b`，本轮无需同步许可证。

## 对本项目的启发与吸收判断

### P0：吸收 Claude Code `Read.pages` 空字符串修复

当前项目存在 `backend/internal/pkg/apicompat/responses_to_anthropic.go`，且 `function_call` 仍直接使用 `json.RawMessage(item.Arguments)`。这意味着当上游 Responses 返回：

```json
{"file_path":"/tmp/demo.py","limit":2000,"offset":0,"pages":""}
```

Anthropic 兼容输出也会保留 `pages:""`。对 Claude Code 的 `Read` 工具来说，空字符串页码没有语义，且可能触发工具参数校验失败。上游修复的关键点是只针对 `Read` 工具删除 `pages:""`，不影响其它工具的空字符串字段。

这条修复建议直接吸收，风险低、收益明确。

### P1：条件吸收 OpenAI Images 显式会话修复

上游新增 `GenerateExplicitSessionHash(c, body)`，只从 `session_id`、`conversation_id`、`prompt_cache_key` 这些显式信号派生会话，不再对图片请求使用内容派生 fallback。

原因是图片生成/编辑是偏无状态请求。若把 prompt 内容作为会话 fallback，相同或相似 prompt 容易被绑定到固定账号，削弱负载均衡，也可能让单账号承压。

当前项目未发现 `backend/internal/handler/openai_images.go`、`ParseOpenAIImagesRequest`、`ForwardImages`、`SelectAccountWithSchedulerForImages` 等上游图片入口/服务方法。因此这条不应机械照搬 handler 改动。建议先把 `GenerateExplicitSessionHash` 作为小型服务能力引入并补测试；只有确认当前项目已有其它 OpenAI 图片路由，或后续准备吸收上游 `/v1/images/generations`、`/v1/images/edits` 能力时，再把图片 handler 的 `GenerateSessionHash` 切换为 `GenerateExplicitSessionHash`。

## Task 1: 清理 Responses 转 Anthropic 的 `Read.pages` 空字符串

**Files:**
- Modify: `backend/internal/pkg/apicompat/responses_to_anthropic.go`
- Test: `backend/internal/pkg/apicompat/anthropic_responses_test.go`

**Step 1: 写失败测试**

在 `TestResponsesToAnthropic_ToolUse` 附近新增两个非流式测试：

```go
func TestResponsesToAnthropic_ReadToolDropsEmptyPages(t *testing.T) {
	resp := &ResponsesResponse{
		ID:     "resp_read",
		Model:  "gpt-5.5",
		Status: "completed",
		Output: []ResponsesOutput{{
			Type:      "function_call",
			CallID:    "call_read",
			Name:      "Read",
			Arguments: `{"file_path":"/tmp/demo.py","limit":2000,"offset":0,"pages":""}`,
		}},
	}

	anth := ResponsesToAnthropic(resp, "claude-opus-4-6")
	require.Len(t, anth.Content, 1)
	assert.JSONEq(t, `{"file_path":"/tmp/demo.py","limit":2000,"offset":0}`, string(anth.Content[0].Input))
}

func TestResponsesToAnthropic_PreservesEmptyStringsForOtherTools(t *testing.T) {
	resp := &ResponsesResponse{
		ID:     "resp_other",
		Model:  "gpt-5.5",
		Status: "completed",
		Output: []ResponsesOutput{{
			Type:      "function_call",
			CallID:    "call_other",
			Name:      "Search",
			Arguments: `{"query":""}`,
		}},
	}

	anth := ResponsesToAnthropic(resp, "claude-opus-4-6")
	require.Len(t, anth.Content, 1)
	assert.JSONEq(t, `{"query":""}`, string(anth.Content[0].Input))
}
```

再新增流式测试，覆盖 `response.function_call_arguments.delta` 与 `response.function_call_arguments.done`：

```go
func TestStreamingReadToolDropsEmptyPages(t *testing.T) {
	state := NewResponsesEventToAnthropicState()
	ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:     "response.created",
		Response: &ResponsesResponse{ID: "resp_read_stream", Model: "gpt-5.5"},
	}, state)

	events := ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:        "response.output_item.added",
		OutputIndex: 0,
		Item:        &ResponsesOutput{Type: "function_call", CallID: "call_read", Name: "Read"},
	}, state)
	require.Len(t, events, 1)

	events = ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:        "response.function_call_arguments.delta",
		OutputIndex: 0,
		Delta:       `{"file_path":"/tmp/demo.py","limit":2000,"offset":0,"pages":""}`,
	}, state)
	assert.Len(t, events, 0)

	events = ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type:        "response.function_call_arguments.done",
		OutputIndex: 0,
		Arguments:   `{"file_path":"/tmp/demo.py","limit":2000,"offset":0,"pages":""}`,
	}, state)
	require.Len(t, events, 2)
	assert.JSONEq(t, `{"file_path":"/tmp/demo.py","limit":2000,"offset":0}`, events[0].Delta.PartialJSON)
}
```

**Step 2: 运行测试确认失败**

Run:

```bash
cd backend && go test -tags=unit ./internal/pkg/apicompat -run 'TestResponsesToAnthropic_ReadToolDropsEmptyPages|TestResponsesToAnthropic_PreservesEmptyStringsForOtherTools|TestStreamingReadToolDropsEmptyPages'
```

Expected: `Read.pages` 测试失败，证明当前实现仍透传空字符串。

**Step 3: 实现最小修复**

在 `responses_to_anthropic.go` 中新增 helper：

```go
func sanitizeAnthropicToolUseInput(name string, raw string) json.RawMessage {
	if name != "Read" || raw == "" {
		return json.RawMessage(raw)
	}
	var input map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &input); err != nil {
		return json.RawMessage(raw)
	}
	if pages, ok := input["pages"]; !ok || string(pages) != `""` {
		return json.RawMessage(raw)
	}
	delete(input, "pages")
	sanitized, err := json.Marshal(input)
	if err != nil {
		return json.RawMessage(raw)
	}
	return sanitized
}
```

非流式 `function_call` 的 `Input` 改为调用该 helper。流式状态中记录当前工具名与累计参数；对 `Read` 工具的参数 delta 先缓存，直到 done 时输出一次清理后的完整 `input_json_delta`，再关闭 block。其它工具保持逐 delta 透传，避免改变现有流式行为。

**Step 4: 运行定向测试**

Run:

```bash
cd backend && go test -tags=unit ./internal/pkg/apicompat
```

Expected: apicompat 测试全部通过。

## Task 2: 增加显式 OpenAI 会话 hash 能力

**Files:**
- Modify: `backend/internal/service/openai_gateway_service.go`
- Test: `backend/internal/service/openai_gateway_service_test.go`

**Step 1: 写失败测试**

新增 `TestOpenAIGatewayService_GenerateExplicitSessionHash_SkipsContentFallback`，覆盖三点：

- 图片请求体只有 `model` 与 `prompt` 时返回空 hash，且不写 legacy hash。
- 请求体有 `prompt_cache_key` 时返回 `xxhash` hash，并写入 legacy hash。
- header `session_id` 优先于 body `prompt_cache_key`。

**Step 2: 抽出显式会话读取 helper**

把 `GenerateSessionHash` 与 `ExtractSessionID` 中重复的显式信号读取逻辑收敛为私有函数：

```go
func explicitOpenAISessionID(c *gin.Context, body []byte) string
```

读取顺序保持为 `session_id`、`conversation_id`、`prompt_cache_key`。

**Step 3: 新增 `GenerateExplicitSessionHash`**

实现：

```go
func (s *OpenAIGatewayService) GenerateExplicitSessionHash(c *gin.Context, body []byte) string
```

该方法只使用 `explicitOpenAISessionID`，不调用 `deriveOpenAIContentSessionSeed`。`GenerateSessionHash` 继续保留内容 fallback，确保聊天、Responses、WS ingress 的既有粘性行为不被破坏。

**Step 4: 运行 service 定向测试**

Run:

```bash
cd backend && go test -tags=unit ./internal/service -run 'TestOpenAIGatewayService_GenerateSessionHash|TestOpenAIGatewayService_GenerateExplicitSessionHash'
```

Expected: 会话 hash 相关测试全部通过。

## Task 3: 映射 OpenAI Images 入口并决定是否接线

**Files:**
- Inspect: `backend/internal/handler/openai_gateway_handler.go`
- Inspect: `backend/internal/server`
- Conditional modify: 当前项目实际承载 `/v1/images/generations` 或 `/v1/images/edits` 的 handler 文件

**Step 1: 确认当前项目是否支持 OpenAI Images API**

Run:

```bash
rg -n 'images/generations|images/edits|ParseOpenAIImagesRequest|ForwardImages|SelectAccountWithSchedulerForImages|GenerateSessionHash\\(c, body\\)' backend/internal
```

Expected:

- 若没有 OpenAI Images handler：记录为 N/A，本任务不改 handler。
- 若存在 handler 且使用 `GenerateSessionHash(c, body)`：继续 Step 2。

**Step 2: 只在图片 handler 中切换 hash 生成**

将图片 handler 的：

```go
sessionHash := h.gatewayService.GenerateSessionHash(c, body)
```

改为：

```go
sessionHash := h.gatewayService.GenerateExplicitSessionHash(c, body)
```

不要改聊天、Responses、Anthropic messages 或 WS ingress 的 hash 逻辑。

**Step 3: 补充路由级行为测试**

若项目已有图片 handler，新增或扩展对应 handler/service 测试，验证无显式会话的图片请求选择账号时 `sessionHash == ""`；带 `session_id` 或 `prompt_cache_key` 时仍使用稳定 hash。

## Task 4: 回归验证与 license 复核

**Files:**
- No source change expected

**Step 1: 运行定向测试**

Run:

```bash
cd backend && go test -tags=unit ./internal/pkg/apicompat ./internal/service
```

Expected: 全部通过。

**Step 2: 格式和差异检查**

Run:

```bash
gofmt -w backend/internal/pkg/apicompat/responses_to_anthropic.go backend/internal/pkg/apicompat/anthropic_responses_test.go backend/internal/service/openai_gateway_service.go backend/internal/service/openai_gateway_service_test.go
git diff --check
```

Expected: 无格式问题、无 whitespace error。

**Step 3: license 复核**

Run:

```bash
git diff c056db740d56ce008292a7b414c804cc6f308208..c92b88e34abd1b032aa5307d760b8bc0aad49b28 -- LICENSE*
```

Expected: 输出为空。本轮不修改本项目 license。

## Rollback Notes

若 `Read.pages` 修复引发工具调用兼容问题，只回滚 `sanitizeAnthropicToolUseInput`、流式 `Read` 参数缓存逻辑及对应测试，不影响 OpenAI 调度层。

若 `GenerateExplicitSessionHash` 引入后未被任何 handler 使用，可保留为小型纯函数能力；若团队希望零未使用代码，则回滚 Task 2，等 OpenAI Images handler 真正引入时一并实现。

## Review Checklist

- `Read` 工具只删除值为 JSON 空字符串的 `pages` 字段，不删除非空 pages，也不删除其它工具字段。
- 流式 `Read` 工具参数只在 done 时输出一次完整 JSON，避免先透传脏 delta 再追加干净 JSON。
- `GenerateSessionHash` 的内容 fallback 不被移除，避免破坏聊天多轮粘性会话。
- 图片请求只有显式会话信号时才粘住账号，避免无状态图片请求被 prompt 内容隐式固定到单账号。
- license 无变化，不产生无意义许可证 churn。
