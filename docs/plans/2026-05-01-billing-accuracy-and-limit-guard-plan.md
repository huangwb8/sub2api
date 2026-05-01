# 计费准确性与限额保护优化计划

**Goal:** 降低“少计用户消耗”和“隐性超量”风险，让成功消耗尽量被完整计费，并让订阅/API Key/余额限额在高并发和大请求场景下更可控。

**Architecture:** 分三层推进：先修复已知漏计路径，再增加软限额保护垫，最后补齐审计和告警。计费事实仍以请求完成后的真实 usage 为准，不在第一阶段引入复杂预扣系统。

**Tech Stack:** Go / Gin / Ent ORM / PostgreSQL / Redis。

**Minimal Change Scope:** 优先修改 `backend/internal/service/gateway_service.go`、`backend/internal/service/openai_gateway_service.go`、`backend/internal/service/billing_cache_service.go`、`backend/internal/repository/usage_billing_repo.go`、相关测试与必要配置。除非发现现有字段不足，否则不新增表；暂不改前端 UI。

**Success Criteria:** 流式中断但已解析到 usage 的请求不会完全漏计；订阅、API Key rate limit、余额模式能减少单次请求导致的明显越线；远程只读核对中 `usage_logs` 与订阅/API Key 用量差异可解释、可告警。

**Verification Plan:** 运行 `cd backend && go test -tags=unit ./...`；补充计费单元测试覆盖流式中断、重复 request_id、订阅日限额保护垫、API Key rate limit 保护垫、余额透支边界；用 `remote.env` 只读接口复核活跃订阅、API Key 和 usage 汇总。

---

## 背景

本次源码调研发现两类风险：

- 少计用户消耗：流式响应在缺少终止事件、客户端断开或上游读错误时，即使已解析到部分 usage，当前主路径也可能直接返回错误，不进入 `RecordUsage`，导致这部分消耗不扣费。
- 隐性超量：请求前只检查“当前已用量是否达到限额”，请求完成后才知道真实成本。因此当用户接近日限额或余额接近 0 时，单次大请求或并发请求可能让用量越过阈值。

远程只读核对显示，当前没有发现活跃订阅/API Key 已大面积超限；但看到过一个订阅的 `usage_logs` 今日汇总略高于订阅表 `daily_usage_usd` 的小额差异。这个差异不大，但说明需要更明确的审计和对账能力。

## 非目标

- 不追求“永远不超一分钱”的强一致预扣系统。
- 不阻止所有接近限额用户发起请求；第一阶段采用保守保护垫和告警。
- 不改变模型定价口径、渠道定价口径和倍率规则。
- 不把错误请求伪装成成功请求；只对已确认有 usage 的消耗进行计费或审计。

## 关键设计

### 计费事实优先级

计费应遵循以下优先级：

| 情况 | 处理策略 |
|------|----------|
| 成功响应且 usage 完整 | 正常按真实 usage 扣费 |
| 流式中断但 usage 非零 | 按已解析 usage 扣费，日志标记为 partial/incomplete |
| 流式中断且 usage 为零 | 不扣费，只记录错误/观测信息 |
| usage 解析失败但上游明确成功 | 记录解析失败告警，避免静默跳过 |

### 限额保护策略

限额检查从“只看已用量”升级为“已用量 + 保护垫”：

| 限额类型 | 第一阶段策略 |
|----------|--------------|
| 订阅日/周/月限额 | 当剩余额度低于配置保护垫时拒绝或降级提示 |
| API Key 5h/1d/7d 限额 | 与订阅类似，按窗口剩余额度设置保护垫 |
| 余额模式 | 允许极小透支，但设置最大负余额保护线 |
| 并发请求 | 接受短暂 soft overrun，但请求后立即刷新缓存，减少连续放行 |

保护垫应可配置，默认值建议保守：

- `billing.limit_guard.min_remaining_usd`: `0.5`
- `billing.limit_guard.percent`: `1%`
- `billing.balance.max_overdraft_cny`: 可按站点汇率折算为约 `1 USD`

实际阈值取 `max(min_remaining_usd, limit * percent)`。

## 实施任务

### Task 1: 补齐流式中断 partial usage 计费

**Files:**

- Modify: `backend/internal/service/gateway_service.go`
- Modify: `backend/internal/service/openai_gateway_service.go`
- Test: `backend/internal/service/gateway_streaming_test.go`
- Test: `backend/internal/service/gateway_record_usage_test.go`

**Steps:**

1. 梳理 `handleStreamingResponse` 返回 `streamingResult` 和 `error` 的路径。
2. 当错误属于 `stream usage incomplete` 且 `usage` 非零时，允许上层继续构造 `ForwardResult`。
3. 给结果增加或复用标记，区分 `ClientDisconnect`、`PartialUsage`、`UsageIncompleteReason`。
4. `RecordUsage` 正常按 partial usage 计费，但 usage log 中要能体现不完整状态。
5. 如果现有 `usage_logs` 没有合适字段，优先复用错误日志/系统日志；确需结构化查询时再新增轻量字段。

**Risk:** 中等。需要避免把真实失败且无 usage 的请求误扣费。

### Task 2: 增加 partial usage 幂等测试

**Files:**

- Test: `backend/internal/service/gateway_record_usage_test.go`
- Test: `backend/internal/repository/usage_billing_repo_integration_test.go`

**Steps:**

1. 构造同一 `request_id + api_key_id` 重复提交 partial usage 的场景。
2. 验证 `usage_billing_dedup` 仍只扣一次。
3. 验证 request fingerprint 冲突时返回冲突错误，不静默重复扣费。
4. 覆盖订阅、余额、API Key quota/rate limit 三类副作用。

**Risk:** 低到中。重点是保护现有幂等语义。

### Task 3: 加订阅限额保护垫

**Files:**

- Modify: `backend/internal/service/billing_cache_service.go`
- Modify: `backend/internal/config/config.go`
- Test: `backend/internal/service/billing_cache_service_test.go`

**Steps:**

1. 新增配置读取与默认值。
2. 在 `checkSubscriptionEligibility` 中计算日/周/月剩余额度。
3. 当剩余额度小于保护垫时返回现有限额错误，避免引入新错误码导致客户端兼容问题。
4. 日志中记录触发保护垫的窗口、limit、used、remaining、guard。
5. 保持无订阅限额的分组不受影响。

**Risk:** 中等。保护垫过大可能提前拒绝合法用户，需要默认值谨慎。

### Task 4: 加 API Key rate limit 保护垫

**Files:**

- Modify: `backend/internal/service/billing_cache_service.go`
- Test: `backend/internal/service/api_key_service_quota_test.go`
- Test: `backend/internal/service/billing_cache_service_test.go`

**Steps:**

1. 在 `evaluateRateLimits` 中使用同一保护垫策略。
2. 对 5h/1d/7d 三个窗口分别计算剩余额度。
3. 窗口过期后的内存归零逻辑保持不变。
4. 远程/本地查询接口显示的 `remaining` 可以暂不改变，避免 UI 口径突然变化；错误判断先应用保护垫。

**Risk:** 低到中。当前站点配置 API Key rate limit 较少，影响面有限。

### Task 5: 加余额最大透支保护

**Files:**

- Modify: `backend/internal/service/billing_cache_service.go`
- Modify: `backend/internal/repository/usage_billing_repo.go`
- Test: `backend/internal/repository/usage_billing_repo_integration_test.go`

**Steps:**

1. 请求前仍检查余额必须大于 0。
2. DB 扣费时增加可配置下限：例如 `balance - amount >= -max_overdraft_cny`。
3. 如果扣费会越过最大透支线，返回明确错误，并确保 usage log 不写成已扣费成功。
4. 对订阅模式不应用余额透支规则。

**Risk:** 中等。请求已经成功后再拒绝扣费会形成“用户拿到结果但未扣费”的边界问题；第一版可只在请求前用余额保护垫拦截，DB 下限作为最后保险。

### Task 6: 增加对账与告警查询

**Files:**

- Modify: `backend/internal/repository/usage_log_repo.go`
- Optional: `backend/internal/handler/admin/usage_handler.go`
- Test: 对应 repository/handler 测试

**Steps:**

1. 增加只读对账函数：按订阅维度比较 `user_subscriptions.daily_usage_usd` 与当日 `usage_logs.actual_cost`。
2. 增加 API Key rate limit 对账：比较 `api_keys.usage_1d` 与当日对应 `usage_logs.actual_cost`。
3. 输出差异超过阈值的记录，供管理员排查。
4. 初期可只作为内部 service/repository 方法，不急于做 UI。

**Risk:** 低。只读分析，不影响热路径。

### Task 7: 远程站点回归核对

**Steps:**

1. 使用 `remote.env` 只读查询活跃订阅和分组限额。
2. 对比每个活跃订阅今日 `daily_usage_usd` 与 `usage_logs` 今日汇总。
3. 查询所有配置了 API Key rate limit 的 key，确认 `usage_* <= limit` 或差异可解释。
4. 抽查系统日志中是否仍出现 `stream usage incomplete` 后无 usage 记录的模式。
5. 不执行任何写操作。

**Risk:** 低。注意不输出真实 API Key、用户隐私和密钥。

## 验证矩阵

| 场景 | 预期 |
|------|------|
| 正常非流式成功 | 正常计费，行为不变 |
| 正常流式成功 | 正常计费，行为不变 |
| 流式中断，usage 非零 | 按 partial usage 计费，并可审计 |
| 流式中断，usage 为零 | 不扣费，记录错误/告警 |
| 同一请求重复上报 | 只扣一次 |
| 订阅剩余额度低于保护垫 | 请求前拒绝 |
| API Key rate limit 剩余额度低于保护垫 | 请求前拒绝 |
| 余额接近 0 | 根据保护垫/最大透支策略拒绝或限制 |
| Redis rate limit 缓存未命中 | DB 回源后按同一保护垫判断 |

## 推荐落地顺序

1. **P0:** Task 1 + Task 2，先堵住 partial usage 完全漏计。
2. **P1:** Task 3 + Task 4，降低订阅和 API Key 的隐性超量。
3. **P1:** Task 5，给余额模式加最后保险。
4. **P2:** Task 6 + Task 7，形成可持续对账和远程回归流程。

## 开放问题

- partial usage 是否需要新增 `usage_logs.usage_status` 字段，还是先用系统日志和 `client_disconnect` 类字段承接。
- 保护垫默认值是否按全站统一，还是按分组/计划可覆盖。
- 余额最大透支用 CNY 配置还是 USD 配置后按汇率折算。
- 对已经成功但扣费失败的请求，是否需要新增补偿队列；第一阶段建议先通过现有幂等扣费和告警定位，不立即引入队列。
