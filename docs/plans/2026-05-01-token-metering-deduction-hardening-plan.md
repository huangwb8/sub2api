# Token 计量与扣费准确性硬化计划

## 背景

2026-05-01 基于本地代码审计和 `remote.env` 远程站点只读抽样，排查“用户实际 token 消耗是否可以被正确计量和扣除”。结论是：当前主链路已经具备 usage 解析、成本计算、幂等扣费和订阅用量累加能力，但仍存在几类对管理员不利的缺陷，需要优先硬化。

远程只读抽样结果：

- 最近 500 条 usage 均为订阅扣费记录，时间范围约为 `2026-05-01 11:08:12 +0800` 到 `2026-05-01 12:26:17 +0800`。
- 最近 500 条中未发现“非零 token 但 `actual_cost=0`”记录。
- 当前 14 条订阅中 11 条 active，未发现 active 订阅已超过日限额；最高日用量约 `92.62 / 100 USD`。
- 最近 500 条中未发现图片类 usage 样本，因此 OpenAI Images 路径风险尚未在线上样本中被触发。

以上远程结果只能说明当前样本未复现异常，不能抵消代码层面的结构性风险。

## 当前链路判断

正向设计：

- `usage_billing_dedup` 通过 `(request_id, api_key_id)` 实现幂等，避免同一请求重复扣费。
- `UsageBillingRepository.Apply` 在单个数据库事务里执行订阅用量、余额、API Key quota、API Key rate limit、账号 quota 更新。
- usage log 写入在扣费后执行，避免“写日志成功但扣费失败”被误认为已经完成。
- Redis billing cache 在扣费后异步更新，并且关键缓存更新有同步回退。

主要缺口：

- 使用量记录任务可以在队列满时被丢弃，导致整次成功请求完全不扣费。
- 订阅限额采用请求前检查、请求后累加，缺少原子预占或带限额条件的原子扣减，高并发下可能明显超过日限额。
- 计价失败会回退为 `ActualCost=0` 并继续写 usage，未知模型或定价解析异常会形成免费调用。
- OpenAI Images 成功返回但 usage 缺失时，当前 `OpenAIGatewayService.RecordUsage` 会直接跳过记录，未使用 `ImageCount/ImageSize` 做按次兜底。
- OpenAI 流式响应在已收集部分 usage 但缺少终止事件时返回 error，handler 不会执行扣费，存在部分消耗漏扣风险。

## P0 问题

### P0-1 使用量记录队列满时会丢扣费任务

证据：

- `backend/internal/service/usage_record_worker_pool.go` 默认 `overflow_policy=sample`，队列满时只有部分任务同步执行，其余返回 `dropped`。
- gateway handler 提交 usage task 后不检查 `Submit` 返回值，因此被丢弃的任务不会进入 `RecordUsage`，也不会调用 `usage_billing_dedup` 扣费。

影响：

- 高峰期或数据库短暂变慢时，成功请求可能完全不计量、不扣费。
- 这是对管理员最不利的缺陷，因为用户已拿到上游响应，平台没有任何账务记录。

修复策略：

- 将计费类 usage 任务的默认溢出策略改为 `sync`，或新增 `SubmitCritical`，队列满时必须同步执行。
- 对统计类、非扣费类任务可保留 sample/drop，但不得复用同一丢弃策略处理扣费任务。
- handler 在提交 usage task 后必须检查返回值；如果仍可能 dropped，需要至少记录结构化错误并触发告警。

验收：

- 构造 worker=1、queue=1、任务阻塞的测试，连续提交多条计费任务时，所有任务最终都执行扣费。
- `DroppedQueueFull` 对计费任务必须为 0；若出现必须有可观测告警。

### P0-2 订阅限额不是原子“检查并扣减”

证据：

- 请求前通过 middleware 和 `BillingCacheService.CheckBillingEligibility` 检查 `daily_usage >= daily_limit`。
- 转发成功后才在 `incrementUsageBillingSubscription` 中执行无条件 `daily_usage_usd = daily_usage_usd + cost`。
- SQL 没有 `daily_usage_usd + cost <= daily_limit_usd` 这样的条件，也没有预占额度。

影响：

- 当用户接近日限额时，多个并发请求都可能通过检查，随后全部成功扣到限额以上。
- 示例：日限额 50 USD，当前 49 USD，并发 20 个每个 2 USD 的请求，理论上可能最终扣到 89 USD。

修复策略：

- 引入订阅额度 reservation：请求进入上游前按保守估算预占，响应后按实际消耗结算，多退少补。
- 或在扣费事务中改为带限额条件的原子更新，并在超限时进入补偿策略：记账但标记 overage，立刻冻结订阅后续请求。
- 日/周/月限额都必须在同一个原子语义内处理，避免只守住日限额。

验收：

- 并发集成测试：同一订阅接近日限额，100 个并发请求不得让可继续使用额度超过策略允许的最大误差。
- 超限后 Redis billing cache 和 DB 状态一致，下一请求稳定返回 `USAGE_LIMIT_EXCEEDED`。

### P0-3 定价失败静默变成 0 费用

证据：

- `GatewayService.calculateTokenCost` 遇到 `CalculateCost` error 时返回 `&CostBreakdown{ActualCost: 0}`。
- `OpenAIGatewayService.RecordUsage` 遇到计价 error 时也设置 `CostBreakdown{ActualCost: 0}`。

影响：

- 新模型、模型映射错误、定价服务异常或渠道配置缺失时，用户可能成功调用但不扣费。
- 远程最近 500 条未发现该现象，但这是代码层面的 fail-open 设计。

修复策略：

- 对扣费路径改为 fail-closed：计价失败时不应写 0 元 usage 并放行账务。
- 已经返回给用户的请求不能撤回时，应至少按保守默认价格扣费，并标记 `billing_estimated=true` 或 `pricing_fallback_reason`。
- 管理后台提供“计价失败/保守计价”审计列表，便于补差。

验收：

- 未知模型非零 token 的单元测试不得产生 `actual_cost=0` 的 usage。
- 计价失败必须产生告警或审计记录。

## P1 问题

### P1-1 OpenAI Images usage 缺失时不按图片数量兜底扣费

证据：

- OpenAI Images forward result 会携带 `ImageCount`、`ImageSize`。
- `OpenAIGatewayService.RecordUsage` 的早返回只检查 token/cache 字段，不检查 `ImageCount`。
- usage log 构建也未持久化 `ImageCount/ImageSize`。

影响：

- 如果上游图片接口成功返回图片但不返回 usage，整次图片调用会被跳过计量。

修复策略：

- `OpenAIGatewayService.RecordUsage` 识别 `ImageCount > 0`，即使 token usage 为 0，也应按图片数量和尺寸使用 `CalculateImageCost` 或渠道 image/per_request 定价扣费。
- usage log 必须持久化 `image_count`、`image_size`、`billing_mode=image`。

验收：

- 构造 OpenAI Images 成功响应无 usage、有 `data[]` 的测试，必须产生非零 `actual_cost` 和 `image_count`。

### P1-2 OpenAI 流式缺少终止事件时已收集 usage 不会扣费

证据：

- `handleStreamingResponse` 在未看到 terminal event 时返回 `resultWithUsage()` 和 error。
- 调用方收到 error 后返回失败路径，不调用 `RecordUsage`。

影响：

- 如果上游已经返回过 usage 片段但连接在终止事件前异常，平台可能不扣这部分已知消耗。

修复策略：

- 引入 `ForwardPartialUsageError`，携带已收集 usage 和可计费标记。
- handler 对该错误执行一次幂等扣费，再按现有逻辑向客户端返回流错误。

验收：

- 流式测试模拟 message/usage 已到达但 terminal 缺失，必须记录 partial usage 并只扣一次。

## 实施任务

### Task 1: 计费任务不可丢

Files:

- Modify: `backend/internal/service/usage_record_worker_pool.go`
- Modify: `backend/internal/handler/gateway_handler.go`
- Modify: `backend/internal/handler/openai_gateway_handler.go`
- Test: `backend/internal/service/usage_record_worker_pool_test.go`
- Test: `backend/internal/handler/usage_record_submit_task_test.go`

步骤：

1. 新增 `SubmitCritical` 或把计费任务默认策略改为 `sync`。
2. handler 提交 usage task 后检查返回值。
3. 添加队列满场景测试，证明计费任务不会 dropped。

### Task 2: 订阅额度预占/原子扣减

Files:

- Modify: `backend/internal/service/billing_cache_service.go`
- Modify: `backend/internal/repository/usage_billing_repo.go`
- Modify: `backend/internal/service/gateway_service.go`
- Modify: `backend/internal/service/openai_gateway_service.go`
- Test: `backend/internal/repository/usage_billing_repo_integration_test.go`
- Test: `backend/internal/service/gateway_service_subscription_billing_test.go`

步骤：

1. 定义 reservation 方案和保守估算规则。
2. 请求前预占额度，失败则不转发上游。
3. 响应后用实际费用结算，多退少补。
4. 并发测试覆盖日/周/月限额。

### Task 3: 计价失败 fail-closed 或保守计价

Files:

- Modify: `backend/internal/service/gateway_service.go`
- Modify: `backend/internal/service/openai_gateway_service.go`
- Modify: `backend/internal/service/billing_service.go`
- Test: `backend/internal/service/gateway_record_usage_test.go`
- Test: `backend/internal/service/openai_gateway_record_usage_test.go`

步骤：

1. 把 `ActualCost=0` 静默回退改为显式错误或保守价格。
2. 增加审计字段或至少结构化日志。
3. 测试未知模型非零 token 不会免费。

### Task 4: OpenAI Images 兜底计费

Files:

- Modify: `backend/internal/service/openai_gateway_service.go`
- Modify: `backend/internal/service/openai_gateway_images.go`
- Test: `backend/internal/service/openai_gateway_record_usage_test.go`
- Test: `backend/internal/service/openai_gateway_images_test.go`

步骤：

1. `ImageCount > 0` 时跳过零 token 早返回。
2. 记录 `image_count/image_size`。
3. usage 缺失时按图片尺寸和数量兜底计费。

### Task 5: partial stream usage 扣费

Files:

- Modify: `backend/internal/service/openai_gateway_service.go`
- Modify: `backend/internal/handler/openai_chat_completions.go`
- Test: `backend/internal/service/openai_gateway_service_test.go`

步骤：

1. 定义携带 partial usage 的错误类型。
2. handler 识别该错误并执行幂等扣费。
3. 覆盖缺失 terminal event 但已有 usage 的场景。

## 验证计划

```bash
cd backend
go test -tags=unit ./internal/service ./internal/handler ./internal/repository
go test -tags=integration ./internal/repository
```

远程只读复核：

```bash
set -a; source remote.env; set +a
curl -fsS -H "x-api-key: $REMOTE_ADMIN_API_KEY" \
  "$REMOTE_BASE_URL/api/v1/admin/usage?page=1&page_size=500"

curl -fsS -H "x-api-key: $REMOTE_ADMIN_API_KEY" \
  "$REMOTE_BASE_URL/api/v1/admin/subscriptions?page=1&page_size=500"
```

复核重点：

- 最近 usage 中不应出现非零 token/image 但 `actual_cost=0`。
- active 订阅不得持续超过日/周/月限额。
- usage record worker pool 不得出现计费任务 dropped。
- 图片成功请求必须有 `image_count` 和非零扣费。

