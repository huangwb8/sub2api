# 上游连接错误 Failover 修复计划

> **日期：** 2026-04-26
> **状态：** 已实施
> **优先级：** P0（线上正在持续 502）
> **关联文档：** [2026-04-26-openai-temp-unschedulable-failover-plan.md](./2026-04-26-openai-temp-unschedulable-failover-plan.md)

---

## 问题概述

当 SOCKS5 代理连接失败（`connection refused`、超时等）时，上游请求在 HTTP 协议层之前就失败了。此时 `DoWithTLS` 返回 `error` 而非 HTTP 响应。**所有 Forward 函数对这类错误的处理都绕过了 failover 机制**，导致：

1. 请求直接以 502 返回客户端，没有尝试切换账户
2. 临时不可调度规则（temp-unschedulable）不会被触发，因为 `HandleUpstreamError` 只处理有 HTTP 响应的场景
3. Session sticky 持续绑定到坏账户，后续请求全部失败

## 实施结果

已完成三项闭环修复：

- 连接级上游错误不再由 Forward 层提前写入 502，而是统一包装为 `UpstreamFailoverError`，交给 Handler 层执行账号 failover。
- `count_tokens` 路径补齐账号 failover 循环，避免连接错误返回未写响应或无法换号。
- OpenAI 粘性会话增加运行时错误率保护，并新增 `gateway.openai_ws.sticky_session_error_rate_threshold` / `gateway.openai_ws.sticky_session_error_rate_min_samples` 配置；连接级 502 也会触发短时临时不可调度。

### 线上实锤

```
upstream_error_message: "socks connect tcp gate.decodo.com:7000->chatgpt.com:443: unknown error connection refused"
```

- 健康评分：28/100
- 错误率：81.26%（460/571 请求失败）
- 涉及账户：account 10（Decodo proxy 46 不可达）
- 影响模型：gpt-5.5、gpt-5.4 等所有通过该账户的请求
- 持续时间：已超过 1 小时，仍在进行

---

## 根因分析

### Bug 1：连接错误不触发 Failover（代码 Bug）

**涉及文件与行号：**

| # | 文件 | 行号 | 函数 | 严重程度 |
|---|------|------|------|---------|
| 1 | `gateway_forward_as_responses.go` | L121 | `ForwardAsResponses` | **Critical** |
| 2 | `gateway_forward_as_chat_completions.go` | L123 | `ForwardAsChatCompletions` | **Critical** |
| 3 | `gateway_service.go` | L4093 | `Forward` | **Critical** |
| 4 | `gateway_service.go` | L4583 | `Forward` passthrough 分支 | **Critical** |
| 5 | `gateway_service.go` | L5291 | `Forward` Bedrock 分支 | High |
| 6 | `gateway_service.go` | L8252 | `ForwardCountTokens` | Medium |
| 7 | `openai_gateway_chat_completions.go` | L128 | `ForwardAsChatCompletions` | **Critical** |
| 8 | `openai_gateway_messages.go` | L165 | `ForwardAsAnthropic` | **Critical** |
| 9 | `openai_gateway_service.go` | L2342 | `Forward` | **Critical** |
| 10 | `openai_gateway_service.go` | L2563 | `ForwardPassthrough` | **Critical** |
| 11 | `antigravity_gateway_service.go` | L4259 | `ForwardAsAntigravity` | High |

**统一模式：**

```go
// 当前代码（错误）
resp, err := s.httpUpstream.DoWithTLS(...)
if err != nil {
    writeXxxError(c, http.StatusBadGateway, ...)  // 已写入响应 → 无法 failover
    return nil, fmt.Errorf("upstream request failed: %s", safeErr)  // 非 UpstreamFailoverError
}
```

Handler 层通过 `errors.As(err, &failoverErr)` 检测 `UpstreamFailoverError`。普通 `error` 匹配不到，failover 循环不会触发。

**额外问题：先写入响应再返回**

所有 Forward 函数在返回前直接 `writeXxxError(c, 502, ...)` 写入 HTTP 响应。Handler 层即使收到 `UpstreamFailoverError`，也会因为 `c.Writer.Size() != writerSizeBeforeForward` 而跳过 failover。

### Bug 2：Session Sticky 不清除（设计缺陷）

`shouldClearStickySession` 只检查：

- 账户状态（Error/Disabled）
- 账户可调度性
- 模型限速剩余时间

**不检查**：上游连接失败、代理故障、EWMA 错误率。

导致 session sticky 一旦绑定到坏账户，永远不会被清除（除非账户状态变为 Error 或触发限速）。

### Bug 3：临时不可调度规则不覆盖连接错误（设计缺陷）

`rateLimitService.HandleUpstreamError` 只在 `resp.StatusCode >= 400` 时被调用（即有 HTTP 响应时）。代理连接失败没有 HTTP 响应，temp-unschedulable 规则不会被触发。

---

## 修复方案

### 修复 A：连接错误包装为 UpstreamFailoverError（核心修复）

**目标：** 代理连接失败时，不写入响应，返回 `UpstreamFailoverError`，让 Handler 层 failover 循环正常工作。

**修改原则：**

1. **不写入响应**：移除 Forward 函数中的 `writeXxxError` 调用
2. **返回 UpstreamFailoverError**：用 `&UpstreamFailoverError{StatusCode: 502}` 替代 `fmt.Errorf`
3. **保留 Ops 错误记录**：`setOpsUpstreamError` 和 `appendOpsUpstreamError` 保留（纯元数据，不写响应）

**修改模板（适用于所有 11 处）：**

```go
// 修改前
resp, err := s.httpUpstream.DoWithTLS(...)
if err != nil {
    if resp != nil && resp.Body != nil {
        _ = resp.Body.Close()
    }
    safeErr := sanitizeUpstreamErrorMessage(err.Error())
    setOpsUpstreamError(c, 0, safeErr, "")
    appendOpsUpstreamError(c, OpsUpstreamErrorEvent{...})
    writeXxxError(c, http.StatusBadGateway, "server_error", "Upstream request failed")  // ← 删除
    return nil, fmt.Errorf("upstream request failed: %s", safeErr)  // ← 改为 UpstreamFailoverError
}

// 修改后
resp, err := s.httpUpstream.DoWithTLS(...)
if err != nil {
    if resp != nil && resp.Body != nil {
        _ = resp.Body.Close()
    }
    safeErr := sanitizeUpstreamErrorMessage(err.Error())
    setOpsUpstreamError(c, 0, safeErr, "")
    appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
        Platform:           account.Platform,
        AccountID:          account.ID,
        AccountName:        account.Name,
        UpstreamStatusCode: 0,
        Kind:               "request_error",
        Message:            safeErr,
    })
    return nil, &UpstreamFailoverError{  // ← 改为 UpstreamFailoverError
        StatusCode:   http.StatusBadGateway,
        ResponseBody: []byte(safeErr),
    }
}
```

**需要逐一修改的文件：**

#### A1. `gateway_forward_as_responses.go`（最高优先级）

```
文件：backend/internal/service/gateway_forward_as_responses.go
行号：120-137
函数：ForwardAsResponses
当前：writeResponsesError + fmt.Errorf
修改为：UpstreamFailoverError（不写响应）
```

#### A2. `gateway_forward_as_chat_completions.go`

```
文件：backend/internal/service/gateway_forward_as_chat_completions.go
行号：123-139
函数：ForwardAsChatCompletions
当前：writeGatewayCCError + fmt.Errorf
修改为：UpstreamFailoverError（不写响应）
```

#### A3. `gateway_service.go` — Forward 主函数

```
文件：backend/internal/service/gateway_service.go
行号：4093-4117
函数：Forward
当前：c.JSON(502) + fmt.Errorf
修改为：UpstreamFailoverError（不写响应）
```

#### A4. `gateway_service.go` — Forward passthrough 分支

```
文件：backend/internal/service/gateway_service.go
行号：4583-4607
函数：Forward
当前：c.JSON(502) + fmt.Errorf
修改为：UpstreamFailoverError（不写响应）
```

#### A5. `gateway_service.go` — Forward Bedrock 分支

```
文件：backend/internal/service/gateway_service.go
行号：5291-5314
函数：Forward
当前：c.JSON(502) + fmt.Errorf
修改为：UpstreamFailoverError（不写响应）
```

#### A6. `gateway_service.go` — ForwardCountTokens

```
文件：backend/internal/service/gateway_service.go
行号：8252-8256
函数：ForwardCountTokens
当前：countTokensError + fmt.Errorf
修改为：UpstreamFailoverError（不写响应）
注意：此函数可能需要额外调整返回类型以支持 failover
```

#### A7. `openai_gateway_chat_completions.go`

```
文件：backend/internal/service/openai_gateway_chat_completions.go
行号：128-139
函数：ForwardAsChatCompletions
当前：writeChatCompletionsError + fmt.Errorf
修改为：UpstreamFailoverError（不写响应）
```

#### A8. `openai_gateway_messages.go`

```
文件：backend/internal/service/openai_gateway_messages.go
行号：165-176
函数：ForwardAsAnthropic
当前：writeAnthropicError + fmt.Errorf
修改为：UpstreamFailoverError（不写响应）
```

#### A9. `openai_gateway_service.go` — Forward

```
文件：backend/internal/service/openai_gateway_service.go
行号：2342-2353
函数：Forward
当前：c.JSON(502) + fmt.Errorf
修改为：UpstreamFailoverError（不写响应）
```

#### A10. `openai_gateway_service.go` — ForwardPassthrough

```
文件：backend/internal/service/openai_gateway_service.go
行号：2563-2574
函数：ForwardPassthrough
当前：c.JSON(502) + fmt.Errorf
修改为：UpstreamFailoverError（不写响应）
```

#### A11. `antigravity_gateway_service.go` — ForwardAsAntigravity

```
文件：backend/internal/service/antigravity_gateway_service.go
行号：4259
函数：ForwardAsAntigravity
当前：fmt.Errorf（无 writeError，较简单）
修改为：UpstreamFailoverError
```

---

### 修复 B：Session Sticky 错误率感知清除

**目标：** 当账户的 EWMA 错误率超过阈值时，清除绑定到该账户的 sticky session。

**修改位置：** `backend/internal/service/openai_account_scheduler.go`

**方案：** 在 `selectBySessionHash` 中增加 EWMA 错误率检查：

```go
// 在 selectBySessionHash 函数中，shouldClearStickySession 检查之后追加：
if !shouldClearStickySession(account, req.RequestedModel) {
    // 新增：检查 EWMA 错误率
    if s.stats != nil {
        errorRate, _, _ := s.stats.snapshot(accountID)
        if errorRate > 0.5 {  // 阈值可配置化
            _ = s.service.deleteStickySessionAccountID(ctx, req.GroupID, sessionHash)
            return nil, nil
        }
    }
}
```

**注意事项：**

- 错误率阈值建议默认 0.5（50%），通过配置暴露
- 需要考虑样本量：新账户可能只有 1-2 次采样，EWMA 不稳定。可增加最小样本数检查
- 同步修改 `shouldClearStickySession` 函数签名或在 `selectBySessionHash` 中单独处理

---

### 修复 C：连接错误触发临时不可调度

**目标：** 代理连接失败时，通过 `TempUnscheduleRetryableError` 触发临时不可调度。

**修改位置：** `backend/internal/handler/failover_loop.go`

**方案：** 在 `HandleFailoverError` 中，对连接错误（`UpstreamFailoverError.StatusCode == 502`）也触发临时不可调度：

```go
// HandleFailoverError 中，在"同账号重试用尽"判断之后：

// 连接级错误（proxy failure、connection refused 等）也触发临时封禁
if failoverErr.StatusCode == http.StatusBadGateway {
    gatewayService.TempUnscheduleRetryableError(ctx, accountID, failoverErr)
}
```

**修改 `TempUnscheduleRetryableError`：**

当前 `tempUnscheduleEmptyResponse` 只在 `RetryableOnSameAccount == true` 时才调用 502 封禁。需要让它在非 RetryableOnSameAccount 的 502（连接错误）也能触发封禁。

---

## 修改影响范围

### Handler 层

所有调用 Forward 函数的 Handler 都有 failover 循环（`FailoverState`），已经能正确处理 `UpstreamFailoverError`。修改只影响 Forward 层的返回类型，Handler 层不需要修改。

涉及 Handler：

- `gateway_handler_responses.go` — `/v1/responses` 路由
- `gateway_handler.go` — `/v1/chat/completions`（Gateway 模式）
- `openai_chat_completions.go` — `/v1/chat/completions`（OpenAI 直连模式）
- 其他调用 Forward 函数的 Handler

### 临时不可调度

需要确认 `TempUnscheduleRetryableError` 对连接错误的封禁时长是否合理。当前 `tempUnscheduleEmptyResponse` 使用的是调度机制规则中配置的时长，对代理连接失败场景可能需要更短的封禁时间。

### 不受影响

- Gemini 平台：已有内部重试机制，不依赖 failover
- Antigravity 主循环：已有 URL fallback + 内部重试，不依赖 failover
- 正常 HTTP 响应（status >= 400）：已有 `UpstreamFailoverError` 处理，不受影响

---

## 实施步骤

### 阶段 1：核心修复（修复 A）

1. 按 A1 → A11 逐一修改，将连接错误包装为 `UpstreamFailoverError` 并移除响应写入
2. 每修改一个文件，运行对应单元测试
3. 全部修改完后运行 `cd backend && go test -tags=unit ./...`
4. 重点测试：模拟 `DoWithTLS` 返回 error，验证 Handler 层 failover 正常触发

### 阶段 2：Sticky Session 保护（修复 B）

1. 在 `selectBySessionHash` 中增加 EWMA 错误率检查
2. 在 `shouldClearStickySession` 或调用处增加错误率阈值参数
3. 编写单元测试验证高错误率账户的 sticky session 被清除

### 阶段 3：临时不可调度增强（修复 C）

1. 修改 `HandleFailoverError` 让连接错误也触发临时封禁
2. 调整 `TempUnscheduleRetryableError` 放宽 502 封禁条件
3. 编写单元测试

### 阶段 4：集成验证

1. `cd backend && go test -tags=unit ./...`
2. `cd backend && go test -tags=integration ./...`
3. 本地启动服务，模拟代理连接失败场景验证 failover
4. 部署到测试环境验证

---

## 风险与回退

### 风险

| 风险 | 概率 | 影响 | 缓解措施 |
|------|------|------|---------|
| 代理瞬时抖动导致大量 failover 切换 | 中 | 轻微增加延迟 | failover 循环有 maxSwitches 限制 |
| 移除 writeError 后 Handler 层未正确兜底写响应 | 低 | 客户端收到空响应 | 阶段 1 测试重点验证 Handler 层兜底 |
| EWMA 阈值不当导致频繁清除 sticky | 低 | 负载均衡不均 | 可配置化阈值，默认保守值 |

### 回退方案

如果修复引入问题，可以：
1. 将 `UpstreamFailoverError` 改回 `fmt.Errorf`（恢复原始行为）
2. 通过配置开关控制是否对连接错误触发 failover
