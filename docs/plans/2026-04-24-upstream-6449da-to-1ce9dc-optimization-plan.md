# Upstream `6449da6c..1ce9dc03` 选择性吸收优化计划

> **For Codex / Claude:** 本文档只负责规划与取舍，不直接修改业务源码。后续实施应按主题拆包、分批验证，禁止把当前 fork 直接追平上游。

**Goal:** 基于上游 `Wei-Shaw/sub2api` 在 `6449da6c8daf2a443854cf25de96f3a972e3297c..1ce9dc03f9d15e8a633dafc0e5f1bbf5ac1e179a` 之间的演进，梳理哪些变化值得当前个人 fork 吸收，并沉淀为低风险、可验证的优化路线。

**Method:** 本轮按 `awesome-code` 工作流运行 `agent_coordinator.py` 做任务拆解；`dispatch_gate.can_proceed = true`，无 required agent 阻塞。随后结合本地 `git log`、`git diff`、当前 fork 源码探针与 license hash 对比，按“建议吸收 / 条件吸收 / 暂缓吸收 / 不吸收 / 已吸收”分类。

## 范围结论

- 该区间共 **74** 个提交，其中 **63** 个非 merge 提交。
- 上游差异涉及约 **189** 个后端文件、**74** 个前端文件、**2** 个 docs 文件与 **1** 个 GitHub Actions 文件。
- 主要变化集中在：
  - Channel Monitor：管理员可配置的渠道健康监控、用户侧渠道状态页、请求模板、30 天原始保留与日聚合。
  - Available Channels：按渠道/平台聚合展示可用模型、倍率、订阅组与独占组信息。
  - RPM 限流：新增分组级、用户级、用户-分组覆盖级 RPM 限制及缓存。
  - OpenAI/Codex 兼容：图片生成桥接、Spark 模型限制提示、工具调用 ID 保留、图片响应保留、gpt-5.5 默认映射。
  - 错误与调度：OpenAI 403 先临时冷却，再按连续次数判坏号；修复配额跨越时调度快照入队。
  - 支付正确性：未知订单 webhook 返回 2xx，避免支付服务商无限重试；支付二维码 fallback 与后台提示优化。
  - 安全与资源保护：OpenAI 图片请求处理增加 `io.LimitReader`，降低 OOM 风险。
- 当前 fork 已独立吸收或具备相近能力的部分：
  - `gpt-5.5` 模型映射、计费兜底、前端白名单与 Codex 归一化已存在。
  - 多处上游响应读取已使用 `io.LimitReader`，但 OpenAI 图片链路尚不存在，需要在引入图片接口时继续沿用上限策略。
- 结论：
  - 当前 fork 与上游仍是深度分叉状态，**不建议整段 merge / rebase / 批量 cherry-pick**。
  - 本区间最值得吸收的是“运营可观测性 + 限流保护 + OpenAI/Codex 稳定性”三类能力。
  - 适合按 P0/P1/P2 分主题重写或选择性 cherry-pick，而不是一次性导入全部 Ent/迁移/前端页面。

## License 检查

- 上游 `LICENSE` 在 `version1=6449da6c8daf2a443854cf25de96f3a972e3297c` 与 `version2=1ce9dc03f9d15e8a633dafc0e5f1bbf5ac1e179a` 的 blob hash 相同：`153d416dc8d2d60076698ec3cbfce34d91436a03`。
- 该区间未新增、删除或修改 license 文件。
- 当前 fork 本轮**无需同步 license**。

## 上游变化摘要

### 主题 1：Channel Monitor 渠道健康监控

代表提交：

- `20a4e418` `feat(monitor): admin channel monitor MVP with SSRF protection and batch aggregation`
- `a1425b45` `feat(channel-monitor): redesign user dashboard as card grid`
- `7da51240` `feat(channel-monitor): add feature switch settings + fix extra_models save`
- `8cf83c98` `feat(channel-monitor): aggregate history to daily rollups + soft delete`
- `ef6ec8a1` `fix(channel-monitor): drop soft delete, refactor feature flag to declarative form`
- `a2964259` `feat(channel-monitor): request templates with snapshot apply + headers/body override`
- `a7415d4d` `feat(monitor): 30-day raw retention + timeline 4-tier style + CC template seed + JSON format button`
- `e1193212` `feat(monitor): switch headers input to key-value rows`
- `c2f9ad7a` `refactor(channel-monitor): event-driven scheduler + sidebar cleanup`
- `c46744f3` `refactor(channel-monitor): tighten runner lifecycle + add unit tests`
- `0dcc0e05` `feat(monitor): proportion-based overall status + reusable auto-refresh`
- `f7c8377a` `fix(monitor): remove UNAVAILABLE status, keep only OPERATIONAL/DEGRADED`

核心变化：

- 新增 `channel_monitors`、`channel_monitor_events`、`channel_monitor_daily_rollups`、`channel_monitor_request_templates` 等数据结构。
- 后端新增管理员 CRUD、用户只读状态、定时/事件驱动 runner、检查器、SSRF 防护、请求模板快照、历史聚合与清理。
- 前端新增管理端监控配置页、用户侧渠道状态页、卡片式状态、时间线、自动刷新、模板管理与请求体 JSON 格式化。
- 监控模板采用“应用即拷贝”的快照语义，模板更新不会静默改变已有监控，避免运行时不可追溯。

对当前 fork 的启发：

- 这是一套非常契合个人运营的能力：可以让用户自助判断哪些渠道、模型、平台当前可用，减少人工客服与盲目报障。
- 当前 fork 已做盈利/超售/支付等运营面板，但缺少“上游健康状态”这个关键可观测性维度。
- 上游实现已包含 SSRF 防护与 runner 生命周期测试，说明该主题虽大，但工程方向成熟。

建议：**P1 建议吸收，但不要一次性全量导入。**

### 主题 2：Available Channels 可用渠道聚合页

代表提交：

- `654cfb64` `feat(channels): add "Available Channels" aggregate view`
- `365ef1fd` `refactor(channels): consolidate pricing index, tighten types, polish DTOs`
- `375aefa2` `refactor(channels): centralize BillingModelSource normalization and exhaustive enum maps`
- `9ba42aa5` `feat(channels): gate available channels behind feature switch (backend)`
- `800802b8` `feat(channels): explode available channels by platform + apply platform theme`
- `3cdd5754` `feat(channels): aggregate by channel with platform sections + rowspan table`
- `ff4ef1b5` `feat(channels): themed model popover + group-badge with rate, subscription & exclusivity`
- `9dae6c7a` `feat(sidebar+groups): available-channels above channel-status; show rate for subscription groups`
- `25a50355` `fix(available-channels): description as own column, fixed table layout`
- `6cd7c605` `fix(channels): supported models = mapping ∪ pricing with global LiteLLM fallback`
- `1f81b779` `feat(settings): link feature toggles to their config pages`

核心变化：

- 后端聚合渠道、平台、模型、计费来源、倍率、订阅组、独占组等信息。
- 前端提供用户侧可用渠道表格，按平台分区并展示模型 popover、分组 badge、倍率与说明。
- 通过公开设置 feature flag 控制入口，SSR payload 注入修复 `available_channels_enabled` 与 `channel_monitor_enabled`。

对当前 fork 的启发：

- 当前 fork 的商业化能力更强，可用渠道页能降低用户理解成本，把“为什么某模型可用/不可用、哪个组能用、倍率多少”公开透明化。
- 该能力与 Channel Monitor 是互补关系：一个回答“有哪些可用”，一个回答“现在是否健康”。
- 当前 fork 已有复杂分组倍率、订阅、闲时动态计费与盈利逻辑，直接吸收上游 DTO 可能无法完整表达 fork 语义，需要按当前业务重写聚合口径。

建议：**P1 建议吸收，最好排在 Channel Monitor 后或并行做只读版。**

### 主题 3：RPM 限流模块优化

代表提交：

- `dc5d42ad` `feat(rpm): RPM 限流模块优化`
- `6b0cf466` `Merge pull request #1815 from james-6-23/feat_rpm`

核心变化：

- 新增分组级 `rpm_limit`、用户级 `rpm_limit`、用户-分组覆盖 `rpm_override`。
- 新增 Redis 缓存与本地缓存辅助，降低限流计算路径上的数据库压力。
- 管理端新增用户 RPM、分组 RPM 与分组专属用户 RPM 覆盖配置。
- 语义上支持 `NULL` 继承默认、`0` 不限制、正数按分钟限流。

对当前 fork 的启发：

- 这是本轮对当前 fork 最有直接收益的稳定性能力。
- 个人商业化网关通常更容易遇到少数用户突发请求拖垮上游账号池的问题；RPM 比余额/额度更适合做实时保护。
- 当前 fork 已有复杂计费与订阅体系，RPM 能作为“成本保护层”独立叠加，不必改变现有计费语义。

建议：**P0 优先吸收。**

### 主题 4：OpenAI/Codex 图片与工具调用兼容

代表提交：

- `00778dca` `fix openai image request handling`
- `0b85a8da` `fix: add io.LimitReader bounds to prevent OOM in image handling`
- `eea6f388` `使用codex的生图接口代替web2api`
- `5f418997` `fix: bridge codex image generation over responses`
- `ca204ddd` `fix(openai): preserve image outputs when text content serialization fails`
- `c4d496da` `fix(openai): handle codex spark model limitations`
- `959af1c8` `fix(openai): preserve codex tool call ids`
- `1ce9dc03` `Merge pull request #1895 from gaoren002/fix/codex-spark-limitations`

核心变化：

- OpenAI 图片请求处理补齐响应转换、请求体边界读取与输出保留。
- Codex OAuth/Responses 转换中新增图片生成桥接提示，兼容 local Codex 未暴露 `image_gen` namespace 的场景。
- 对 `gpt-5.3-codex-spark` 明确阻止图片输入/生图工具，并注入更准确的模型限制提示。
- 工具续链场景保留 tool call id，避免上下文断裂。

对当前 fork 的启发：

- 当前 fork 已有较多 Codex/gpt-5.5 兼容逻辑，但仍应逐项核对 Spark 限制、图片工具桥接和 tool call id 保留是否完全覆盖。
- 如果后续接入 OpenAI 图片接口，这组修复必须作为基础安全与兼容要求一起吸收，不能只做 happy path。
- 当前 fork 已有 `io.LimitReader` 模式，应把“所有读取上游/用户图片体都必须有上限”作为设计约束固化。

建议：**P0/P1 分拆吸收：Codex 稳定性 P0，OpenAI 图片完整接口 P1。**

### 主题 5：OpenAI 403 与配额跨越调度修复

代表提交：

- `11cf23da` `修改403逻辑: 先临时冷却，再根据连续次数决定是否判坏号`
- `bcf4aedc` `fix: 修复账户配额跨越时调度快照入队逻辑`
- `9e5a6351` `修复计费问题以及模型回显`
- `ef967d8f` `fix: 修复 golangci-lint 报告的 36 个问题`

核心变化：

- OpenAI 403 不再立即强判坏号，而是先做短时冷却，再按连续次数升级处置。
- 修复账号日/周配额跨越时调度快照缓存失效与入队问题。
- 修复计费与模型回显相关问题，并补充 lint 清理。

对当前 fork 的启发：

- 这组修复与当前 fork 的账号可调度性、粘性会话、额度与盈利计算高度相关。
- 403 分级冷却能减少上游偶发风控、区域限制、临时错误导致的误杀账号。
- 配额跨越缓存失效是典型隐蔽生产 bug，建议优先审计当前 fork 是否存在相同状态漂移。

建议：**P0 优先吸收或重写等价逻辑。**

### 主题 6：支付 webhook 与二维码 fallback

代表提交：

- `f35e9675` `fix payment qr fallback and admin guidance`
- `75e1b40f` `fix(payment): ack unknown-order webhooks with 2xx to stop provider retries`
- `d5dac84e` `test(payment): cover ErrOrderNotFound sentinel contract`

核心变化：

- 支付 provider 回调遇到未知订单时返回 2xx，避免上游支付平台持续重试造成噪音或封禁风险。
- 支付二维码 fallback 与后台配置提示更清晰。
- 用 sentinel error 明确未知订单契约，并补测试。

对当前 fork 的启发：

- 当前 fork 已深度定制支付/订阅，支付回调幂等、未知订单 ack 与错误分类属于高优先级正确性能力。
- 如果当前 fork 已有类似逻辑，也应补测试锁定“未知订单不触发履约但返回成功 ack”的契约。

建议：**P0 优先审计，必要时吸收。**

### 主题 7：版本、CI、文档与发布杂项

代表提交：

- `3fe4fd4c` `chore: add model gpt-5.5`
- `a22a5b9e` `chore: fix docker pull version tag in TG notification`
- `0a80ec80` `chore: sync VERSION to 0.1.116 [skip ci]`
- `d162604f` `chore: sync VERSION to 0.1.117 [skip ci]`
- `59290e39` `chore(channels): drop admin-side available channels view`
- `67518a59` `revert: remove fork-only changes from release sync`

核心变化：

- 同步上游版本号与发布通知细节。
- 移除部分上游临时/错误合入内容。
- `gpt-5.5` 已由当前 fork 独立实现，不需要重复吸收。

建议：**已吸收 / 不吸收。** 当前 fork 有自己的版本号与发布节奏，不应同步上游 `VERSION`。

## 当前 fork 差距判断

### 已确认当前 fork 尚未具备

- `backend/internal/service/channel_monitor_service.go`、`channel_monitor_runner.go`、`channel_monitor_ssrf.go` 等 Channel Monitor 核心文件不存在。
- `frontend/src/views/user/ChannelStatusView.vue`、`frontend/src/views/admin/ChannelMonitorView.vue` 等状态页不存在。
- `backend/internal/service/channel_available.go` 与 `frontend/src/views/user/AvailableChannelsView.vue` 不存在。
- `backend/internal/service/user_rpm_cache.go`、`openai_403_counter.go` 不存在。
- `backend/internal/service/openai_images_responses.go` 不存在，说明上游 OpenAI 图片响应桥接未整体吸收。

### 已确认当前 fork 已有或部分已有

- `gpt-5.5` 已在后端计费、Codex transform、模型映射、前端白名单与价格资源中出现。
- 当前 fork 已有多处 `io.LimitReader`，并已抽象 `upstream_response_limit.go`，后续引入图片接口时应复用该模式。
- 当前 fork 已有丰富支付 provider、订阅升级、盈利与超售逻辑，因此吸收上游支付补丁时必须以当前 fork 为主，避免覆盖个性化业务。

## 吸收优先级

### P0：直接影响稳定性、成本与支付正确性

1. **RPM 限流保护**
   - 增加分组级、用户级、用户-分组覆盖级 RPM。
   - 接入认证缓存/计费缓存/网关限流路径，避免每次请求查库。
   - 管理端补最小配置入口与明确语义：空值继承、0 不限制、正数限流。

2. **OpenAI 403 分级冷却与连续计数**
   - 将“一次 403 即坏号”的策略改为短时冷却 + 连续阈值升级。
   - 计数器应按账号维度缓存，成功请求后清零。
   - 保留管理员可理解的状态说明，避免“临时冷却”和“坏号”混淆。

3. **配额跨越缓存失效审计**
   - 核对日/周/月配额跨界时，调度快照、账号可调度性、粘性会话与认证缓存是否同步失效。
   - 为跨日/跨周边界补单元测试或集成测试。

4. **支付未知订单 webhook ack 契约**
   - 未知订单应记录告警，不履约，但对支付平台返回 2xx。
   - 用 sentinel error 或显式错误类型区分“未知订单”和“真实处理失败”。
   - 补测试防止未来误改为 4xx/5xx 导致 provider 重试风暴。

5. **Codex 工具调用与 Spark 限制稳定性**
   - 审计当前 fork 是否保留 tool call id、item reference、图片输出与文本序列化失败时的非文本内容。
   - 对 Spark 模型图片输入/生图工具给出明确阻断与错误提示。

### P1：提升运营透明度与用户体验

1. **Channel Monitor 最小闭环**
   - 先实现后端只读/管理端最小 CRUD、手动检查、SSRF 防护、最近状态查询。
   - 再实现用户侧状态页、自动刷新、历史时间线与日聚合。
   - 请求模板与 Claude Code 伪装模板应作为第二阶段，避免首版过大。

2. **Available Channels 只读页**
   - 先做后端聚合 DTO 与用户侧只读表格。
   - 聚合口径必须包含当前 fork 的订阅、倍率、闲时计费与独占组语义。
   - 用 feature flag 控制入口，默认关闭或仅管理员确认后打开。

3. **OpenAI 图片接口完整接入**
   - 如果需要对外提供 `/v1/images/generations` 或 Responses image generation，再引入完整 handler/service/计费链路。
   - 必须同步接入请求体读取上限、输出计费、错误保留和模型能力限制。

4. **支付二维码 fallback 与后台提示**
   - 在不改支付核心链路的前提下改善管理员配置提示、用户端 fallback 文案与失败恢复路径。

### P2：暂缓或仅参考

1. **上游 Channel Monitor 全量 UI 视觉追平**
   - 当前 fork 的产品定位与页面风格已有自定义，不需要逐像素追上游。

2. **上游 Available Channels 管理端移除/侧边栏排序**
   - 侧边栏位置可按当前 fork 信息架构决定，不必照搬。

3. **上游版本号与发布通知**
   - 当前 fork 使用自己的版本节奏，不同步上游 `backend/cmd/server/VERSION`。

## 推荐实施路线

### 阶段 1：稳定性补丁优先

目标：不新增大 UI，不引入大表，先降低生产风险。

任务：

- 审计并实现 OpenAI 403 连续计数与临时冷却。
- 审计配额跨界缓存失效路径。
- 补支付未知订单 webhook ack 契约测试。
- 审计 Codex tool call id / Spark 图片限制 / 图片输出保留。

验收：

- 后端相关单元测试通过。
- 未改变现有正常支付履约、订阅计费和账号调度行为。
- 管理端账号状态能区分临时冷却与正式不可用。

### 阶段 2：RPM 限流能力

目标：给商业化网关增加实时成本保护层。

任务：

- 设计当前 fork 的 RPM 数据结构与迁移编号，避免与现有 `111` 后续迁移冲突。
- 增加分组、用户、用户-分组覆盖三层 RPM 语义。
- 接入网关请求入口与缓存失效机制。
- 前端只补必要字段与弹窗，不重做页面结构。

验收：

- 用户级、分组级、覆盖级限流优先级清晰。
- Redis 不可用时行为明确：保守拒绝或降级到本地缓存需文档化。
- RPM 不影响余额不足、套餐额度、闲时计费等既有判断。

### 阶段 3：可用渠道与状态透明化

目标：减少用户报障和人工解释成本。

任务：

- 先实现 Available Channels 只读聚合，暴露当前 fork 的模型、渠道、倍率、订阅组、闲时策略摘要。
- 再实现 Channel Monitor MVP：管理端配置、手动检查、SSRF 防护、用户状态页。
- 最后补历史聚合、请求模板、自动刷新、Claude Code 模板等高级能力。

验收：

- 所有公开入口受 feature flag 控制。
- 不暴露上游账号密钥、真实 base URL、内部错误堆栈。
- 状态页和可用渠道页的数据口径与实际调度/计费一致。

### 阶段 4：OpenAI 图片能力

目标：只有在明确要提供图片生成/编辑能力时实施。

任务：

- 引入 OpenAI 图片 handler/service 与计费路径。
- 复用当前 fork 的上游响应读取上限和价格兜底体系。
- 对 Codex image generation bridge、Spark 限制、图片输出保留补测试。

验收：

- 大图请求不会造成 OOM。
- 图片 token/张数计费与用量记录可追溯。
- 非图片模型请求不受影响。

## 风险与约束

- **迁移编号冲突：** 上游从 `125` 开始，本 fork 当前迁移到 `111`，后续实施不能直接复制编号，应按当前 fork 最新编号递增。
- **Ent 生成代码体量大：** Channel Monitor 与 RPM 都涉及 Ent Schema，实施时必须 `go generate ./ent` 并提交生成文件。
- **业务语义冲突：** 当前 fork 的订阅、闲时计费、盈利统计与支付逻辑已深度定制，不能直接覆盖上游 service。
- **公开信息泄露：** Available Channels 与 Channel Monitor 面向用户展示时，必须只暴露抽象渠道与模型能力，不泄露内部账号、密钥、成本或真实上游地址。
- **调度一致性：** RPM、403 冷却、配额跨界与 channel status 都会影响“可调度性”认知，实施时应明确哪个状态只展示、哪个状态真实影响调度。

## 不建议吸收的内容

- 上游 `VERSION` 同步提交与 release 节奏。
- 上游临时 revert / fork-only cleanup 类提交。
- 与当前 fork README、支付文档、品牌定位冲突的文档还原类变化。
- 未经过当前 fork 业务语义重写的 Available Channels DTO 与前端表格。

## 最终建议

- **必须吸收：** RPM 限流、403 分级冷却、配额跨界缓存失效、支付未知订单 ack、Codex 工具调用/模型限制稳定性。
- **建议吸收：** Channel Monitor MVP、Available Channels 只读页、OpenAI 图片接口安全基线。
- **谨慎吸收：** 请求模板、Claude Code 伪装模板、完整历史聚合与复杂 UI。
- **无需吸收：** license、上游版本号、发布通知和纯上游仓库治理杂项。

