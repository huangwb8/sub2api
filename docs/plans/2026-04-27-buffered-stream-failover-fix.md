# Buffered 流式响应缺少终止事件时的 Failover 优化计划

## 背景

在 [2026-04-27-admin-ops-health-diagnosis.md](2026-04-27-admin-ops-health-diagnosis.md) 中发现 `gpt-5.4` 流式终止错误反复出现：过去 1 小时全部 6 个 request-error 均为 `Upstream stream ended without a terminal response event`（502），来自同一用户、同一模型、涉及 3 个不同上游账号。

排查代码后发现这是一个可优化的逻辑缺陷：当上游 SSE 流异常断开（未发送终止事件）时，系统直接给客户端返回 502，不触发账号切换重试。

## 问题分析

### 涉及文件与位置

| 文件 | 行号 | 函数 |
|------|------|------|
| `backend/internal/service/openai_gateway_chat_completions.go` | 295-297 | `handleChatBufferedStreamingResponse` |
| `backend/internal/service/openai_gateway_messages.go` | 339-341 | 对应的 buffered 处理函数 |

### 当前代码逻辑（以 chat_completions 为例）

```go
// openai_gateway_chat_completions.go:295-297
if finalResponse == nil {
    writeChatCompletionsError(c, http.StatusBadGateway, "api_error",
        "Upstream stream ended without a terminal response event")
    return nil, fmt.Errorf("upstream stream ended without terminal event")
}
```

### 问题链路

1. **提前写入响应**：`writeChatCompletionsError` 调用 `c.JSON(502, ...)`，已将 502 发送给客户端
2. **错误类型不对**：返回 `fmt.Errorf(...)` 而非 `*UpstreamFailoverError`
3. **调用方无法 failover**：`openai_chat_completions.go:209` 通过 `errors.As(err, &failoverErr)` 判断是否换账号——普通 error 不匹配，跳过 failover 逻辑
4. **无 Ops 上报**：此处没有调用 `appendOpsUpstreamError`，ops 监控面板依赖上游 HTTP 层的错误上报，流层面的缺失终止事件可能未被完整追踪

### 后果

- 上游偶发断流 → 直接 502 → 不尝试其他账号 → 用户必须手动重试
- 如果某个上游账号持续异常（断流但不返回标准错误码），同一用户反复请求会反复命中同一个坏账号
- 这正是诊断报告中观察到的现象：同一用户每隔约 1.5 分钟重复 502

### 对比：HTTP 层错误已正确处理

同一个函数的 HTTP 层错误处理（[第 155-183 行](../../backend/internal/service/openai_gateway_chat_completions.go)）已正确返回 `UpstreamFailoverError` 触发 failover。流层面的同类错误是遗漏。

## 优化方案

### 核心改动

将 buffered 路径的 "写 502 + 返回普通 error" 改为 "不写响应 + 返回 `UpstreamFailoverError`"，让调用方的 failover 循环尝试其他账号。

### 安全性前提

Buffered 路径（`stream=false`）在到达第 295 行之前，**没有调用过 `c.Writer.WriteHeader()`**，响应尚未提交。因此不写错误响应、改为返回 failover error 是安全的——调用方会在下一轮重试中重新写入成功响应。

### 具体改动

#### 改动 1：`openai_gateway_chat_completions.go:295-297`

```go
// Before
if finalResponse == nil {
    writeChatCompletionsError(c, http.StatusBadGateway, "api_error",
        "Upstream stream ended without a terminal response event")
    return nil, fmt.Errorf("upstream stream ended without terminal event")
}

// After
if finalResponse == nil {
    return nil, &UpstreamFailoverError{
        StatusCode: http.StatusBadGateway,
        ResponseBody: []byte("Upstream stream ended without a terminal response event"),
    }
}
```

#### 改动 2：`openai_gateway_messages.go:339-341`

同样的模式，将 `writeAnthropicError` + `fmt.Errorf` 替换为返回 `UpstreamFailoverError`。

#### 改动 3（可选增强）：补充 Ops 错误上报

在返回 `UpstreamFailoverError` 之前，调用 `appendOpsUpstreamError` 上报事件，使 ops 监控面板能追踪流层面缺少终止事件的情况：

```go
if finalResponse == nil {
    appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
        Platform:           account.Platform,
        AccountID:          account.ID,
        AccountName:        account.Name,
        UpstreamStatusCode: http.StatusBadGateway,
        Kind:               "failover",
        Message:            "Upstream stream ended without a terminal response event",
    })
    return nil, &UpstreamFailoverError{
        StatusCode:   http.StatusBadGateway,
        ResponseBody: []byte("Upstream stream ended without a terminal response event"),
    }
}
```

注意：`appendOpsUpstreamError` 需要 `account` 参数，需确认该函数签名中是否能获取到。如果当前函数不接收 `account`，可能需要将 account 传入或在调用方补充上报。

### 调用方行为（无需改动）

调用方 [openai_chat_completions.go:209-246](../../backend/internal/handler/openai_chat_completions.go) 已有完整的 failover 循环：

```
收到 UpstreamFailoverError
  → 临时标记当前账号不可用
  → 记录失败账号 ID
  → 选择新账号重试（最多 maxAccountSwitches 次）
  → 全部耗尽时才返回最终错误
```

返回 `UpstreamFailoverError` 后，上述逻辑自动生效，无需额外改动 handler 层。

同时，`TempUnscheduleRetryableError`（[openai_gateway_service.go:1794](../../backend/internal/service/openai_gateway_service.go)）会对 502 状态码的 failover error 触发临时封禁，自动将断流账号暂时移出调度池，防止后续请求继续命中。

### 流式路径（不改）

`handleChatStreamingResponse`（`stream=true`）在进入读取循环前已调用 `c.Writer.WriteHeader(http.StatusOK)`，响应已提交。此时无法 failover，当前行为（发送 `[DONE]` 关闭流）是合理的选择。不修改流式路径。

## 预期效果

| 场景 | 改动前 | 改动后 |
|------|--------|--------|
| 上游偶发断流 | 直接 502，用户需手动重试 | 自动切换账号重试，用户无感 |
| 上游账号持续断流 | 反复 502，每次都命中坏账号 | 自动临时封禁坏账号，切换到正常账号 |
| ops 监控 | 可能漏报流层面错误 | 通过 `appendOpsUpstreamError` 完整追踪（如采纳改动 3） |

## 风险评估

- **风险等级**：低
- **影响范围**：仅影响 `stream=false` 的 buffered 路径
- **回归风险**：调用方 failover 逻辑已成熟（HTTP 层错误已长期使用该机制），新增一个 failover 触发点的行为与现有模式一致
- **不涉及**：流式路径（`stream=true`）、Anthropic/Gemini/Bedrock 平台的网关逻辑

## 验证方式

1. 单元测试：模拟上游返回不完整 SSE 流（缺少终止事件），验证返回 `UpstreamFailoverError` 而非普通 error
2. 手动验证：在测试环境配置一个会断流的上游账号，发送 `stream=false` 请求，确认系统自动切换到另一个账号并成功返回
3. 集成测试：确认 ops 监控面板能正确追踪此类 failover 事件（如采纳改动 3）
