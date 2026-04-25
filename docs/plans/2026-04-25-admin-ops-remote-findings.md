# `admin/ops` 远程运维态势排查记录

## 背景

- 排查时间：2026-04-25 23:50 左右（Asia/Shanghai）
- 排查对象：`https://api.benszresearch.com/admin/ops`
- 排查方式：基于根目录 `remote.env` 中的远程测试站点凭据，仅调用只读 Admin Ops API；未执行 retry、resolve、cleanup 或任何写操作
- 主要接口：`/api/v1/admin/ops/dashboard/snapshot-v2`、`/api/v1/admin/ops/account-availability`、`/api/v1/admin/ops/concurrency`、`/api/v1/admin/ops/request-errors`、`/api/v1/admin/ops/upstream-errors`、`/api/v1/admin/ops/alert-events`、`/api/v1/admin/ops/system-logs/health`

## 总体结论

当前 `admin/ops` 揭示的核心问题不是机器资源不够，也不是 Redis/DB 故障，而是**OpenAI 上游链路的稳定性与长尾延迟已经明显拖低用户可用性，同时告警与运维配置没有形成有效闭环**。

最突出的现象是：近 5 分钟健康分只有 `26`，成功率约 `68.12%`，错误率约 `31.88%`；近 1 小时有 `237` 次请求、`22` 次错误，全部是 `502` 上游失败，SLA 只有 `90.72%`。但同一时间应用侧 CPU 约 `10.6%`、内存约 `2.8%`、DB/Redis 均正常，并发仅 `3/58`，队列为 `0`。

换句话说，页面表现出来的是“平台很空，但用户体验差”。瓶颈主要在上游请求质量、账号限流、错误归因与告警配置。

## 关键发现

### 上游错误在短时间内集中爆发

近 1 小时错误分布非常集中：

| 指标 | 观测值 |
|------|--------|
| 请求总数 | `237` |
| 错误总数 | `22` |
| 错误状态码 | `502` 共 `22` 次 |
| 错误阶段 | `upstream` 共 `22` 次 |
| 错误级别 | `P1` 共 `22` 次 |
| 主要模型 | `gpt-5.5` 共 `21` 次，`gpt-5.4` 共 `1` 次 |
| 错误时间 | 主要集中在 `2026-04-25 23:41:42` 到 `23:42:51`（Asia/Shanghai） |

近 24 小时错误总量为 `114`，其中：

- `502`：`70` 次，是最大头
- `403`：`18` 次，标记为 business limited，不计入 SLA
- `401`：`17` 次
- `400`：`6` 次
- `507`：`2` 次
- `503`：`1` 次

请求错误归因显示近 24 小时有 `96` 个 SLA 错误，其中 `35` 个为 provider/upstream，`61` 个为 platform/gateway。Top message 包括：

- `Upstream request failed`
- `Upstream stream ended without a terminal response event`
- `Invalid API key`
- `{"detail":"Instructions are required"}`
- `exceeded request buffer limit while retrying upstream`

这里至少说明两类问题并存：一类是真上游失败，一类是网关侧把认证/请求体/重试缓冲等问题计入了 SLA 错误，需要进一步区分“用户输入错误、网关实现问题、上游不可用”。

### 延迟长尾已经远超告警阈值

近 1 小时成功请求的延迟分布非常重：

| 指标 | 观测值 |
|------|--------|
| 成功请求延迟直方图 | `219/219` 全部落在 `2000ms+` |
| duration p50 | `12.1s` 左右 |
| duration p95 | `34.2s` 左右 |
| duration p99 | `61.0s` 左右 |
| TTFT p50 | `2.0s` 左右 |
| TTFT p95 | `7.5s` 左右 |
| TTFT p99 | `10.6s` 左右 |

近 6 小时更明显：

- duration p95 约 `102.9s`
- duration p99 约 `131.3s`
- max duration 约 `269.2s`
- TTFT p99 约 `46.1s`
- max TTFT 约 `148.5s`

这说明即使请求最终成功，体验也已经不只是“慢一点”，而是有明显长尾。结合机器资源和并发都很低，长尾更像是上游模型/网络/流式终止行为导致，而不是本机排队。

### 账号池有局部限流，但不是整体并发打满

账号可用性显示：

- OpenAI 账号总数：`12`
- 可用账号：`10`
- 限流账号：`2`
- 错误账号：`0`
- 过载账号：`0`

两个不可用账号均处于 `active`，但被标记为 rate limited，剩余限流时间约 `600s` 和 `1110s`。它们归属 `GPT_Standard`。

并发侧则很空：

- OpenAI 当前并发：`3`
- 最大容量：`58`
- 平台负载：约 `5.17%`
- 等待队列：`0`
- 热点账号：`0`

这说明当前不是“请求太多导致平台排队”，而是上游账号/模型层面对部分请求产生限流或失败。短期应优先排查被限流账号的调度权重、冷却策略、模型可用性和是否存在少数用户/模型把 Standard 池打穿。

### 告警已触发，但通知闭环缺失

近 24 小时告警事件很多，且最新仍有 firing：

- `P0: 成功率过低`：当前值约 `68.12`，阈值 `95`
- `P1: 错误率过高`：当前值约 `31.88`，阈值 `5`
- 多次 `P0: 错误率极高` 曾触发并恢复

但所有事件的 `email_sent=false`。邮件告警配置显示：

- `alert.enabled=true`
- `recipients_count=0`

也就是说，系统“认为自己在告警”，但没有接收人，真实运维上等同于没有通知链路。

### 有两条延迟告警规则实际不会被评估

告警规则共有 `8` 条且全部启用，但心跳显示：

- `rules=8 enabled=8 evaluated=5`

代码对照显示，告警评估器支持的 dashboard 指标主要是：

- `success_rate`
- `error_rate`
- `upstream_error_rate`
- 以及 CPU、内存、账号可用性、限流、队列等指标

远程规则中存在：

- `p95_latency_ms`
- `p99_latency_ms`

这两个 metric type 不在当前评估器支持范围内，所以会被跳过。结果是：页面上看起来配置了 P95/P99 延迟告警，但实际不会生效。结合当前延迟长尾，这属于比较危险的“假安全感”。

### 运维数据维护配置不一致

当前设置显示：

- `ops_monitoring_enabled=true`
- `ops_realtime_monitoring_enabled=true`
- `cleanup_enabled=false`
- `aggregation_enabled=false`
- `auto_refresh_enabled=false`
- `ops_metrics_interval_seconds=60`

同时作业心跳里又能看到：

- `ops_metrics_collector` 正常
- `ops_alert_evaluator` 正常
- `ops_preaggregation_hourly` 正常
- `ops_preaggregation_daily` 正常
- `ops_cleanup` 每日执行但删除计数均为 `0`

这说明监控采集是开的，但清理与聚合策略没有形成一致的生产配置。系统日志健康目前还好：

- sink queue depth：`0/5000`
- dropped：`0`
- write failed：`0`
- written：`5372`
- avg write delay：约 `3ms`

但系统日志列表显示近 1 小时约 `865` 条、近 24 小时约 `12387` 条；如果 cleanup 长期关闭，后续会让运维表膨胀，增加 raw 查询成本。

### 错误归因字段还不够可操作

`upstream-errors` 里近 1 小时有 `23` 条，近 24 小时有 `44` 条，但聚合字段里：

- `upstream_status_code` 大量为 `null`
- `kind` 大量为 `null`
- 近期 `request-errors` 的 `upstream_error_events_count` 为 `0`

这会导致 ops 页面能告诉我们“上游失败了”，但很难直接告诉我们“哪个账号、哪个上游状态、哪个请求 ID、哪个网络错误类型最该处理”。对于 502 集中爆发这类问题，当前可观测性还差半步。

## 问题判断

### P0：真实用户可用性正在被上游失败拉低

证据是近 5 分钟成功率 `68.12%`、错误率 `31.88%`，且最新告警仍 firing。即使这是短时尖峰，也足以影响正在使用的用户。

建议动作：

- 先按 `gpt-5.5`、`gpt-5.4` 分模型查看最近 502 的账号分布
- 检查对应账号的 base_url、代理、上游凭据、模型映射与流式终止行为
- 对连续 502 的账号增加临时降权或短期摘除
- 对 `Upstream stream ended without a terminal response event` 建专门分类，避免只看到泛化的 `502`

### P1：延迟长尾严重，但告警没有覆盖

当前 P95/P99 延迟已经远超页面阈值，但对应规则不被评估。用户感知会是“成功但很慢”，而运维不会收到延迟类告警。

建议动作：

- 修复 `p95_latency_ms`、`p99_latency_ms` 告警规则的评估支持，或把规则迁移到当前已支持的 metric type
- 给 TTFT 和 duration 分别配置阈值，避免只看错误率
- 将默认 `ttft_p99_ms_max=500` 重新校准；以当前上游模型形态看，`500ms` 可能过于理想化，容易让页面长期显示红色但不指导行动

### P1：告警通知没有接收人

告警已经触发，但 `recipients_count=0` 且 `email_sent=false`。这会让后台页面变成“事后看板”，而不是生产告警系统。

建议动作：

- 配置至少一个真实告警收件人或接入机器人 Webhook
- 设置 `min_severity`，至少 P0/P1 要出站
- 验证一次测试告警投递链路，并在 ops 页面显示“告警通知未配置”的显式风险提示

### P2：账号池局部限流需要调度层处理

当前两个 OpenAI 账号处于 rate limited，均在 `GPT_Standard`。虽然整体容量还有 `10/12` 可用、并发负载很低，但限流账号如果仍参与选择，会造成抖动。

建议动作：

- 核对限流账号在冷却期是否完全退出调度
- 在账号列表/ops 页面展示限流剩余时间和最近触发模型
- 对 Standard 分组单独看 24 小时 429/502/延迟趋势，确认是否需要扩容或调低权重

### P2：运维存储策略需要收敛

监控采集打开、cleanup 关闭、aggregation 关闭，但 preaggregation 作业仍在跑。短期没出问题，长期会造成 raw 查询越来越重。

建议动作：

- 明确生产策略：要么开启 cleanup，并设置合理保留期；要么明确这是短期排查模式
- 如果 ops 页面默认使用 `auto` 查询，建议开启 aggregation 或让页面清楚显示当前正在走 raw path
- 对 `http.access` 日志考虑采样或降噪，否则高频访问会迅速撑大系统日志表

## 后续排查清单

- 拉取最近 `22` 条 502 的详情，按 `account_id`、`base_url`、`proxy`、`upstream_request_id` 聚合，确认是否集中在单账号或单上游。
- 对 `gpt-5.5` 做近 24 小时模型级错误率与延迟分位数，判断是否需要单独调度策略。
- 修复或迁移 `p95_latency_ms`、`p99_latency_ms` 规则，并增加对应单元测试，确保心跳里的 `evaluated` 数量等于应评估规则数。
- 配置告警收件人后做一次端到端测试，确认 `email_sent=true` 会落到事件。
- 评估 `cleanup_enabled=false` 是否只是临时排查配置；如果不是，建议开启并保留 30 天错误日志、7 到 14 天高频系统日志。

