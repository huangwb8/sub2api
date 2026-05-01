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

- 当前 `usage_billing_dedup` 通过 `(request_id, api_key_id)` 实现幂等，避免同一请求重复扣费。
- `UsageBillingRepository.Apply` 在单个数据库事务里执行订阅用量、余额、API Key quota、API Key rate limit、账号 quota 更新。
- usage log 写入在扣费后执行，避免“写日志成功但扣费失败”被误认为已经完成。
- Redis billing cache 在扣费后异步更新，并且关键缓存更新有同步回退。

主要缺口：

- 使用量记录任务可以在队列满时被丢弃；即使任务进入 worker，执行超时或 DB 短暂异常后也只有日志，没有持久化重试，导致成功请求可能最终不扣费。
- 当前“请求成功返回客户端”和“usage task 真正入库/执行”之间存在 crash window，只依赖内存 worker 无法满足“成功请求不得永久漏扣”。
- 余额模式和订阅模式都采用请求前检查、请求后扣减/累加，缺少请求前原子预占和请求后原子结算，高并发下可能余额透支或订阅限额明显超卖。
- 计价失败会回退为 `ActualCost=0` 并继续写 usage，未知模型或定价解析异常会形成免费调用。
- OpenAI Images 成功返回但 usage 缺失时，当前 `OpenAIGatewayService.RecordUsage` 会直接跳过记录，未使用 `ImageCount/ImageSize` 做按次兜底；流式 Images 还未稳定携带图片元数据。
- OpenAI、Anthropic 等流式响应在已收集部分 usage 但缺少终止事件时返回 error，handler 不会执行扣费，存在部分消耗漏扣风险。
- `partial usage` 与 `terminal usage` 目前没有升级协议，直接复用现有 `request_fingerprint` 语义会触发冲突，而不是安全地“同账单升级”。

## 账务硬不变量

后续实现必须同时满足以下不变量，任何方案如果无法证明这些条件，都不能进入主链路：

- **任何可计费请求在被视为“已成功返回客户端”之前，必须已经拥有可恢复的持久化账务锚点；不能只存在于内存队列或 goroutine 中。**
- **账务主键必须由服务端生成并控制，例如 `billing_request_id`；客户端传入的 request id、上游 request id 只能作为审计关联字段，不得直接作为最终扣费幂等主键。**
- **逻辑结算主键是 `(billing_request_id, api_key_id)`；`request_fingerprint` 仅用于冲突检测和审计，不得把合法的 partial -> terminal 升级误判为第二笔账。**
- **同一逻辑请求最多完成一次最终结算；partial usage、retry usage、terminal usage 只能升级同一账务实体，不能并列生成多笔账单。**
- **余额模式不得无条件扣成负数；订阅模式不得让下一请求继续通过已超限订阅。**
- **预占不足或补扣失败时，不得把差额静默免单；必须冻结后续可计费请求或进入应收/人工审核状态，直到管理员确认处理。**
- **释放预占只能发生在可证明未触达上游、上游明确未计费、或已确认无可计费 usage 的场景；只要上游可能已经消耗资源，就不得直接释放为免费。**
- **reservation / billing intent 必须具备状态机：`created -> reserved -> response_observed -> settlement_pending -> settled | released | expired | failed_retryable | failed_terminal | manual_review`，并具备后台回收和重试。**
- **`failed_terminal` 不等于免单；它表示自动结算已无法安全完成，必须保留管理员可见的处置入口。**
- **`expired` 不得自动等同于释放预占；如果请求可能触达上游或可能已产生成本，过期后必须进入重试或人工审核。**
- **usage log 只是审计展示，不是扣费成功的唯一凭证；扣费状态必须由独立账务表表达。**
- **所有 fail-open 路径必须显式证明“不扣费对管理员无损”；证明不了时默认 fail-closed、保守计费或进入人工审核。**

## 核心设计决策

### 管理员利益优先但不过度扣费

- 管理员利益优先的含义是“不免费、不漏扣、不让明显超限继续扩大损失”，不是对所有失败请求无条件扣费。
- 只有当系统能证明请求未触达上游、上游明确未产生计费资源、或没有任何可计费用量时，才能释放 reservation。
- 当系统无法证明是否产生上游成本时，进入 `billing_estimated`、`failed_retryable` 或 `manual_review`，按保守价格、最低可解释价格或人工审核处理。
- 对用户不确定的扣费必须可解释、可追溯、可由管理员调整；但默认不能因为不确定而让管理员承担损失。

### 持久化账务锚点先于成功响应

- 对所有余额/订阅计费请求，在请求进入上游前创建或幂等 claim `billing_reservations`（或等价命名）的持久化账务锚点。
- 该锚点必须使用服务端生成的 `billing_request_id` 作为账务主键；客户端 request id、上游 request id、网关 trace id 只作为外部关联字段。
- 该锚点至少记录：`billing_request_id`、`client_request_id`、`upstream_request_id`、`api_key_id`、`request_payload_hash`、用户/账号/订阅身份、计费模式、预占金额、请求入口、状态、最近错误。
- 后续 worker、handler、定时恢复任务都围绕这个锚点推进，而不是把内存队列当作唯一事实来源。

### 账务表分层

- `billing_reservations`：请求级锚点和预占状态来源，负责限额原子控制、崩溃恢复、过期回收。
- `billing_attempts`：上游尝试级记录，负责 failover、账号切换、上游 request id、是否已向客户端输出、是否可能产生上游成本等事实留痕。
- `usage_billing_outbox`：结算工作队列，负责可重试执行与错误留痕；可以重放，但不能替代 reservation。
- `usage_billing_dedup`：最终扣费幂等栅栏，只保证“最终 effects 最多一次”，不承担 partial/terminal 合并编排。

### Failover 与上游尝试

- 一个客户端请求可能对应多个上游尝试；账务 reservation 表示用户侧逻辑请求，attempt 表示每次上游账号尝试。
- 未触达上游的 attempt 可以安全忽略；已触达但未向客户端输出且明确失败的 attempt 可按上游错误性质决定释放或审核。
- 只要 attempt 已向客户端输出首字节、已收到 usage、或无法排除上游已计费，就必须保留可计费证据，不得因后续 failover 或流错误直接释放。
- 最终正常情况下只对成功或已输出的主 attempt 结算；异常情况下可进入 manual review，但不能因为 failover 结构导致免费调用。

### Partial Usage 升级协议

- 当流式请求只拿到 partial usage 时，先把 reservation 标记为 `response_observed` 并写入 partial usage 快照，状态进入 `settlement_pending`。
- partial 不应长时间延迟扣费；只能在很短的 grace window 内等待 terminal usage 升级，窗口内若收到终态则覆盖 usage/cost 快照并进入最终结算。
- grace window 到期仍未收到 terminal usage，则按 partial usage 结算一次，并标记 `settled_partial` 或等价审计字段。
- 若后续又收到 terminal usage，只允许在“同 `(billing_request_id, api_key_id)` 且不可变身份字段一致”的前提下做单次 top-up/upgrade；若不可变字段不一致，必须进入冲突审计，不得静默第二次扣费。

## P0 问题

### P0-1 成功请求在崩溃、队列满或暂时性失败后可能永久漏扣

证据：

- `backend/internal/service/usage_record_worker_pool.go` 默认 `overflow_policy=sample`，队列满时只有部分任务同步执行，其余返回 `dropped`。
- 多条 handler 主链路在响应成功后才提交 usage task，且不检查 `Submit` 返回值，因此“客户端已拿到响应，但 usage task 未被持久化/未被执行”的窗口真实存在。
- worker 执行默认有超时，`RecordUsage` 内部扣费失败后 handler 只记录日志，没有持久化 pending 状态和后台重试。

影响：

- 高峰期、进程崩溃、数据库短暂抖动时，成功请求可能完全不计量、不扣费。
- 这是对管理员最不利的缺陷，因为用户已拿到上游响应，平台可能没有任何可恢复的账务记录。

修复策略：

- 不再把 `SubmitCritical` 当成唯一保障；真正的正确性保障是“请求进入上游前已持久化 reservation / billing intent”。
- `SubmitCritical` 仍保留，但只作为结算加速器：队列满时同步执行、pool stopped 时立刻写 outbox，不允许静默 drop。
- 新增 `billing_reservations` 作为请求级锚点；新增 `usage_billing_outbox` 作为结算重试队列，二者都要保存 `billing_request_id`、`api_key_id`、`request_payload_hash`、usage 快照、计价快照、状态、重试次数和最近错误。
- handler 在“成功返回客户端”前必须至少完成账务锚点创建；如果结算任务未执行，立即把锚点推进到 pending/outbox，由后台 worker 按幂等键重试 `UsageBillingRepository.Apply`。
- 新增启动恢复任务，扫描 `reserved | response_observed | settlement_pending | failed_retryable`，自动重试或回收。
- usage log 写入失败不得影响扣费；扣费失败不得只写日志，必须留下可重放的 reservation/outbox 记录。

验收：

- 构造 worker=1、queue=1、任务阻塞的测试，连续提交多条计费任务时，所有任务最终进入 `settled`、`released`、`failed_retryable` 或 `manual_review`，不允许永久消失。
- 模拟“客户端已成功收到响应，但进程在 usage task 执行前崩溃”，重启恢复后必须仍能找到 reservation 并完成一次结算、进入审核，或在可证明未计费时释放。
- 模拟 `UsageBillingRepository.Apply` 第一次失败、第二次成功，后台重试后只扣一次。
- `DroppedQueueFull` 对计费任务必须为 0；若出现必须有结构化告警和持久化 pending 记录。

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
- reservation 必须以 `(billing_request_id, api_key_id)` 幂等创建；同一请求重试不能重复预占。
- 预占必须与限额判断使用同一个原子 SQL 语义：余额保证 `balance - reserved_amount >= 0`，订阅保证 `usage + reserved_amount <= limit`，除非显式配置允许透支/超售。
- 响应后按实际费用结算，多退少补；只有可证明未触达上游、上游明确未计费或明确零消耗时才释放预占。
- 保守估算不能拍脑袋：必须按模型、端点、请求体 max_tokens / max_output_tokens、图片 `n/size`、渠道价格和管理员配置上限计算；无法得到合理上限时 fail-closed，不转发上游。
- 预占小于实际费用时，只允许在同一账单内 top-up；如果 top-up 因余额/限额不足失败，必须冻结用户/API Key/订阅后续可计费请求，并进入应收差额或人工审核队列，不能静默免单。
- 释放预占必须带 release reason；`client_disconnected`、`missing_terminal`、`unknown_usage` 不能单独作为释放理由，除非同时能证明上游未计费。
- reservation 结算完成后，再推进 Redis billing cache；缓存只是派生态，不能先于 DB 成为判定依据。
- 如果暂时采用“响应后原子扣减”过渡方案，必须在扣费事务中带限额条件，并在超限时立刻冻结订阅/API Key 后续请求，同时落 overage 审计，但这只能作为临时过渡，不能视为最终硬化完成。

验收：

- 并发集成测试：同一余额用户接近余额耗尽、同一订阅接近日/周/月限额时，100 个并发请求不得让余额低于允许透支下限，不得让超限订阅继续通过下一次请求。
- 预占金额大于实际金额时，多余部分必须被正确释放；预占金额小于实际金额时，只允许在同一逻辑账单内补扣一次。
- 补扣失败不得被视为结算成功；对应主体必须被阻断后续可计费请求，并在后台显示待处理差额。
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
- 已经返回给用户的请求不能撤回时，不得返回 `ActualCost=0`；应至少按保守默认价格、渠道 per-request 价格或明确配置的 fallback price 扣费，并标记 `billing_estimated=true`、`pricing_fallback_reason`。
- 对“保守计价后待补差”的请求提供后台审计列表，便于后续核对和人工干预。
- 未知模型与定价解析失败都要有结构化告警，且能够从日志或后台按 `billing_request_id`、客户端 request id 或上游 request id 追溯。

验收：

- 未知模型非零 token 的单元测试不得产生 `actual_cost=0` 的 usage。
- 计价失败必须产生告警、审计记录和可追溯的 fallback reason。

### P0-4 流式 partial usage 缺少终止事件时不扣费，且缺少升级协议

证据：

- OpenAI `handleStreamingResponse` 在未看到 terminal event 时返回 `resultWithUsage()` 和 error。
- Anthropic/OpenAI passthrough 流式路径也存在“已解析 usage，但缺少 terminal event 时返回 error”的分支。
- 调用方收到 error 后进入失败路径，不调用 `RecordUsage`。
- 现有 `request_fingerprint` 含 token/cost 等可变字段，直接拿 partial 和 terminal 分别结算会触发冲突，而不是安全升级。

影响：

- 如果上游已经返回过 usage 片段但连接在终止事件前异常，平台可能不扣这部分已知消耗。
- 如果后续又收到 terminal usage，缺少升级协议会导致冲突审计、重复扣费或永久挂起三选一。

修复策略：

- 引入统一 `ForwardPartialUsageError` 或等价接口，携带已收集 usage、请求模型、`billing_request_id`、客户端/上游 request id、request_payload_hash、是否已向客户端发送首字节、可计费标记和原始错误。
- handler 对该错误不再简单走失败路径，而是把 reservation 状态推进到 `response_observed/settlement_pending`，提交 critical 结算任务，再按现有逻辑向客户端返回流错误。
- partial usage 和 terminal usage 共享同一个逻辑账单；terminal 到达时只能升级现有 reservation/outbox 快照，不得新开第二个 settlement key。
- 增加 grace window：partial usage 先进入待升级状态，窗口内若收到 terminal usage 则覆盖快照并最终结算；超时后按 partial 结算一次。
- 只有当不可变身份字段不一致时才触发冲突审计；不能因为 token 数量变化本身就报 `request_fingerprint` 冲突。

验收：

- OpenAI Responses、OpenAI ChatCompletions、Anthropic Messages、Anthropic API Key passthrough 分别模拟 usage 已到达但 terminal 缺失，必须记录 partial usage 并最终扣一次。
- partial -> terminal 升级不得触发错误冲突；真正的不一致请求才允许触发冲突审计。
- partial usage 扣费失败时必须进入 pending retry，而不是只写日志。

### P0-5 OpenAI Images usage 缺失时不按图片数量兜底扣费

证据：

- OpenAI Images forward result 会携带 `ImageCount`、`ImageSize`，但 `OpenAIGatewayService.RecordUsage` 的早返回只检查 token/cache 字段，不检查 `ImageCount`。
- buffered Images 路径会解析 `ImageCount/ImageSize`，但流式 Images 路径尚未稳定把这些元数据带回计费链路。
- usage log 构建还未把图片兜底场景作为强制验收对象。

影响：

- 如果上游图片接口成功返回图片但不返回 usage，整次图片调用会被跳过计量。
- 如果是流式 Images，可能既没有 usage，也没有图片元数据回传，导致兜底逻辑根本没有输入。
- 图片接口单次成本可能高于普通文本请求，免费成功请求对管理员不利，因此不应放在 P1。

修复策略：

- `OpenAIGatewayService.RecordUsage` 识别 `ImageCount > 0`，即使 token usage 为 0，也应按图片数量和尺寸使用 `CalculateImageCost` 或渠道 image/per_request 定价扣费。
- buffered 和 streaming 两条 Images 路径都必须带回 `image_count`、`image_size`；必要时从 request body 的 `n/size` 兜底。
- usage log 必须持久化 `image_count`、`image_size`、`billing_mode=image`。
- 如果响应和请求都无法确定图片数量或尺寸，但已经确认上游成功返回图片，应进入 `billing_estimated/manual_review`，不得直接跳过计费。

验收：

- 构造 OpenAI Images 成功响应无 usage、有 `data[]` 的 buffered 测试，必须产生非零 `actual_cost` 和 `image_count`。
- 构造 streaming Images 成功输出但无 usage 的测试，必须仍然带回 `image_count/image_size` 并完成非零扣费。
- 构造图片成功但元数据缺失的异常样本，必须进入 estimated/manual review，而不是返回 `ActualCost=0` 或无账务记录。

### P0-6 客户端 request id 复用、failover 与释放策略可能破坏扣费正确性

证据：

- 当前 `resolveUsageBillingRequestID` 优先使用客户端 request id，再使用本地 request id，最后才使用上游 request id 或生成值。
- handler 的 failover loop 可能在同一客户端请求中尝试多个账号；若直接把账号身份放进 reservation 不可变字段，合法 failover 可能触发冲突。
- 释放策略如果只写“上游未成功、客户端断连且无可计费 usage 时释放预占”，但不定义如何证明“无可计费 usage”，会留下免费成功请求的空间。

影响：

- 恶意或异常客户端可以复用 request id，造成后续真实请求被误判为重复账单或冲突挂起。
- failover 尝试如果没有 attempt 级账务记录，可能在“上游已消耗但主请求失败/切换账号”时漏扣。
- release 条件过宽时，会把“不知道是否已计费”的请求变成免费请求，损害管理员利益。

修复策略：

- 在认证/入口层生成 `billing_request_id`，并贯穿 reservation、attempt、outbox、dedup、usage log；外部 request id 只能作为 `client_request_id/upstream_request_id`。
- 新增 `billing_attempts` 或等价结构，记录每次上游尝试的 account、platform、upstream request id、是否已触达上游、是否已向客户端输出、usage 快照和失败原因。
- failover 成功时，最终账单关联实际成功或已输出的 attempt；失败但可能产生上游成本的 attempt 进入 estimated/manual review，不得被覆盖消失。
- release 必须执行白名单判断：未发出上游请求、连接前失败、上游明确非计费错误、或上游明确返回零消耗。其它不确定状态进入 `failed_retryable/manual_review`。

验收：

- 同一 API Key 下复用相同客户端 request id 发起不同请求，不得静默去重为同一账单；必须生成不同 `billing_request_id` 或进入明确冲突审计。
- 同一逻辑请求发生 failover，未触达上游的 attempt 不扣费，已输出或有 usage 的 attempt 必须能追溯并结算/审核。
- `client_disconnected`、`missing_terminal`、`unknown_usage` 单独出现时不得自动 release；必须有明确未计费证据。

## P1 问题

### P1-1 管理员可观测性和人工处置闭环不足

影响：

- 仅有日志和后台 worker 状态不足以支撑核心账务运营；管理员需要知道哪些请求已估算、待重试、待补扣、冲突或需要人工确认。
- 没有人工处置入口时，系统可能在 `failed_retryable/failed_terminal/manual_review` 中长期积压，最终仍然变成事实漏扣。

修复策略：

- 增加管理员只读列表：reservation 状态、outbox 积压、estimated billing、manual review、补扣失败、释放记录、request id 冲突。
- 增加受控操作：重试结算、确认估算扣费、标记释放、冻结/解冻用户或 API Key、导出对账数据。
- 所有人工操作必须写审计日志，包含管理员身份、原状态、新状态、金额、原因和关联 request id。

验收：

- 管理员能按状态筛选并定位每一笔异常账务。
- 对 failed/manual review 记录执行重试、确认、释放或冻结后，状态机和审计日志一致。

## 实施任务

### Task 1: 持久化账务锚点与原子预占

Files:

- Modify: `backend/internal/service/billing_cache_service.go`
- Modify: `backend/internal/service/gateway_service.go`
- Modify: `backend/internal/service/openai_gateway_service.go`
- Modify: `backend/internal/service/usage_billing.go`
- Modify: `backend/internal/repository/usage_billing_repo.go`
- Add: `backend/migrations/{next}_create_billing_reservations.sql`
- Add: `backend/migrations/{next}_create_billing_attempts.sql`
- Test: `backend/internal/repository/usage_billing_repo_integration_test.go`
- Test: `backend/internal/service/gateway_service_subscription_billing_test.go`
- Test: `backend/internal/service/gateway_service_balance_billing_test.go`

步骤：

1. 定义 `billing_reservations` 表、`billing_attempts` 表、状态机、保守估算规则和过期回收策略。
2. 在认证/入口层生成服务端 `billing_request_id`，并把客户端 request id、上游 request id 作为审计字段保存。
3. 请求进入上游前创建或幂等 claim reservation，并在同一原子 SQL 中完成余额/订阅限额预占。
4. 将 reservation 标识透传到后续计费链路，结算与释放都围绕同一逻辑账单完成。
5. 对补扣失败执行冻结/应收/人工审核策略；release 必须要求明确未计费证据。
6. 添加余额、订阅日/周/月限额、重复 create/settle/release、预占大于/小于实际费用、补扣失败冻结、错误 release 拒绝的测试。

### Task 2: 结算 outbox、崩溃恢复与计费任务不可丢

Files:

- Modify: `backend/internal/service/usage_record_worker_pool.go`
- Modify: `backend/internal/service/usage_billing.go`
- Modify: `backend/internal/handler/gateway_handler.go`
- Modify: `backend/internal/handler/gateway_handler_responses.go`
- Modify: `backend/internal/handler/gateway_handler_chat_completions.go`
- Modify: `backend/internal/handler/openai_gateway_handler.go`
- Modify: `backend/internal/handler/openai_chat_completions.go`
- Modify: `backend/internal/handler/openai_images.go`
- Modify: `backend/internal/handler/gemini_v1beta_handler.go`
- Modify: `backend/internal/repository/usage_billing_repo.go`
- Add: `backend/migrations/{next}_create_usage_billing_outbox.sql`
- Test: `backend/internal/service/usage_record_worker_pool_test.go`
- Test: `backend/internal/handler/usage_record_submit_task_test.go`
- Test: `backend/internal/repository/usage_billing_outbox_integration_test.go`

步骤：

1. 新增 `SubmitCritical`，但明确其角色是加速器而不是唯一正确性保障。
2. handler 提交 usage task 后必须检查返回值；无法立即执行时写 outbox/pending，不能静默 drop。
3. 为所有可计费入口补齐 `billing_request_id` 与 `request_payload_hash` 传递，避免幂等保护强弱不一致。
4. 新增启动恢复任务，扫描 reservation/outbox 的可恢复状态并自动重试。
5. 添加队列满、worker 超时、DB 首次失败后重试成功、客户端成功后进程崩溃再恢复的测试。

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
3. 增加 `billing_estimated`、`pricing_fallback_reason`、`fallback_price_source`、`manual_review_required` 等审计字段、结构化日志和后台筛选入口。
4. 测试未知模型非零 token 不会免费，且 fallback 请求可追溯。

### Task 4: 全流式 partial usage 升级结算

Files:

- Modify: `backend/internal/service/usage_billing.go`
- Modify: `backend/internal/service/openai_gateway_service.go`
- Modify: `backend/internal/service/gateway_service.go`
- Modify: `backend/internal/handler/gateway_handler_responses.go`
- Modify: `backend/internal/handler/gateway_handler_chat_completions.go`
- Modify: `backend/internal/handler/openai_gateway_handler.go`
- Modify: `backend/internal/handler/openai_chat_completions.go`
- Modify: `backend/internal/repository/usage_billing_repo.go`
- Test: `backend/internal/service/openai_gateway_service_test.go`
- Test: `backend/internal/service/gateway_streaming_test.go`
- Test: `backend/internal/service/gateway_anthropic_apikey_passthrough_test.go`
- Test: `backend/internal/repository/usage_billing_partial_upgrade_integration_test.go`

步骤：

1. 定义携带 partial usage 的错误类型和 reservation/outbox 升级协议。
2. OpenAI、Anthropic、passthrough 流式路径统一包装 partial usage。
3. handler 识别该错误并推进 reservation 到 `settlement_pending`；grace window 必须很短，不能长时间延迟已知 usage 的保底结算。
4. 覆盖缺失 terminal event、partial -> terminal 升级、partial 超时结算、真正冲突请求审计、补扣失败冻结五类场景。

### Task 5: OpenAI Images 兜底计费

Files:

- Modify: `backend/internal/service/openai_gateway_service.go`
- Modify: `backend/internal/service/openai_gateway_images.go`
- Modify: `backend/internal/handler/openai_images.go`
- Test: `backend/internal/service/openai_gateway_record_usage_test.go`
- Test: `backend/internal/service/openai_gateway_images_test.go`

步骤：

1. `ImageCount > 0` 时跳过零 token 早返回。
2. buffered 与 streaming Images 路径统一记录 `image_count/image_size`。
3. usage 缺失时按图片尺寸和数量兜底计费。
4. 图片成功但无法确定数量/尺寸时进入 estimated/manual review，不允许无账务记录。
5. 覆盖 buffered、streaming、`n/size` request body 兜底、元数据缺失进入审核四类测试。

### Task 6: 管理员账务审计与人工处置

Files:

- Modify: `backend/internal/handler/admin/ops_handler.go`
- Modify: `backend/internal/repository/usage_billing_repo.go`
- Modify: `backend/internal/service/usage_billing.go`
- Test: `backend/internal/handler/admin/ops_handler_test.go`
- Test: `backend/internal/repository/usage_billing_admin_integration_test.go`

步骤：

1. 增加 reservation/outbox/estimated/manual review/补扣失败/释放记录的管理员查询入口。
2. 增加受控操作：重试结算、确认估算扣费、确认释放、冻结/解冻用户或 API Key、导出对账数据。
3. 所有人工操作写审计日志，保留管理员身份、金额、原因、旧状态、新状态和关联 request id。
4. 覆盖异常状态查询、重试成功、确认释放、冻结主体、审计日志一致性测试。

## 验证计划

```bash
cd backend
go test -tags=unit ./internal/service ./internal/handler ./internal/repository
go test -tags=integration ./internal/repository
go test -race -tags=unit ./internal/service ./internal/handler ./internal/repository
```

建议补充的故障注入验证：

- 在“成功返回客户端”后、worker 实际执行前主动 kill 进程，验证重启恢复。
- 对 partial usage 注入 terminal 延迟与永不到达两种情况，验证 grace window 行为。
- 对 reservation 预占金额设置高估/低估场景，验证 release、top-up、补扣失败冻结与应收/人工审核行为。
- 对同一客户端 request id 重复提交不同请求，验证服务端 `billing_request_id` 不被复用。
- 对 failover 多 attempt 场景注入“未触达上游”“已触达但无输出”“已输出后失败”，验证 release 白名单和 manual review 分流。
- 对 Images 成功但 usage/元数据缺失场景，验证不会静默免费。

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
- `billing_reservations` 不得长期停留在 `response_observed/settlement_pending/failed_retryable`。
- `failed_terminal/manual_review/billing_estimated` 必须有管理员可见入口、原因和处置状态。
- release 记录必须有明确未计费证据；不能只有 `client_disconnected/missing_terminal/unknown_usage`。
- `billing_attempts` 必须能追溯 failover 中每次上游尝试，且已输出或有 usage 的 attempt 不得消失。
- usage record worker pool 不得出现计费任务 dropped；outbox 不得长期积压。
- 图片成功请求必须有 `image_count` 和非零扣费。
- partial stream usage 必须有对应逻辑账单，且合法升级不得触发 `request_fingerprint` 冲突。
