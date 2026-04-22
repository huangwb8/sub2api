# Upstream `78f691d2..6449da6c` 选择性吸收优化计划

> **For Codex / Claude:** 本文档只负责规划，不直接修改业务源码。后续实施应按主题分批吸收、分批验证，禁止把当前 fork 直接追平上游。

**Goal:** 基于上游 `Wei-Shaw/sub2api` 在 `78f691d2de24d0d13ce68922e120c8119ea32856..6449da6c8daf2a443854cf25de96f3a972e3297c` 之间的演进，梳理哪些变化真正值得当前个人 fork 吸收，并把建议沉淀成低风险、可验证的优化计划。

**Method:** 本轮按 `awesome-code` 的协调方式先运行 `agent_coordinator.py` 做任务拆解；`dispatch_gate.can_proceed = true`。随后结合本地 `git log`、`git diff`、当前 fork 源码现状，以及两路只读并行评估，判断每个主题属于“建议吸收 / 条件吸收 / 暂缓吸收 / 不吸收 / 仅记录无需动作”中的哪一类。

## 范围结论

- 该区间共 **178** 个非 merge 提交。
- 总计涉及 **332** 个文件，约 **93414** 行新增、**12056** 行删除。
- 当前 fork `HEAD=586c20ed` 与 `upstream/main=6449da6c` 的对比是：
  - 当前 fork 领先 **88** 个提交
  - 当前 fork 落后 **350** 个提交
- 当前 fork 的自定义改动主要集中在：
  - 支付与订阅能力扩展
  - 盈利/超售/汇率相关运营看板
  - 首页、费用页、法律页与配套文档
- 上游本区间的主变化集中在：
  - 认证身份体系与 OAuth/WeChat/OIDC 兼容
  - 支付恢复、provider snapshot、resume token、支付方式可见性
  - 管理后台设置联动
  - OpenAI 图片接口与图片计费调度
  - 用户资料页与绑定管理
- 结论：
  - 当前 fork 与上游已经深度分叉，**不适合整段 merge / rebase / 批量 cherry-pick**。
  - 适合继续采用“按主题拆包、按收益吸收、按风险验证”的同步策略。

## 上游变化摘要

### 主题 1：认证身份基础设施重建

代表提交：

- `e9de839d` `feat: rebuild auth identity foundation flow`
- `c6d85924` `feat: add profile auth identity binding flow`
- `6a75bd77` `feat: add pending oauth email onboarding flow`
- `dcd5c43d` `feat: complete email binding and pending oauth verification flows`
- `d4c0a991` `feat(auth): support unbinding third-party identities`
- `65efef1e` `feat: support replacing bound primary email`

核心变化：

- 新增 `auth_identity` / `auth_identity_channel` / `pending_auth_session` / `identity_adoption_decision` 等 Ent Schema 与迁移。
- 把待决 OAuth、邮箱绑定、第三方身份绑定、账号认领冲突处理做成完整后端链路。
- 重写 WeChat/OIDC/LinuxDo 等回调页与 pending session 处理。
- 给用户资料页增加身份绑定管理、解绑、主邮箱替换等能力。

### 主题 2：升级安全与兼容修复

代表提交：

- `1ffebbb5` `fix(migrations): keep auth identity and payment upgrades safe`
- `9de7a72c` `fix(upgrade): close payment and oidc compatibility gaps`
- `06136af8` `fix(upgrade): preserve legacy auth and payment compatibility`
- `0a461d82` `fix: harden auth identity legacy migrations`
- `45065c23` `fix(ci): run 108a migration before 109 in backfill integration test`

核心变化：

- 修复 auth/payment 迁移顺序、兼容回填、旧配置默认值、历史 OAuth 状态与旧支付记录升级风险。
- 为大量历史数据兼容场景补了测试。
- 明显是在给“老站点升级到新身份/支付模型”兜底。

### 主题 3：支付正确性与恢复链路硬化

代表提交：

- `c0b24aef` `feat: snapshot payment provider keys on orders`
- `561405ab` `feat: add payment order provider snapshots`
- `9bebf1c1` `feat: resolve payment results by resume token`
- `b51bc7ee` `feat: wire payment return url payloads`
- `dd314c41` `fix(payment): restore public resume and result flows`
- `d6a04bb7` `fix(payment): support source routing and compatible resume signing`
- `1d8432b8` `fix: harden payment resume and wxpay webhook routing`
- `119f784d` `fix: validate wxpay payments against order snapshots`
- `147ed42a` `fix: restrict payment return urls to internal result page`

核心变化：

- 为订单记录 provider 快照，避免支付配置变动影响历史订单回查与回调归属。
- 用 `resume token` 驱动支付结果页与恢复页查询。
- 收紧 return url、resume 签名、webhook provider 解析与 provider mismatch 校验。
- 让“下单 -> 跳转/拉起 -> 回调 -> 结果页 -> 恢复轮询”这条主链路更稳。

### 主题 4：后台设置与渠道化 WeChat OAuth

代表提交：

- `54dc1767` `feat(settings): support per-channel WeChat OAuth and persist payment options`
- `2cebb0dc` `feat(settings): support dual-mode wechat oauth defaults`
- `b22d00e5` `feat: drive visible payment methods from enabled providers`
- `9e84e2fd` `fix: persist admin payment visibility and scheduler settings`
- `ee3f158f` `fix(settings): restore wechat and payment config persistence`

核心变化：

- Settings API 与后台页新增更多支付/WeChat 配置项。
- 引入渠道维度或双模式 WeChat OAuth 默认值。
- 让“可见支付方式”与真实启用 provider 的关系更一致。
- 修正后台设置保存成功但实际未持久化的缺口。

### 主题 5：OpenAI 图片能力

代表提交：

- `c5480219` `feat(openai): 同步生图 API 支持并接入图片计费调度`
- `1e0d4660` `feat: 补充gpt生图模型测试功能`
- `4d0483f5` `feat: 补充gpt生图模型测试功能`
- `6ad333d6` `fix(openai): 修复生图服务 lint 问题`

核心变化：

- 新增 OpenAI 图片生成/编辑 handler 与 service。
- 把图片请求接入账号调度、计费与用量记录。
- 给测试账号入口补充图片测试能力。

### 主题 6：前端资料页与管理页增强

代表提交：

- `0f4a8d7b` `feat(profile): redesign profile center layout`
- `7309c02f` `refactor(profile): split avatar and bindings cards`
- `6d51834a` `refactor(profile): simplify profile page flow`
- `92041457` `Close profile identity and avatar loop`
- `ed01c599` `feat: track authenticated user activity`

核心变化：

- 资料页围绕身份绑定、头像、账户信息做大改。
- 后台用户列表引入认证后活动时间。
- 更多偏向可观测性与 UX，而不是直接修复网关主链路。

### 主题 7：仓库治理与文档杂项

代表提交：

- `960b2bb8` `feat(legal): add CLA with automated GitHub Actions enforcement`
- `755c7d50` `chore: revert README files to 78f691d2 version`
- `c6d25f69` `chore: 恢复PAYMENT系列文件`
- `6449da6c` `chore: sync VERSION to 0.1.115 [skip ci]`

判断：

- 这类改动不构成当前 fork 的主吸收方向。

## 当前 fork 差距判断

### 已确认当前 fork 尚未吸收

#### 1. Auth Identity Foundation 整体缺席

当前 fork 现状：

- 不存在以下核心文件：
  - `backend/ent/schema/auth_identity.go`
  - `backend/ent/schema/pending_auth_session.go`
  - `backend/internal/handler/auth_oauth_pending_flow.go`
  - `backend/internal/handler/auth_wechat_oauth.go`
  - `frontend/src/views/auth/WechatCallbackView.vue`
- 这意味着当前 fork 仍未引入上游这套新的身份基础设施。

判断：

- 这是一个**高价值但高侵入**主题。
- 它不适合作为“小补丁”混入当前分叉很深的 fork。
- 如果现在强吸收，大概率同时触发 Schema、migration、repository、service、handler、router、store、Profile/Auth 页面联动，风险明显高于一般同步。

建议优先级：

- `P2`

#### 2. 支付 provider snapshot / resume token / resume routing 仍未落地

当前 fork 现状：

- 代码中未检出 `provider_snapshot`、`resume_token`、`payment_resume_service`、`visible_method_source` 等关键实现痕迹。
- 上游这一整组 patch 主要在：
  - `backend/internal/service/payment_order.go`
  - `backend/internal/service/payment_resume_service.go`
  - `backend/internal/service/payment_webhook_provider.go`
  - `frontend/src/views/user/PaymentView.vue`
  - `frontend/src/views/user/PaymentResultView.vue`

判断：

- 这组改动和当前 fork 已有的支付自定义能力是**互补关系**，不是方向冲突。
- 当前 fork 已经在支付与订阅上投入很多定制，这反而意味着支付 correctness 更值得优先补强。
- 它的收益直接落在订单正确性、恢复成功率、回调防错和历史订单稳定性上。

建议优先级：

- `P0`

#### 3. WeChat OAuth 渠道化配置与支付可见性联动仍未落地

当前 fork 现状：

- 代码中未检出 `visible_method_source`、`auth_source_defaults`、`wechat_oauth_mode` 等对应实现。
- 当前 fork 虽已有较多支付前端与后台定制，但这块仍缺少上游后续演进出的“按渠道/模式持久化”和“由启用 provider 驱动可见支付方式”的更强一致性。

判断：

- 如果当前站点存在多入口、多 provider、多支付来源或多渠道回调需求，这组优化很有价值。
- 如果当前部署模型仍较单一，则可以先不抢 P0。

建议优先级：

- `P1`

#### 4. OpenAI 图片网关能力未接入

当前 fork 现状：

- 存在 Gemini 生图测试与图片相关前端文案，但不存在：
  - `backend/internal/handler/openai_images.go`
  - `backend/internal/service/openai_images.go`
- 说明当前 fork 已经对“图片能力”有一定认知，但尚未真正接入 OpenAI 图片 API 网关和图片计费链路。

判断：

- 这是一个**可变现但非必须**的功能增量。
- 只有当你确实准备承接 `/v1/images/generations` 或相邻能力时，才值得排入实施。
- 它比支付 correctness 更容易独立立项，也更适合单独开关式吸收。

建议优先级：

- `P1`

#### 5. 资料页与用户活动增强大多尚未落地

当前 fork 现状：

- 当前 fork 已有账户与 API Key 的 `last_used_at` 体系，但没有上游这套“用户认证活动时间 + 资料页身份绑定中心”的整套增强。
- 这些变化更多集中在：
  - `frontend/src/views/user/ProfileView.vue`
  - `frontend/src/components/user/profile/ProfileIdentityBindingsSection.vue`
  - `frontend/src/views/admin/UsersView.vue`

判断：

- 它们有管理便利性和客服排障价值，但不是当前 fork 主链路最紧迫的稳定性问题。
- 若未来引入 Auth Identity Foundation，再一起做更合适。

建议优先级：

- `P2`

### 已确认无需动作

#### 1. License 本区间无变化

结论：

- `version1` 与 `version2` 的上游 `LICENSE` 内容一致，SHA-256 相同。
- 当前 fork 根目录 `LICENSE` 也是 `LGPL v3`，且 README 系列徽章已经同步为 `LGPL v3`。
- 因此本轮**无需修改本地许可证文件**。

补充说明：

- 上游在更早的 `23def40b` 已经从 MIT 改为 LGPL v3.0。
- 当前 fork 已经跟上这次 license 变更，所以不需要重复处理。

## 选择性吸收建议

## P0：建议优先吸收

### 主题 A：支付 provider snapshot 与支付恢复链路硬化

建议目标：

- 防止 provider 配置变化影响历史订单回调、结果页查询和恢复页恢复。
- 收紧 payment return / resume / webhook provider 识别逻辑，减少支付事故。

建议关注提交：

- `c0b24aef`
- `561405ab`
- `9bebf1c1`
- `b51bc7ee`
- `dd314c41`
- `d6a04bb7`
- `1d8432b8`
- `119f784d`
- `147ed42a`

建议实施文件：

- `backend/ent/schema/payment_order.go`
- `backend/migrations/117_add_payment_order_provider_snapshot.sql`
- `backend/internal/service/payment_order.go`
- `backend/internal/service/payment_resume_lookup.go`
- `backend/internal/service/payment_resume_service.go`
- `backend/internal/service/payment_webhook_provider.go`
- `backend/internal/handler/payment_handler.go`
- `backend/internal/handler/payment_webhook_handler.go`
- `frontend/src/api/payment.ts`
- `frontend/src/views/user/PaymentView.vue`
- `frontend/src/views/user/PaymentResultView.vue`

实施原则：

- 只吸收 correctness / safety / recoverability，不回退当前 fork 已有的支付业务定制。
- 若字段设计与当前 fork 已有订单结构冲突，优先手工 backport，不直接 cherry-pick。
- 所有 migration 必须先做升级兼容评估。

验证方式：

- `cd backend && go test -tags=unit ./internal/service ./internal/handler ./internal/payment/... -run 'Payment|Webhook|Resume'`
- `cd backend && go test -tags=integration ./internal/... -run 'Payment|Webhook|Resume'`
- `cd frontend && pnpm run typecheck && pnpm test -- PaymentView PaymentResultView`
- 手工冒烟：
  - 创建订单后修改 provider 配置，确认历史订单仍能正确回查
  - 支付中断后通过 resume/result 恢复
  - 非法 return url 不被透传到外部页面
  - webhook provider 不会串单

### 主题 B：与支付吸收包直接相关的升级兼容补丁

建议目标：

- 避免“支付主链路修好了，但升级路径把旧数据搞坏”。

建议关注提交：

- `1aab084e`
- `9de7a72c`
- `06136af8`
- `45065c23`

实施原则：

- 只吸收与支付链路直接耦合、且能降低升级风险的补丁。
- 暂不把整个 Auth Identity Foundation 一起带进来。

验证方式：

- 复用现有升级测试基线
- 增加“老订单 + 老配置 + 新代码”集成验证

## P1：条件吸收

### 主题 C：渠道化 WeChat OAuth 与支付可见性设置

建议目标：

- 让后台保存的支付可见性、WeChat OAuth 默认模式与实际运行行为保持一致。
- 让多渠道、多 provider 的运营配置更可控。

建议关注提交：

- `54dc1767`
- `2cebb0dc`
- `b22d00e5`
- `9e84e2fd`
- `ee3f158f`

适用前提：

- 当前站点确实需要多入口 / 多支付源 / 多渠道回调策略。

如果满足前提，建议实施文件：

- `backend/internal/handler/admin/setting_handler.go`
- `backend/internal/handler/dto/settings.go`
- `backend/internal/service/setting_service.go`
- `backend/internal/service/settings_view.go`
- `frontend/src/api/admin/settings.ts`
- `frontend/src/views/admin/SettingsView.vue`

如果不满足前提：

- 保留为后续扩展项，不抢占当前 P0 资源。

验证方式：

- `cd backend && go test -tags=unit ./internal/service ./internal/handler/admin -run 'Setting|Wechat|Payment'`
- `cd frontend && pnpm run typecheck && pnpm test -- SettingsView`
- 手工验证：
  - 修改后台支付可见性后前台即时生效
  - 切换 WeChat OAuth 模式后公开设置与回调页语义一致

### 主题 D：OpenAI 图片接口与图片计费接入

建议目标：

- 在不影响现有文本/Responses 网关稳定性的前提下，单独接入 OpenAI 图片能力。

建议关注提交：

- `c5480219`
- `1e0d4660`
- `4d0483f5`
- `6ad333d6`

判断标准：

- 如果业务准备售卖图片生成/编辑能力：纳入实施。
- 如果当前业务仍以文本 API 为主：暂缓，不阻塞本轮同步。

建议实施文件：

- `backend/internal/handler/openai_images.go`
- `backend/internal/service/openai_images.go`
- `backend/internal/service/openai_gateway_service.go`
- `backend/internal/service/model_pricing_resolver.go`
- `backend/internal/pkg/openai/constants.go`
- `backend/internal/server/routes/gateway.go`
- `frontend/src/components/account/AccountTestModal.vue`
- `frontend/src/components/admin/account/AccountTestModal.vue`

验证方式：

- `cd backend && go test -tags=unit ./internal/service ./internal/handler -run 'Image|OpenAI'`
- `cd frontend && pnpm run typecheck && pnpm test -- AccountTestModal`
- 手工验证：
  - `/v1/images/generations` 请求能路由、计费、记录 usage
  - 非图片模型和 OAuth 限制逻辑符合预期

## P2：建议暂缓，单独立项

### 主题 E：Auth Identity Foundation 全栈重构

建议目标：

- 统一第三方身份、待决 OAuth、邮箱绑定、身份认领冲突与资料页绑定体验。

为什么暂缓：

- 这是本区间最大、最深、最容易与当前 fork 长期分叉冲突的主题。
- 它会同时修改：
  - Ent Schema
  - 多个 migration
  - repository / service / handler / route
  - Auth / Profile / Settings / Payment 前后端联动
- 当前 fork 已经在支付、运营和前台页面上深度定制，不适合把这种“基础设施级重构”混在一般同步里。

建议策略：

- 先单独做一份 “Auth Identity Foundation 可行性与冲突审计计划”。
- 只有在你明确需要统一 OAuth/邮箱/WeChat/OIDC 身份模型时，再独立推进。

### 主题 F：资料页重构与用户活动增强

建议目标：

- 提升资料页的绑定管理可用性与后台用户排障效率。

为什么暂缓：

- 这类改动更偏 UX / 运维辅助。
- 若没有先引入上游身份基础设施，单独做资料页改版收益有限。

## 不建议吸收

### 1. README 回退、版本号同步、赞助商更新

原因：

- 与当前 fork 的产品能力、稳定性和营收主链路关联弱。
- 当前 fork 的 README 体系与上游已经明显不同。

### 2. 上游 CLA 工作流

原因：

- `CLA.md` 与 `.github/workflows/cla.yml` 属于上游仓库治理策略，不是 license 变化。
- 当前 fork 若未来需要 CLA，应按当前维护主体与贡献流程单独设计，而不是直接照搬。

### 3. 纯展示型 Profile / Settings 视觉调整

原因：

- 只改布局、不改善 correctness / observability / upgrade safety 的部分，不值得抢占当前同步预算。

## 实施批次建议

### 阶段一：建立去重基线

- 盘点当前 fork 对以下主题的现状：
  - payment snapshot / resume token / webhook routing
  - WeChat OAuth 多模式与支付可见性
  - OpenAI 图片请求、计费与用量记录
- 明确每个主题的：
  - 已吸收
  - 部分吸收但语义不同
  - 完全未吸收

### 阶段二：先做 P0 支付正确性

- 优先落地 provider snapshot 与 resume/result 修复。
- 同步做升级兼容验证。
- 不动与当前 fork 无关的 UI 风格改动。

### 阶段三：按业务决策选择 P1

- 若站点需要多渠道支付/登录，做 Settings + WeChat OAuth。
- 若站点准备提供图片能力，做 OpenAI Images。

### 阶段四：把 Auth Identity Foundation 独立审计

- 不与支付同步混做。
- 单独形成更细的冲突分析、数据迁移策略和回滚方案。

## 验收标准

- 能清楚解释每个吸收主题“为什么值得吸收”。
- 能清楚解释每个暂缓或不吸收主题“为什么不值得现在做”。
- 不出现跨越 Schema、migration、service、frontend 的无边界大搬运。
- 任何涉及迁移的主题都具备升级前、升级后、回滚影响三类说明。
- 支付、认证、网关三条主链路的验证要求在实施前就已写清。

## License 结论

- 上游 `78f691d2..6449da6c` 区间 **未修改 `LICENSE`**。
- 当前 fork 已经是 `LGPL v3`，并在 README 系列中完成展示同步。
- 本轮 **无需修改 `LICENSE`、README 法律说明或 `CHANGELOG.md`**。
