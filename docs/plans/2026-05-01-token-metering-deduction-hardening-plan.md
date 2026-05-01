# Token 计量与扣费准确性硬化计划

## 背景

2026-05-01 基于本地代码审计和 `remote.env` 远程站点只读抽样，排查“用户实际 token 消耗是否可以被正确计量和扣除”。结论是：当前主链路已经具备 usage 解析、成本计算、幂等扣费和订阅用量累加能力，但扣费链路仍缺少“不可丢、可重试、可恢复、原子限额”的账务级保障，需要按核心业务优先级硬化。

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

- 使用量记录任务可以在队列满时被丢弃；即使任务进入 worker，执行超时或 DB 短暂异常后也只有日志，没有持久化重试，导致成功请求可能最终不扣费。
- 余额模式和订阅模式都采用请求前检查、请求后扣减/累加，缺少请求前原子预占和请求后原子结算，高并发下可能余额透支或订阅限额明显超卖。
- 计价失败会回退为 `ActualCost=0` 并继续写 usage，未知模型或定价解析异常会形成免费调用。
- OpenAI Images 成功返回但 usage 缺失时，当前 `OpenAIGatewayService.RecordUsage` 会直接跳过记录，未使用 `ImageCount/ImageSize` 做按次兜底。
- OpenAI、Anthropic 等流式响应在已收集部分 usage 但缺少终止事件时返回 error，handler 不会执行扣费，存在部分消耗漏扣风险。

## 账务硬不变量

后续实现必须同时满足以下不变量，任何方案如果无法证明这些条件，都不能进入主链路：

- **已成功获得上游响应的可计费请求，不得因为本地队列、请求取消、worker 超时、DB 短暂异常或 usage log 写入失败而永久漏扣。**
- **同一个 `(request_id, api_key_id, request_fingerprint)` 最多结算一次；重复结算、重试结算、partial usage 结算都必须复用同一幂等语义。**
- **余额模式不得无条件扣成负数；订阅模式不得让下一请求继续通过已超限订阅。**
- **reservation 必须有状态机：`reserved -> settled | released | expired | failed_retryable | failed_terminal`，并具备后台回收和重试。**
- **usage log 只是审计展示，不是扣费成功的唯一凭证；扣费状态必须由独立账务表或 outbox 表表达。**

## P0 问题

### P0-1 计费任务会丢失或失败后不可恢复

证据：

- `backend/internal/service/usage_record_worker_pool.go` 默认 `overflow_policy=sample`，队列满时只有部分任务同步执行，其余返回 `dropped`。
- gateway handler 提交 usage task 后不检查 `Submit` 返回值，因此被丢弃的任务不会进入 `RecordUsage`，也不会调用 `usage_billing_dedup` 扣费。
- worker 执行默认有超时，`RecordUsage` 内部扣费失败后 handler 只记录日志，没有持久化 pending 状态和后台重试。

影响：

- 高峰期或数据库短暂变慢时，成功请求可能完全不计量、不扣费。
- 这是对管理员最不利的缺陷，因为用户已拿到上游响应，平台可能没有任何可恢复的账务记录。

修复策略：

- 将计费类 usage 任务改为 `SubmitCritical`：队列满时同步执行；pool stopped 时不得静默 drop，必须转入持久化 outbox 或同步失败告警。
- 新增账务 outbox/pending 表，保存 `request_id`、`api_key_id`、`request_fingerprint`、usage 快照、计价快照、状态、重试次数和最近错误。
- handler 提交 critical task 后必须检查返回值；如果未执行，立即写 pending outbox，后台 worker 按幂等键重试 `UsageBillingRepository.Apply`。
- usage log 写入失败不得影响扣费；扣费失败不得只写日志，必须留下可重放的 pending 记录。

验收：

- 构造 worker=1、queue=1、任务阻塞的测试，连续提交多条计费任务时，所有任务最终进入 `applied` 或 pending retry 状态，不允许永久消失。
- 模拟 `UsageBillingRepository.Apply` 第一次失败、第二次成功，后台重试后只扣一次。
- `DroppedQueueFull` 对计费任务必须为 0；若出现必须有结构化告警和 pending 记录。

### P0-2 余额和订阅都不是原子“预占并结算”

证据：

- 请求前通过 middleware 和 `BillingCacheService.CheckBillingEligibility` 检查 `daily_usage >= daily_limit`。
- 转发成功后才在 `incrementUsageBillingSubscription` 中执行无条件 `daily_usage_usd = daily_usage_usd + cost`。
- 余额模式只检查 `balance > 0`，扣费时执行无条件 `balance = balance - amount`。
- SQL 没有 `daily_usage_usd + cost <= daily_limit_usd` 或 `balance - amount >= 0` 这样的条件，也没有请求前预占额度。

影响：

- 当用户接近日限额时，多个并发请求都可能通过检查，随后全部成功扣到限额以上。
- 示例：日限额 50 USD，当前 49 USD，并发 20 个每个 2 USD 的请求，理论上可能最终扣到 89 USD。
- 余额模式同理：余额 1 CNY 时，并发多个高成本请求都可能通过请求前检查，最终被扣成负数。

修复策略：

- 引入统一 reservation：请求进入上游前按保守估算预占，余额模式预占 CNY，订阅模式预占 USD 日/周/月额度。
- reservation 必须以 `request_id + api_key_id` 幂等创建；同一请求重试不能重复预占。
- 响应后用实际费用结算，多退少补；上游未成功、客户端断连且无可计费 usage 时释放预占。
- 日/周/月限额必须在同一个原子 SQL 语义内处理；余额预占必须保证 `balance - reserved_amount >= 0`，除非显式配置了允许透支。
- 如果暂时采用“响应后原子扣减”替代 reservation，必须在扣费事务中带限额条件，并在超限时立刻冻结订阅/API Key 后续请求，同时落 overage 审计。

验收：

- 并发集成测试：同一余额用户接近余额耗尽、同一订阅接近日/周/月限额时，100 个并发请求不得让余额低于允许透支下限，不得让超限订阅继续通过下一次请求。
- 超限后 Redis billing cache 和 DB 状态一致，下一请求稳定返回 `USAGE_LIMIT_EXCEEDED`。
- reservation 重放测试：同一请求重复 create/settle/release 不得重复预占、重复退款或重复扣费。

### P0-3 定价失败静默变成 0 费用

证据：

- `GatewayService.calculateTokenCost` 遇到 `CalculateCost` error 时返回 `&CostBreakdown{ActualCost: 0}`。
- `OpenAIGatewayService.RecordUsage` 遇到计价 error 时也设置 `CostBreakdown{ActualCost: 0}`。

影响：

- 新模型、模型映射错误、定价服务异常或渠道配置缺失时，用户可能成功调用但不扣费。
- 远程最近 500 条未发现该现象，但这是代码层面的 fail-open 设计。

修复策略：

- 请求进入上游前可识别的未知模型/无定价模型应 fail-closed，不转发上游。
- 已经返回给用户的请求不能撤回时，不得返回 `ActualCost=0`；应至少按保守默认价格或渠道 per-request 价格扣费，并标记 `billing_estimated=true` 或 `pricing_fallback_reason`。
- 管理后台提供“计价失败/保守计价”审计列表，便于补差。

验收：

- 未知模型非零 token 的单元测试不得产生 `actual_cost=0` 的 usage。
- 计价失败必须产生告警或审计记录。

### P0-4 流式 partial usage 缺少终止事件时不扣费

证据：

- OpenAI `handleStreamingResponse` 在未看到 terminal event 时返回 `resultWithUsage()` 和 error。
- Anthropic/OpenAI passthrough 流式路径也存在“已解析 usage，但缺少 terminal event 时返回 error”的分支。
- 调用方收到 error 后进入失败路径，不调用 `RecordUsage`。

影响：

- 如果上游已经返回过 usage 片段但连接在终止事件前异常，平台可能不扣这部分已知消耗。
- 这是流式请求的真实账务风险，不能只作为 OpenAI 单一路径修补。

修复策略：

- 引入统一 `ForwardPartialUsageError` 或等价接口，携带已收集 usage、请求模型、request_id、可计费标记和原始错误。
- handler 对该错误提交 critical billing task，再按现有逻辑向客户端返回流错误。
- partial usage 结算必须复用正常 usage 的幂等键；如果随后正常 terminal usage 也到达，二者必须因同一 `request_id` 幂等而只结算一次。

验收：

- OpenAI Responses、OpenAI ChatCompletions、Anthropic Messages、Anthropic API Key passthrough 分别模拟 usage 已到达但 terminal 缺失，必须记录 partial usage 并只扣一次。
- partial usage 扣费失败时必须进入 pending retry，而不是只写日志。

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

## 实施任务

### Task 1: 计费任务不可丢且失败可恢复

Files:

- Modify: `backend/internal/service/usage_record_worker_pool.go`
- Modify: `backend/internal/handler/gateway_handler.go`
- Modify: `backend/internal/handler/openai_gateway_handler.go`
- Modify: `backend/internal/handler/gemini_v1beta_handler.go`
- Modify: `backend/internal/repository/usage_billing_repo.go`
- Add: `backend/migrations/{next}_create_usage_billing_outbox.sql`
- Test: `backend/internal/service/usage_record_worker_pool_test.go`
- Test: `backend/internal/handler/usage_record_submit_task_test.go`
- Test: `backend/internal/repository/usage_billing_outbox_integration_test.go`

步骤：

1. 新增 `SubmitCritical`，计费任务队列满时同步执行。
2. 新增 usage billing outbox/pending 状态表和后台 retry worker。
3. handler 提交 usage task 后检查返回值，无法执行时写 pending outbox。
4. 添加队列满、worker 超时、DB 首次失败后重试成功的测试。

### Task 2: 余额和订阅统一 reservation/原子结算

Files:

- Modify: `backend/internal/service/billing_cache_service.go`
- Modify: `backend/internal/repository/usage_billing_repo.go`
- Modify: `backend/internal/service/gateway_service.go`
- Modify: `backend/internal/service/openai_gateway_service.go`
- Add: `backend/migrations/{next}_create_billing_reservations.sql`
- Test: `backend/internal/repository/usage_billing_repo_integration_test.go`
- Test: `backend/internal/service/gateway_service_subscription_billing_test.go`
- Test: `backend/internal/service/gateway_service_balance_billing_test.go`

步骤：

1. 定义 reservation 表、状态机、保守估算规则和过期回收策略。
2. 请求前预占余额或订阅额度，失败则不转发上游。
3. 响应后用实际费用结算，多退少补；无可计费 usage 时释放。
4. 将 DB 状态和 Redis billing cache 更新绑定到结算成功后的同一可观测流程。
5. 并发测试覆盖余额、订阅日/周/月限额、重复 create/settle/release。

### Task 3: 计价失败 fail-closed 或保守计价

Files:

- Modify: `backend/internal/service/gateway_service.go`
- Modify: `backend/internal/service/openai_gateway_service.go`
- Modify: `backend/internal/service/billing_service.go`
- Test: `backend/internal/service/gateway_record_usage_test.go`
- Test: `backend/internal/service/openai_gateway_record_usage_test.go`

步骤：

1. 把 `ActualCost=0` 静默回退改为显式错误或保守价格。
2. 请求前可识别的未知模型直接拒绝；请求后才发现的计价失败走保守计价。
3. 增加审计字段或至少结构化日志。
4. 测试未知模型非零 token 不会免费。

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

### Task 5: 全流式 partial usage 扣费

Files:

- Modify: `backend/internal/service/openai_gateway_service.go`
- Modify: `backend/internal/service/gateway_service.go`
- Modify: `backend/internal/handler/openai_chat_completions.go`
- Modify: `backend/internal/handler/openai_gateway_handler.go`
- Modify: `backend/internal/handler/gateway_handler.go`
- Test: `backend/internal/service/openai_gateway_service_test.go`
- Test: `backend/internal/service/gateway_streaming_test.go`
- Test: `backend/internal/service/gateway_anthropic_apikey_passthrough_test.go`

步骤：

1. 定义携带 partial usage 的错误类型。
2. OpenAI、Anthropic、passthrough 流式路径统一包装 partial usage。
3. handler 识别该错误并提交 critical 幂等扣费。
4. 覆盖缺失 terminal event 但已有 usage 的场景。

## 验证计划

```bash
cd backend
go test -tags=unit ./internal/service ./internal/handler ./internal/repository
go test -tags=integration ./internal/repository
go test -race -tags=unit ./internal/service ./internal/handler ./internal/repository
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
- 余额模式不得出现非配置允许的负余额。
- usage record worker pool 不得出现计费任务 dropped；pending outbox 不得长期积压。
- 图片成功请求必须有 `image_count` 和非零扣费。
- partial stream usage 必须有对应幂等扣费记录。
