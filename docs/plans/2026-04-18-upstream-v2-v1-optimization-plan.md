# Upstream `ad64190..6c73b621` 选择性吸收优化计划

> **For Claude:** 本计划是“只规划、不直接改源码”的执行蓝图。后续实施时必须按主题逐项移植、逐项验证，禁止整段 `cherry-pick` 或整体追平上游。

**Goal:** 基于上游 `Wei-Shaw/sub2api` 在 `ad64190bec3605f97e9e1805a0118aaa51f22b08..6c73b6212cee5bb78fb4a70ead7a4ab70ee6102b` 之间的演进，筛出对当前个人 fork 最值得吸收、但尚未稳定落地的优化项，并给出不破坏现有支付/订阅/仪表盘自定义能力的低风险实施路线。

**Architecture:** 当前仓库与上游的共同祖先正是 `ad64190...`，之后双方都独立演进。你的仓库已经自行吸收了 WebSearch、通知、退款控制、账号成本统计、盈利面板等大主题，因此本轮不建议“功能追平”，而是改为“补关键 correctness/stability 缺口”。优先吸收会影响调度稳定性、OpenAI/Anthropic 兼容性和运营治理的补丁；对已被本 fork 自定义改写很深的支付大重构与整套通知 UI 暂缓。

**Tech Stack:** Git、Go、Gin、Ent、Redis、PostgreSQL、Vue 3、TypeScript、pnpm

---

## 范围结论

- 上游这段区间共 **149** 个提交、**247** 个变更文件、约 **15378** 行新增与 **1866** 行删除。
- 改动重心以 `backend/` 为主，约占 **12533** 行；`frontend/` 约 **4661** 行。
- 当前 fork 已经具备以下上游主题的主体能力，无需重复“大搬运”：
  - WebSearch 管理与模拟：`backend/internal/pkg/websearch/manager.go`
  - 余额/额度通知：`backend/internal/service/balance_notify_service.go`
  - 渠道账号统计成本：`backend/internal/service/account_stats_pricing.go`
  - 按 provider 控制用户自助退款：`backend/internal/service/payment_config_service.go`
  - account cost / 盈利面板：`frontend/src/components/admin/usage/UsageStatsCards.vue`、`frontend/src/views/admin/DashboardView.vue`
- 当前最值得吸收的是“后期新增但本 fork 仍缺失或回退掉”的小而硬补丁，而不是再做一轮支付/通知大功能迁移。

## 上游变化摘要

### 已经基本被当前 fork 吸收的主题

- WebSearch 能力增强与管理端配置
- 余额/额度通知与通知邮箱结构化
- `allow_user_refund` 支付治理
- account cost 展示、账号统计成本与后台盈利观察

### 当前仍值得补的主题

- 调度快照稳定性与 WebSocket 标记保真
- OpenAI `messages -> responses` 兼容正确性
- 上游异常/KYC/限流场景的账号治理
- Claude `opus-4.7` 模型目录与计费映射
- 管理台测试弹窗在 SSE 期间的可关闭性

### 明确暂缓的主题

- 支付页整页交互重构
- 文档/赞助商/支付服务商推荐内容同步
- 已在当前 fork 上另起炉灶的支付与订阅前端重构

## 选择性吸收清单

### P0：必须优先吸收

#### 主题 1：调度快照稳定性与 WS 标志保真

**Why:** 这是网关稳定性的底座。当前 fork 相比上游缺少三类关键保护：调度缓存未保留 OpenAI WS 相关开关、outbox watermark 写入没有独立重试上下文、单批次 group rebuild 去重被移除，容易造成快照语义漂移、重复重建和 CPU 抖动。

**Upstream commits:**
- `3944b3d2` `fix: preserve openai ws flags in scheduler cache`
- `e44baa10` `fix: fix outbox watermark context expiry and add in-batch group rebuild dedup`
- `697c41a3` `fix: create fresh context per watermark write retry attempt`

**Files:**
- Modify: `backend/internal/repository/scheduler_cache.go`
- Modify: `backend/internal/service/scheduler_snapshot_service.go`
- Create: `backend/internal/repository/scheduler_cache_unit_test.go`
- Create: `backend/internal/service/openai_account_scheduler_ws_snapshot_test.go`

**Implementation notes:**
- 恢复 `filterSchedulerExtra(...)` 对下列字段的保留，避免调度缓存丢失 OpenAI WS 协议选择：
  - `openai_oauth_responses_websockets_v2_enabled`
  - `openai_oauth_responses_websockets_v2_mode`
  - `openai_apikey_responses_websockets_v2_enabled`
  - `openai_apikey_responses_websockets_v2_mode`
  - `responses_websockets_v2_enabled`
  - `openai_ws_enabled`
  - `openai_ws_force_http`
- 在 `pollOutbox()` 中恢复 watermark 写入的“每次重试独立 `context.WithTimeout(..., 5s)`”模式，并保留最多 3 次重试。
- 恢复单次 poll 批次内 `(groupID, platform)` 去重，避免同一批次反复 rebuild 相同 bucket。
- 保留你当前 fork 现有的容量池、仪表盘、聚合逻辑，不要回退其它与快照无关的自定义改动。

**Validation:**
- `cd backend && go test -tags=unit ./internal/repository -run SchedulerCache`
- `cd backend && go test -tags=unit ./internal/service -run 'Scheduler|OpenAI.*Snapshot'`
- 压测或模拟多个共享 group 的账号连续触发 `account_changed`，确认同批次 rebuild 次数下降、watermark 不再反复卡死。

#### 主题 2：OpenAI 兼容链路 correctness 补丁

**Why:** 这些补丁看似小，但会直接影响真实兼容请求是否稳定。当前 fork 仍缺失 `prompt_cache_key` 注入，且 Anthropic buffered 路径里少了“终态 output 为空时从 delta 重建输出”的兜底；另外上游把响应体超限处理收口为统一 helper，能减少同类错误分叉。

**Upstream commits:**
- `6c89d8d3` `add prompt_cache_key injection for messages→responses`
- `a1e299a3` `fix: Anthropic 非流式路径在上游终态事件 output 为空时从 delta 事件重建响应内容`
- `10699eeb` `refactor: extract ReadUpstreamResponseBody to deduplicate upstream response read + too-large error handling`

**Files:**
- Modify: `backend/internal/service/openai_gateway_messages.go`
- Modify: `backend/internal/service/upstream_response_limit.go`
- Modify: `backend/internal/service/gateway_service.go`
- Modify: `backend/internal/service/openai_gateway_service.go`
- Modify: `backend/internal/service/gemini_messages_compat_service.go`
- Modify/Test: `backend/internal/service/upstream_response_limit_test.go`
- Extend tests near: `backend/internal/service/openai_gateway_service_test.go`

**Implementation notes:**
- 为 API Key 路径的 `messages -> responses` 转换补上 `prompt_cache_key` 注入，确保兼容路径也能得到稳定 session identity。
- 在 `handleAnthropicBufferedStreamingResponse(...)` 恢复 `BufferedResponseAccumulator` 逻辑，避免终态 event 的空 output 导致非流式响应正文丢失。
- 在 `upstream_response_limit.go` 增加统一的 `ReadUpstreamResponseBody(...)`，把 `too large` 分支的 ops 标记与格式化错误输出收敛到一处。
- 逐个替换当前分散调用点，但不要顺手改动与本主题无关的 JSON 结构、日志字段和 billing 自定义。

**Validation:**
- `cd backend && go test -tags=unit ./internal/service -run 'UpstreamResponse|OpenAI|Anthropic'`
- 增加一个回归用例：Responses 终态 output 为空、但 delta 有内容时，Anthropic 非流式客户端仍能收到完整文本。
- 增加一个回归用例：Anthropic `/v1/messages` 走 OpenAI API Key 兼容路径时，请求体会自动带上 `prompt_cache_key`。

#### 主题 3：账号治理与限流/KYC 处理补丁

**Why:** 当前 fork 已经非常偏商用运营场景，这类账号状态治理比“新功能”更重要。上游在这段区间补了 KYC 身份验证要求时的停调度逻辑，以及 Codex 限流快照的回写/探测修复，收益高且风险可控。

**Upstream commits:**
- `5d586a9f` `fix: 上游返回 KYC 身份验证要求时停止账号调度`
- `7451b6f9` `修复 OpenAI 账号限流回流误判：7d 窗口可用时不因 5h 窗口为 0 回写 429`

**Files:**
- Modify: `backend/internal/service/ratelimit_service.go`
- Modify: `backend/internal/service/account_usage_service.go`
- Modify: `backend/internal/service/account_test_service.go`
- Modify: `backend/internal/service/openai_gateway_service.go`
- Modify/Test: `backend/internal/service/account_test_service_openai_test.go`
- Modify/Test: `backend/internal/service/openai_ws_ratelimit_signal_test.go`
- Extend tests near: `backend/internal/service/account_usage_service_test.go`

**Implementation notes:**
- 在 `HandleUpstreamError(...)` 的 400 分支中补上 `identity verification is required` 检测，并按永久不可调度处理。
- 对照上游的 Codex snapshot / resetAt 处理，把“429 与响应头快照”的回写逻辑统一到 `account_usage_service` 与测试账号探测流程。
- 审核当前 fork 的 `RateLimitResetAt` 更新路径，避免 5h 局部耗尽覆盖 7d 仍可用窗口的真实状态。
- 这部分只吸收状态判定与持久化逻辑，不要回退你已有的盈利计费、汇率或通知注入改造。

**Validation:**
- `cd backend && go test -tags=unit ./internal/service -run 'RateLimit|Codex|AccountTestService'`
- 构造 KYC 错误 body，确认账号会被标记为 auth error / 不再参与调度。
- 构造仅 5h 窗口触顶、7d 窗口仍可用的 header 快照，确认不会误写长期 429。

### P1：建议吸收

#### 主题 4：Claude `opus-4.7` 模型目录与计费映射

**Why:** 这是低耦合的产品支持项。当前 fork 的 Claude 模型常量里还没有 `claude-opus-4-7`，如果你的站点仍面向 Claude Code / Claude OAuth / Antigravity 兼容场景，这会直接影响模型可见性、映射与定价链路。

**Upstream commit:**
- `a789c8c4` `feat: 支持opus-4.7`

**Files:**
- Modify: `backend/internal/pkg/claude/constants.go`
- Modify: `backend/internal/domain/constants.go`
- Modify: `backend/internal/pkg/antigravity/claude_types.go`
- Modify: `backend/internal/pkg/antigravity/request_transformer.go`
- Modify: `backend/internal/service/pricing_service.go`
- Modify: `backend/internal/service/billing_service.go`
- Modify: `frontend/src/composables/useModelWhitelist.ts`

**Implementation notes:**
- 先核对你当前 fork 的模型命名策略与价格来源，避免只把模型名加进白名单却没有价格映射。
- 如果你当前运营中根本不提供 `opus-4.7`，可以把它降级为“条件吸收”：只同步常量与 transformer，不开放前台选择。
- 若吸收，必须同步检查模型显示、定价、请求归一化和计费归档是否一致。

**Validation:**
- `cd backend && go test -tags=unit ./internal/service ./internal/pkg/...`
- `cd frontend && pnpm run typecheck`
- 冒烟验证 Claude 相关模型列表、选型白名单与 billing model 归一化结果。

#### 主题 5：账号测试弹窗在 SSE 期间允许关闭

**Why:** 这不是大功能，但会明显改善管理员排障体验。当前 fork 相比上游缺少该修复，仍可能在连通性测试时让弹窗进入不可关闭或取消不彻底的状态。

**Upstream commit:**
- `38c00872` `fix(ui): allow closing test dialog during active SSE stream`

**Files:**
- Modify: `frontend/src/components/account/AccountTestModal.vue`
- Modify: `frontend/src/components/admin/account/AccountTestModal.vue`

**Implementation notes:**
- 恢复可取消的流式请求控制，不要让“Close”按钮在 `connecting` 时被永久禁掉。
- 明确“关闭弹窗”和“停止测试”是同一条中断链路，避免 zombie fetch / 状态残留。
- 这两个文件是镜像实现，必须同时改，避免一个入口修好、另一个入口继续卡住。

**Validation:**
- `cd frontend && pnpm run typecheck`
- 手工验证：测试开始后立即关闭弹窗，确认 UI 能退出且不会继续刷日志。

### P2：可以顺手整理，但不必阻塞本轮

#### 主题 6：统一上游异常读取与错误格式回写

**Why:** 这是可维护性收益，不是第一优先级。若本轮已经在主题 2 中动到多个 call site，可以一并收口；若改动面扩大，则允许拆到下一轮。

**Files:**
- Review around: `backend/internal/service/gateway_service.go`
- Review around: `backend/internal/service/openai_gateway_service.go`
- Review around: `backend/internal/service/gemini_messages_compat_service.go`

**Completion bar:**
- 所有非流式上游 body 读取统一经 `ReadUpstreamResponseBody(...)`
- `too large` 分支的客户端错误格式不再各自实现

## 不建议本轮吸收的内容

- 上游支付服务商推荐文档与 sponsor 变更
- 大块支付页面重构与移动端支付改版
- 当前 fork 已单独演进的公开套餐页、订阅补差价、盈利面板、容量推荐和首页体系

## 实施顺序

### 阶段 1：建立基线

- 固化当前分支回归基线，防止后续误把现有定制能力改坏。
- 必跑：
  - `cd backend && go test -tags=unit ./...`
  - `cd frontend && pnpm run typecheck`
  - `cd frontend && pnpm run lint:check`

### 阶段 2：先做 P0 的调度与兼容性补丁

- 先做“调度快照稳定性”
- 再做“messages/responses correctness”
- 最后做“KYC/限流治理”

### 阶段 3：再做 P1 的模型与前端体验补丁

- `opus-4.7` 支持
- 账号测试弹窗可关闭

### 阶段 4：收口与验收

- 补所有新增回归测试
- 对 OpenAI / Anthropic / WebSocket / 调度路径做一次最小冒烟
- 只要任一主题回归失败，就不要继续向下叠加更多上游补丁

## 成功标准

- 不破坏当前 fork 已有的支付、订阅、盈利、容量推荐与首页自定义能力
- OpenAI/Anthropic 兼容路径在 `messages`、非流式 buffered、WS 调度和限流场景下更稳
- 调度 outbox 不再因 watermark 写入失败反复重放同一批事件
- 管理员账号测试弹窗在长连接期间可随时退出
- 上游需要的补丁被“按主题最小吸收”，而不是把当前 fork 拉回上游形态

## 备注

- 本计划刻意没有把 WebSearch、通知、支付大重构继续列为本轮目标，因为这些主题在当前 fork 中已经有自定义落地，重复同步只会增大冲突面。
- 如果后续要真正实施，建议按主题分别提交，提交粒度控制在“一个主题一个 commit/PR”。
