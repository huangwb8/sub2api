# 让 chatapi 账号支持 Responses API 调度

## 背景

Codex VS Code 扩展走 `/v1/responses`（Responses API），而 sub2api 把所有 `chatapi` 类型账号硬编码为仅支持 Chat Completions 格式，导致 Responses API 调度器直接过滤掉这些账号，返回 503。

实际上，部分第三方上游（如 packyapi）同时支持 Responses API 和 Chat Completions API。需要一个按账号 opt-in 的开关，让管理员可以选择性地让 chatapi 账号也响应 Responses API 请求。

## 方案

在账号的 `extra` JSON 字段中新增 `chatapi_responses_enabled` 布尔标记（默认 `false`）。复用 `extra` 无需 schema 迁移，与现有 `openai_passthrough`、`codex_cli_only` 等标记模式一致。

## 变更清单

### 1. Account 新增 accessor 方法

**文件**: [account.go](backend/internal/service/account.go)

在 `IsOpenAIPassthroughEnabled()` 附近新增：

```go
func (a *Account) IsChatAPIResponsesEnabled() bool {
    if a == nil || a.Type != AccountTypeChatAPI || a.Extra == nil {
        return false
    }
    if enabled, ok := a.Extra["chatapi_responses_enabled"].(bool); ok {
        return enabled
    }
    return false
}
```

### 2. 修改调度格式兼容检查

**文件**: [openai_account_scheduler.go:797](backend/internal/service/openai_account_scheduler.go#L797)

`isAccountAPIFormatCompatible` 新增分支：当请求格式为 `OpenAIAPIFormatResponses` 且账号启用了 `IsChatAPIResponsesEnabled()` 时，视为兼容。

```go
func (s *defaultOpenAIAccountScheduler) isAccountAPIFormatCompatible(account *Account, requiredFormat OpenAIAPIFormat) bool {
    if requiredFormat == OpenAIAPIFormatAny {
        return true
    }
    if account == nil {
        return false
    }
    if account.OpenAIAPIFormat() == requiredFormat {
        return true
    }
    if requiredFormat == OpenAIAPIFormatResponses && account.IsChatAPIResponsesEnabled() {
        return true
    }
    return false
}
```

### 3. Responses API 转发路径支持 chatapi

**文件**: [openai_gateway_service.go](backend/internal/service/openai_gateway_service.go)

**3a.** `buildUpstreamRequestOpenAIPassthrough`（~行 2735）：在 `case AccountTypeAPIKey:` 后新增 `case AccountTypeChatAPI:`，逻辑与 apikey 一致（通过 `GetOpenAIBaseURL()` 获取 base_url，构建 `/v1/responses` URL）。

**3b.** `buildUpstreamRequest`（~行 3276）：同上，新增 `case AccountTypeChatAPI:`。

**3c.** 请求体字段处理（~行 2072、2101）：将 `account.Type == AccountTypeAPIKey` 条件扩展为 `account.Type == AccountTypeAPIKey || account.Type == AccountTypeChatAPI`，因为第三方上游同样不支持 `max_output_tokens` 和 `max_completion_tokens`。

### 4. 前端：编辑账号弹窗新增开关

**文件**: [EditAccountModal.vue](frontend/src/components/account/EditAccountModal.vue)

仅当 `platform === 'openai' && type === 'chatapi'` 时显示 toggle。复用现有 `openaiPassthroughEnabled` 开关的 UI 模式和 load/save 逻辑。

### 5. i18n 文案

**文件**: [en.ts](frontend/src/i18n/locales/en.ts)、[zh.ts](frontend/src/i18n/locales/zh.ts)

- 中文：`同时服务 Responses API` / `开启后，此 ChatAPI 账号也可被调度用于 Responses API 请求。仅在上游代理支持 Responses API 格式时开启。`

### 6. 测试

**文件**: [openai_account_scheduler_test.go](backend/internal/service/openai_account_scheduler_test.go)

新增测试：验证带 `chatapi_responses_enabled: true` 的 chatapi 账号可被 `OpenAIAPIFormatResponses` 选中，不带标记的仍被排除。现有 `FiltersByAPIFormat` 测试无需修改（默认行为不变）。

## 验证

1. `cd backend && go test -tags=unit ./internal/service/ -run TestSelectAccountWithScheduler -v`
2. `cd backend && go build ./...` 确认编译通过
3. 在远程站点管理后台为 packycode 账号开启 `chatapi_responses_enabled`，用 Codex 发请求验证 503 是否消失
