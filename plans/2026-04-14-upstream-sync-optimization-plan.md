# Sub2API 上游同步优化计划

**Goal:** 在不直接改动当前业务源码的前提下，基于上游 `Wei-Shaw/sub2api` 自 `ad64190bec3605f97e9e1805a0118aaa51f22b08` 到 `7c671b5373b0d6bf60dc433e4bcbb56755fa9b4e` 的演进，筛出对当前 fork 最值得吸收的优化项，并给出低风险、可验证、可分阶段执行的落地路线。

**Architecture:** 当前仓库已经从共同祖先 `ad64190...` 明显分叉，且 `main` 上有自定义支付、通知、首页、扩容与仪表盘能力，因此不建议做整段 rebase 或批量 cherry-pick。推荐采用“按主题选择性吸收”的策略：先吸收正确性与稳定性改进，再吸收中等风险的网关与支付增强，最后评估通知与支付前端重构这类高耦合改动。

**Tech Stack:** Git、Go、Gin、Ent、PostgreSQL、Redis、Vue 3、TypeScript、pnpm

---

## 结论

- 上游最新提交不是 `ad64190bec3605f97e9e1805a0118aaa51f22b08`，而是 `7c671b5373b0d6bf60dc433e4bcbb56755fa9b4e`。
- 当前 fork 不适合直接“整体追平上游”，因为双方都已经在支付、通知、运维与前端上继续演化。
- 最值得吸收的是“正确性/稳定性补丁 + 中等范围的支付退款控制 + WebSearch 可用性增强”。
- 最应该暂缓的是“大体量支付页重构”和“整套余额/额度通知系统直接整体搬运”，因为这两块与你当前自定义能力耦合较深，误伤风险明显更高。

## 上游这段时间的主要变化

### 主题 A：WebSearch 模拟与调度增强

- 代表提交：
  - `1b53ffca` `feat(gateway): add web search emulation for Anthropic API Key accounts`
  - `fda61b06` `feat(websearch): proxy failover, timeout, quota-weighted load balancing`
  - `7c729293` `feat: websearch quota enhancements and balance notify hint`
  - `0a4ece5f` `fix: audit round-3 — proxy safety, intervals persistence, SMTP timeout, sort fix`
- 关键文件：
  - `backend/internal/pkg/websearch/manager.go`
  - `backend/internal/service/gateway_websearch_emulation.go`
  - `backend/internal/service/websearch_config.go`
  - `frontend/src/views/admin/SettingsView.vue`
  - `frontend/src/components/account/CreateAccountModal.vue`
  - `frontend/src/components/account/EditAccountModal.vue`
- 价值：
  - 代理不可用时能更稳地 failover
  - quota 权重调度更合理
  - 管理端配置与 reset usage 能力更完整
  - 对 Anthropic API Key 的 WebSearch 兼容更成熟

### 主题 B：支付与退款能力增强

- 代表提交：
  - `f1297a36` `feat: add per-provider allow_user_refund control and align wildcard matching`
  - `4aa0070e` `fix: Stripe payment type matching in load balancer`
  - `c738cfec` `fix(payment): critical audit fixes for security, idempotency and correctness`
  - `5240b444` `refactor(payment): inline payment flow, mobile support, renewal modal`
- 关键文件：
  - `backend/internal/service/payment_refund.go`
  - `backend/internal/service/payment_config_service.go`
  - `backend/internal/service/payment_config_providers.go`
  - `backend/internal/payment/load_balancer.go`
  - `frontend/src/views/user/PaymentView.vue`
  - `frontend/src/components/payment/*`
- 价值：
  - 退款权限粒度更细
  - 支付 provider 类型映射更稳
  - 移动端支付路径更完整
  - 通配符匹配与计费规则一致性更高

### 主题 C：通知系统扩展

- 代表提交：
  - `b32d1a2c` `feat(notify): add balance low & account quota notification system`
  - `915b7a4a` `feat(notify): convert email lists to NotifyEmailEntry struct with toggle support`
  - `eba289a7` `feat(notify): add global toggles, percentage threshold, and visibility control`
- 关键文件：
  - `backend/internal/service/balance_notify_service.go`
  - `backend/internal/service/user_service.go`
  - `backend/internal/handler/admin/setting_handler.go`
  - `frontend/src/components/user/profile/ProfileBalanceNotifyCard.vue`
  - `frontend/src/components/account/QuotaLimitCard.vue`
  - `frontend/src/views/admin/SettingsView.vue`
- 价值：
  - 用户余额不足邮件提醒
  - 账号 quota 告警
  - 额外通知邮箱与验证流程

### 主题 D：渠道统计计费与调度细化

- 代表提交：
  - `7535e312` `feat(channels): add custom account stats pricing rules`
  - `0a4ece5f` `fix: ... intervals persistence ...`
  - `2dce4306` `refactor: move channel model restriction from handler to scheduling phase`
- 关键文件：
  - `backend/internal/service/account_stats_pricing.go`
  - `backend/internal/repository/channel_repo_account_stats_pricing.go`
  - `backend/internal/service/channel_service.go`
  - `backend/internal/service/gateway_service.go`
  - `frontend/src/views/admin/ChannelsView.vue`
- 价值：
  - 后台统计成本与用户计费解耦
  - 规则表达力更强
  - 调度阶段的限制逻辑更统一

### 主题 E：收尾正确性与可维护性修复

- 代表提交：
  - `8548a130` `fix: Messages() routing refactor and subscription group test coverage`
  - `3d202722` `fix: update wire_gen.go to use ProvideSchedulerCache with config injection`
  - `6a08efee` `fix: resolve upstream CI failures (lint, test, gofmt)`
- 关键文件：
  - `backend/internal/handler/openai_gateway_handler.go`
  - `backend/cmd/server/wire_gen.go`
  - 若干测试文件与小范围修复文件
- 价值：
  - 修复隐藏配置未生效问题
  - 让 Messages 路由更稳定、可预测
  - 测试和 CI 更可靠

## 对当前 fork 的建议判断

## 建议优先吸收

### P0：先吸收正确性与低侵入稳定性补丁

- 推荐吸收：
  - `3d202722` 的 `wire_gen.go` 修复
  - `8548a130` 的 Messages 路由重构
  - 与支付/网关直接相关的 CI 修复、小范围 correctness 修复
- 原因：
  - 这类改动收益高、冲突面小、容易验证
  - 不会强依赖大规模前端结构变化
- 涉及文件：
  - `backend/cmd/server/wire.go`
  - `backend/cmd/server/wire_gen.go`
  - `backend/internal/handler/openai_gateway_handler.go`
  - `backend/internal/service/openai_gateway_messages.go`
  - 对应测试文件

### P1：吸收“按 provider 控制用户退款”的支付增强

- 推荐吸收：
  - `f1297a36`
  - `4aa0070e`
  - `c738cfec` 中与当前支付链路直接相关的 correctness 部分
- 原因：
  - 你的 fork 已经对支付链路投入很多定制，细粒度退款控制很符合商用站点治理需求
  - 这类能力更接近“风险收敛”和“后台治理”，比 UI 重构更值得优先投资
- 风险：
  - 涉及 migration、Ent 生成代码、前后端 DTO 和退款校验逻辑
- 涉及文件：
  - `backend/ent/schema/payment_provider_instance.go`
  - `backend/internal/service/payment_refund.go`
  - `backend/internal/service/payment_config_service.go`
  - `backend/internal/payment/load_balancer.go`
  - `backend/migrations/103_add_allow_user_refund.sql`
  - `frontend/src/views/admin/SettingsView.vue`
  - `frontend/src/components/payment/ProviderCard.vue`

### P1：吸收 WebSearch 可用性与运维增强

- 推荐吸收：
  - `fda61b06`
  - `7c729293`
  - `0a4ece5f` 中 WebSearch 与 SMTP timeout 的部分
- 原因：
  - 这部分偏“高收益运维能力”，能明显提升代理异常场景下的可控性
  - 对现在的 API 网关定位有直接价值
- 风险：
  - 需要确认当前 fork 的 WebSearch/代理/Redis key 约定是否已自定义
  - 需要重点检查 failover 行为是否与现有选路策略冲突
- 涉及文件：
  - `backend/internal/pkg/websearch/manager.go`
  - `backend/internal/service/gateway_websearch_emulation.go`
  - `backend/internal/service/websearch_config.go`
  - `backend/internal/service/email_service.go`
  - `backend/internal/server/http.go`
  - `frontend/src/views/admin/SettingsView.vue`

## 建议择机吸收

### P2：吸收余额/额度通知中的“后台治理”部分，不要整包搬运

- 建议只优先评估：
  - 通知阈值模型
  - 额外邮箱结构化存储思路
  - 邮件验证状态和可见性控制
- 暂不建议直接全量吸收：
  - 用户侧完整余额提醒界面
  - 账号 quota 通知全链路
- 原因：
  - 你当前 fork 已经自带管理员订阅通知相关能力，整包合并容易让设置页和通知语义变得混乱
  - 更适合先抽象出统一通知域模型，再决定是否扩展用户侧能力

### P2：吸收 account stats pricing 思路，但暂不直接抄实现

- 原因：
  - 你的 fork 已经新增了仪表盘容量推荐与运营扩容规划，说明“统计与运营分析”很重要
  - 但 upstream 这块落地牵涉仓库、服务、前端配置和 migration，复杂度偏高
- 建议：
  - 先借鉴“统计成本与用户计费解耦”的设计思想
  - 再决定是否在你的 dashboard/recommendation 体系里单独实现

## 建议暂缓

### P3：暂缓支付页面整页重构

- 暂缓对象：
  - `5240b444` 及其后续一串支付 UI 交互重构
- 原因：
  - 变更面太大，且你当前 fork 的支付/订阅体验已经有独立演进
  - 很容易和你已有的 UX、路由参数、商户配置逻辑互相覆盖
- 只有在以下条件满足时再做：
  - 当前支付页确实存在明显用户投诉
  - 你准备为支付前端单独安排一次完整回归测试

## 实施顺序

### 阶段 1：建立上游对照与回归基线

- 动作：
  - 固化 `ad64190..7c671b5` 的提交清单与主题映射
  - 建立“选择性吸收清单”，明确每个提交是“吸收 / 拆吸收 / 暂缓”
  - 先补齐当前 fork 的回归命令基线
- 产出：
  - 一份可执行的吸收清单
  - 一套最小回归命令
- 验证：
  - `cd backend && go test -tags=unit ./...`
  - `cd frontend && pnpm run typecheck`
  - `cd frontend && pnpm run lint:check`

### 阶段 2：先做 P0 正确性补丁

- 动作：
  - 对照 `3d202722` 和 `8548a130`，手工移植小范围修复
  - 不整段 cherry-pick，避免把无关变更带进来
- 重点检查：
  - `wire_gen.go` 是否仍与 `wire.go` 的 provider 签名一致
  - Messages 路由是否仍存在 try-fail-retry 或 gin context anti-pattern
- 验证：
  - 针对网关消息路由的单元测试
  - 启动编译验证 `go test ./cmd/server/... ./internal/...`

### 阶段 3：做 P1 支付退款治理增强

- 动作：
  - 引入 `allow_user_refund`
  - 审核当前退款链路、provider 选择、payment type 映射
  - 核对 wildcard 匹配行为是否与你当前 pricing 规则一致
- 数据库动作：
  - 增加 migration
  - `go generate ./ent`
- 验证：
  - 退款成功、退款失败回滚、用户可退/不可退 provider、管理员可退 provider
  - 支付 provider 类型映射与订单创建/退款链路冒烟测试

### 阶段 4：做 P1 WebSearch 可用性增强

- 动作：
  - 引入 proxy failover、timeout、quota reset usage、配额显示优化
  - 对照现有 Redis key 和代理层约定做最小兼容改造
- 验证：
  - 代理不可用时是否正确 failover
  - quota 递增/回滚/reset 是否正确
  - 管理后台配置保存、读取、展示是否一致

### 阶段 5：评估 P2 通知与统计增强是否单独立项

- 动作：
  - 抽象当前 fork 的通知模型
  - 评估是否做统一通知中心
  - 评估是否单做 account stats pricing，而不是直接搬 upstream 前端实现
- 产出：
  - 继续做或暂缓的决策记录

## 不建议采用的方式

- 不建议直接 `git merge upstream/main`
- 不建议整段 `cherry-pick ad64190..7c671b5`
- 不建议先搬大前端页面再补后端
- 不建议在没有回归基线前直接跑 migration 与 Ent 大同步

## 建议的执行清单

### Task 1：建立提交映射表

**Files:**
- Create: `plans/upstream-sync-working-notes.md`

**Action:**
- 按主题整理以下提交：
  - 正确性修复
  - 支付退款
  - WebSearch
  - 通知
  - 统计计费

**Verify:**
- 每个提交都有“吸收 / 暂缓 / 待拆分”的标签

### Task 2：先做正确性补丁分支

**Files:**
- Modify: `backend/cmd/server/wire_gen.go`
- Modify: `backend/internal/handler/openai_gateway_handler.go`
- Modify: 对应测试文件

**Action:**
- 只移植 P0 修复

**Verify:**
- 后端单元测试通过
- 关键 handler/service 编译通过

### Task 3：做支付退款增强分支

**Files:**
- Modify: `backend/ent/schema/payment_provider_instance.go`
- Modify: `backend/internal/service/payment_refund.go`
- Modify: `backend/internal/service/payment_config_service.go`
- Modify: `frontend/src/views/admin/SettingsView.vue`
- Create: `backend/migrations/XXX_add_allow_user_refund.sql`

**Action:**
- 引入 provider 级退款权限与联动校验

**Verify:**
- 退款相关单元测试与手工冒烟通过

### Task 4：做 WebSearch 运维增强分支

**Files:**
- Modify: `backend/internal/pkg/websearch/manager.go`
- Modify: `backend/internal/service/websearch_config.go`
- Modify: `backend/internal/service/gateway_websearch_emulation.go`
- Modify: `frontend/src/views/admin/SettingsView.vue`

**Action:**
- 引入 failover、timeout、reset usage 与配额展示增强

**Verify:**
- WebSearch 管理与运行链路测试通过

### Task 5：决定通知与统计是否立项

**Files:**
- Create: `plans/upstream-notify-and-stats-evaluation.md`

**Action:**
- 对通知域模型与统计成本模型做二次设计评审

**Verify:**
- 明确“继续做 / 暂缓 / 重写设计”的结论

## 最终建议

- 立即吸收：
  - `3d202722`
  - `8548a130`
  - `f1297a36` 相关的退款权限控制
  - `fda61b06` 和 `7c729293` 中 WebSearch 高价值增强
- 谨慎吸收：
  - `0a4ece5f` 中与 WebSearch、SMTP timeout、account stats interval 直接相关的部分
- 暂缓吸收：
  - `5240b444` 支付页大重构
  - `b32d1a2c` 这一整套通知系统的完整 UI/后端链路
  - `7535e312` 的整套 account stats pricing 前后端实现

## 执行原则

- 选择性吸收，不整体追平
- 每次只处理一个主题
- 先建立回归，再动 migration
- 所有吸收都要以当前 fork 的产品目标为准，而不是机械跟随 upstream
