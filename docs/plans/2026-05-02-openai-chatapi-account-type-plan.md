# 新增 OpenAI "Chat Completions API" 账户类型

## Context

当前网关对所有 OpenAI 账户统一使用 Responses API (`/responses`) 转发请求，即使用户发送的是 Chat Completions 请求也会被转换为 Responses 格式。这导致只支持标准 Chat Completions API 的上游服务商（如 packyapi.com）无法作为账户接入。

**目标**：在 OpenAI 平台下新增第 3 种账户类型 "Chat Completions API"，直接透传 `/v1/chat/completions` 请求到上游，不做格式转换。

## 设计决策

**新增账户类型常量 `chatapi`**（而非在 `apikey` 上加 flag），原因：
- `apikey` 类型深度绑定 Responses API 路径，加 flag 需在 10+ 处加条件分支
- 独立类型让调度器过滤更清晰（`/v1/responses` 请求排除 `chatapi` 账户）
- 前端天然呈现为第 3 张卡片

## 实施步骤

### 1. 后端：常量与校验

**`backend/internal/domain/constants.go:33`** — 新增：
```go
AccountTypeChatAPI = "chatapi" // Chat Completions API 类型账号（直接转发 /v1/chat/completions）
```

**`backend/internal/handler/admin/account_handler.go`** — 更新 Create/Update 请求的 `oneof` 校验，加入 `chatapi`。

### 2. 后端：Account 辅助方法

**`backend/internal/service/account.go`** — 三处扩展：

- `GetOpenAIBaseURL()`（约 L1222）：条件从 `a.Type == AccountTypeAPIKey` 扩展为 `a.Type == AccountTypeAPIKey || a.Type == AccountTypeChatAPI`
- `GetOpenAIApiKey()`（约 L1256）：同上
- 新增 `IsOpenAIChatAPI()` 方法

### 3. 后端：URL 构建器

**`backend/internal/service/openai_gateway_service.go`** — 新增函数：
```go
func buildOpenAIChatCompletionsURL(base string) string
```
- `api.openai.com` → `{base}/v1/chat/completions`
- `xxx.com/v1` → `{base}/chat/completions`
- 其他 → `{base}/v1/chat/completions`

### 4. 后端：GetAccessToken 扩展

**`backend/internal/service/openai_gateway_service.go:1786`** — `GetAccessToken` switch 新增 `case AccountTypeChatAPI`，返回 `api_key` 凭据。

### 5. 后端：核心转发函数（核心变更）

**`backend/internal/service/openai_gateway_chat_completions.go`** — 新增 `ForwardAsChatCompletionsDirect` 方法：

- 解析 `model` / `stream` 字段（轻量提取，不复用全量反序列化）
- 调用 `resolveOpenAIForwardModel` 解析模型映射
- 必要时替换 body 中的 model（`ReplaceModelInBody`）
- `GetAccessToken` 获取 token
- `buildOpenAIChatCompletionsURL` 构建 URL
- **原始 body 直接转发**，不走 `ChatCompletionsToResponses` 转换
- streaming：直接 pipe SSE 流
- non-streaming：直接返回 JSON
- 提取 `usage.prompt_tokens` / `usage.completion_tokens` 用于计费

**`backend/internal/service/openai_gateway_service.go`** — 新增 `buildUpstreamChatCompletionsRequest` 辅助方法（比现有 `buildUpstreamRequest` 更简单，无 OAuth 头、无 session 头）。

### 6. 后端：Handler 调度

**`backend/internal/handler/openai_chat_completions.go:198`** — 按账户类型分派：
```go
if account.Type == service.AccountTypeChatAPI {
    result, err = h.gatewayService.ForwardAsChatCompletionsDirect(...)
} else {
    result, err = h.gatewayService.ForwardAsChatCompletions(...)
}
```

### 7. 后端：调度器过滤

**`backend/internal/service/openai_account_scheduler.go`**：

- `OpenAIAccountScheduleRequest`（L24）新增 `RequiredAPIFormat` 字段
- 新增类型 `OpenAIAPIFormat` 及常量（`""` / `"responses"` / `"chat"`）
- `Account` 新增 `OpenAIAPIFormat()` 方法：`chatapi` → `"chat"`，其他 → `"responses"`
- `selectByLoadBalance`（L609 循环内）新增过滤：`req.RequiredAPIFormat` 不匹配时 skip

**`backend/internal/handler/openai_gateway_handler.go:258`** — Responses handler 传入 `RequiredAPIFormat = "responses"`，排除 `chatapi` 账户。

Chat Completions handler 保持 `RequiredAPIFormat = ""`（any），两种账户都能服务。

### 8. 后端：Account 测试服务

**`backend/internal/service/account_test_service.go:412`** — `testOpenAIAccountConnection` 新增 `chatapi` 分支：
- 使用 `buildOpenAIChatCompletionsURL` 构建 URL
- 使用 Chat Completions 格式的测试 payload
- 解析标准 Chat Completions SSE 格式

### 9. 前端：类型与 UI

**`frontend/src/types/index.ts:672`** — `AccountType` 联合类型加入 `'chatapi'`。

**`frontend/src/components/account/CreateAccountModal.vue`**：
- OpenAI 区域 grid 改为 3 列，新增第 3 张卡片（Chat Completions API）
- `accountCategory` ref / `effectiveCreateAccountType` computed 加入 `chatapi` 分支
- API Key + Base URL 表单字段的 `v-if` 条件加入 `chatapi`

**`frontend/src/i18n/locales/en.ts` & `zh.ts`** — 新增翻译：
- `types.chatCompletionsApi`: "Chat Completions API" / "Chat Completions API"
- `types.chatCompletionsDesc`: "Direct /v1/chat/completions passthrough" / "直接透传 /v1/chat/completions"

## 关键文件清单

| 文件 | 变更 |
|------|------|
| `backend/internal/domain/constants.go` | 新增 `AccountTypeChatAPI` |
| `backend/internal/service/account.go` | 扩展 `GetOpenAIBaseURL`/`GetOpenAIApiKey`，新增 `IsOpenAIChatAPI` |
| `backend/internal/service/openai_gateway_service.go` | 新增 URL 构建器、`GetAccessToken` 扩展、请求构建器 |
| `backend/internal/service/openai_gateway_chat_completions.go` | 新增 `ForwardAsChatCompletionsDirect`（核心） |
| `backend/internal/service/openai_account_scheduler.go` | 新增 `RequiredAPIFormat` 调度过滤 |
| `backend/internal/handler/openai_chat_completions.go` | 按类型分派转发 |
| `backend/internal/handler/openai_gateway_handler.go` | Responses handler 传入 API format 约束 |
| `backend/internal/service/account_test_service.go` | `chatapi` 测试路径 |
| `backend/internal/handler/admin/account_handler.go` | 校验 `oneof` 加入 `chatapi` |
| `frontend/src/types/index.ts` | 类型扩展 |
| `frontend/src/components/account/CreateAccountModal.vue` | 第 3 张卡片 + 条件分支 |
| `frontend/src/i18n/locales/en.ts` & `zh.ts` | 翻译 |

## 验证方案

1. **创建账户**：通过 Admin API 或前端创建 `chatapi` 类型 OpenAI 账户（test01 参数），验证创建成功
2. **连通性测试**：Admin `POST /accounts/:id/test` 验证上游连通
3. **Chat Completions 请求**：用用户 API Key 发送 `/v1/chat/completions` 请求，验证 `chatapi` 账户被调度且返回正确结果
4. **Responses 请求隔离**：发送 `/v1/responses` 请求，验证 `chatapi` 账户不被选中
5. **Failover**：`chatapi` 账户失败时正确触发 failover
6. **后端测试**：`cd backend && go test -tags=unit ./...`
7. **前端**：`cd frontend && pnpm run typecheck && pnpm test`
