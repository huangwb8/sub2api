# User Resale Risk Control Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 为订阅用户增加“疑似二次销售 / 转售 / 超售滥用”风险控制，先做到可信观测、0-5 分评分、分级告警、3 天整改宽限和自动锁定闭环，同时尽量降低对正常高频用户的误伤。

**Architecture:** 采用“实时轻信号 + 每日批评估 + 宽限状态机”的增量方案。请求入口只负责采集可信 IP / UA 和实时并发 IP 证据，后台定时 evaluator 结合 `usage_logs`、Redis overlap evidence 与系统配置生成风险事件、更新用户评分、触发邮件与自动锁定；认证中间件基于风险档案返回明确的违规提示，而不是继续只返回泛化的 `USER_INACTIVE`。

**Tech Stack:** Go / Gin / Ent / PostgreSQL / Redis / EmailQueueService / Vue 3 / Pinia / Tailwind CSS

**Minimal Change Scope:** 仅修改 `backend/ent/schema`、`backend/migrations`、`backend/internal/service`、`backend/internal/repository`、`backend/internal/server/middleware`、`backend/internal/handler/admin`、`backend/internal/handler/dto`、`backend/internal/server/routes`、`frontend/src/api/admin`、`frontend/src/views/admin`、`frontend/src/types`、`frontend/src/i18n`、必要的 `docs/` 文档；避免改动网关调度、计费规则、支付流程、前台购买流程和非本需求相关的账号管理逻辑。

**Success Criteria:** 管理员可以配置并启用该机制；系统能基于可信请求证据计算 0-5 分并记录原因；用户分数跌破阈值时收到包含整改建议的警告邮件；若连续 3 个日评估周期未改善则自动锁定；被锁定用户通过 API / JWT 访问时能收到明确的违规提示和联系管理员的指引；管理员可查看证据、手动解锁、豁免或重置评分。

**Verification Plan:** `cd backend && go generate ./ent`；`cd backend && go test -tags=unit ./...`；必要时补充仓储集成测试 `cd backend && go test -tags=integration ./internal/repository/...`；`cd frontend && pnpm run typecheck`；`cd frontend && pnpm run lint:check`；管理员手动验证“告警邮件 -> 3 天宽限 -> 自动锁定 -> API 返回锁定提示 -> 管理员解锁”整条链路。

---

## 背景判断

- 这个需求的本质不是“百分之百识别转售”，而是用较低误伤率识别“明显不符合单人订阅使用方式”的持续行为。
- 现有系统已经具备可复用基础：
  - `usage_logs` 已记录 `ip_address`、`user_agent`、`subscription_id`、`actual_cost`、`total_tokens`。
  - SMTP、异步邮件队列、定时任务、Redis 分布式锁、用户状态禁用链路都已经存在。
  - 管理后台已有用户管理、订阅管理、公告、系统设置等基础页面。
- 现有系统也有两个关键缺口：
  - 当前 `usage_logs.ip_address` 来自 `ip.GetClientIP(c)`，会优先信任原始转发头，不适合作为自动封禁级别的可信证据。
  - 现有用户状态只有 `active/disabled`，没有“风险告警中 / 宽限期 / 风险锁定原因 / 解锁备注”这一层业务状态。

## 推荐边界

### 这个机制应该防什么

- 同一订阅在短时间内被多个公网来源并发使用。
- 连续多天明显接近或压穿单人订阅的合理容量。
- 高频 IP/UA 扇出、长时间 24x7 运行、多 Key 并行分发等“中间商转售”迹象。

### 这个机制不应该试图一次解决什么

- 不做“绝对反作弊”。
- 不在 v1 引入 GeoIP、ASN、设备指纹 SDK、浏览器端探针。
- 不因为一次异常峰值就立刻自动封禁。
- 不直接改动现有订阅计费逻辑或 oversell 计算口径。

### 误伤控制原则

- 仅使用可信公网 IP 作为自动处置依据。
- IPv6 应按 `/64` 前缀归并，避免隐私地址轮换导致误判。
- 单个信号不直接封禁，至少需要“评分跌破阈值 + 连续未整改”。
- 默认分三阶段上线：
  - `observe_only`
  - `warn_only`
  - `auto_lock`

## 推荐评分与状态机

### 评分

- 评分范围：`0.0 ~ 5.0`
- 默认初始分：`5.0`
- 每日最大下调：`1.0`
- 每日最大恢复：`0.5`
- 展示口径：
  - `4.0~5.0`：正常
  - `3.0~3.9`：观察
  - `2.0~2.9`：警告
  - `<2.0`：高风险

### 建议阈值

- `warning_threshold = 3.0`
- `lock_threshold = 2.0`
- `auto_lock_after_consecutive_bad_days = 3`

### 状态机

- `healthy`
- `observed`
- `warned`
- `grace_period`
- `locked`
- `exempted`

### 自动锁定建议规则

- 当日评分 `< 3.0`：发送警告邮件并进入 `warned/grace_period`
- 连续 3 个日评估周期评分仍 `< 3.0`，且最新评分 `< 2.0`：自动锁定
- 任一日恢复到 `>= 3.0`：清空连续未整改计数
- `exempted` 用户只记录事件，不触发自动锁定

## 推荐信号

### 一级信号：强相关

1. 同一时间多个不同公网 IP 并发连接
   - 核心判定必须基于可信 IP，而不是原始 `CF-Connecting-IP/X-Forwarded-For` 头。
   - 建议用 Redis 记录用户当前活跃请求集合，按 5 分钟窗口去抖生成 evidence event。
2. 连续多天高负载
   - 建议同时看 `actual_cost` 与 `total_tokens`。
   - 如果订阅分组配置了 `daily_limit_usd`，优先按“日成本占日限额比例”判断。
   - 如果没有日限额，则回退到“近 14 天个人中位数 × 倍数 + 同套餐用户分位数”。
3. 多 Key 扇出
   - 同一用户多个 API Key 在同一天被不同 IP 池并行消费。

### 二级信号：辅助加权

1. 单日公网 IP 数异常多
2. 归一化 UA family 异常多
3. 活跃小时数长期接近全天
4. 请求节律高度平稳且长期压满并发 / RPM

### v1 不建议直接启用的信号

- GeoIP 跨国跳跃
- ASN 画像
- 浏览器设备指纹
- 纯 token 大小绝对值阈值

## 推荐数据设计

### 首选方案

- 新增 `user_risk_profiles`
  - 每用户一行，保存当前分数、当前状态、最近警告时间、连续未整改天数、锁定时间、锁定原因、豁免信息、最后评估时间。
- 新增 `user_risk_events`
  - 追加式证据表，保存事件类型、严重度、分数变动、窗口时间、摘要、结构化 metadata、是否已解决。

### 暂不建议在 v1 做的表

- 不强制新增 `user_risk_daily_snapshots`。
- 先复用 `usage_logs` 做日统计；只有在查询成本明显变高时，再补快照表。

### 为什么不直接把所有字段塞进 `users`

- 审计性差。
- 不利于解释“用户需要改正什么”。
- 后续要做管理员解锁、申诉备注、事件追溯时会很快失控。

## 可信 IP 设计要求

- 风控与自动锁定必须使用 `ip.GetTrustedClientIP(c)` 或等价可信链路。
- 现有 `usage_logs.ip_address` 由于来自 `ip.GetClientIP(c)`，不应直接作为“上线即自动封禁”的历史依据。
- 推荐做法：
  - 新证据生成逻辑改用可信 IP。
  - 历史旧数据只用于观察，不用于高风险自动处置。
  - 只有在 `trusted_proxies` 配置合理时才允许启用 `auto_lock`。

## 用户触达设计

### 警告邮件必须包含

- 当前评分与风险状态
- 触发的 1-3 条主要证据
- 明确整改建议
  - 停止共享 API Key
  - 停止多地同时在线
  - 若 Key 疑似泄漏，立即轮换
  - 若为合法团队场景，请联系管理员申请豁免 / 企业方案
- 用户协议或联系管理员入口

### 锁定后的 API/JWT 提示

- 不再只返回泛化的 `USER_INACTIVE`
- 新增风险专用错误码，建议：
  - `USER_RISK_LOCKED`
- 默认提示文案建议：
  - `Your account has been locked for suspected resale or Terms of Service violation. Please contact the administrator to unlock it.`

## 配置建议

- 复用系统设置表，新增 JSON 设置键，例如：
  - `user_risk_control_config`
- v1 配置项建议包含：
  - `enabled`
  - `mode`
  - `warning_threshold`
  - `lock_threshold`
  - `auto_lock_after_consecutive_bad_days`
  - `overlap_window_seconds`
  - `max_distinct_public_ips_per_day`
  - `high_load_cost_ratio_threshold`
  - `high_load_token_multiplier`
  - `warning_email_enabled`
  - `warning_email_subject_template`
  - `lock_message`
  - `require_trusted_proxy_for_auto_lock`

## 管理后台建议

- 用户列表显示：
  - 风险评分 badge
  - 风险状态
  - 最近警告时间
  - 是否豁免
- 用户详情增加：
  - 风险事件时间线
  - 近 7/14 天 IP/UA/高负载摘要
  - 手动警告
  - 手动解锁
  - 临时豁免 / 永久豁免
  - 评分重置
- 设置页增加：
  - 风控开关与模式
  - 阈值配置
  - 邮件模板

## 回滚策略

- 配置层回滚：
  - 关闭 `enabled`
  - 将 `mode` 切回 `observe_only`
- 运行时回滚：
  - 停止 evaluator service
  - 保留风险事件数据，不删表
- 行为回滚：
  - 不删除历史 evidence
  - 对已锁定用户提供批量解锁脚本或管理员按钮

## 执行建议

- `@backend-specialist`：风险状态机、仓储、定时任务、认证中间件
- `@security-specialist`：可信 IP、IPv6 前缀归并、误伤边界复核
- `@documentation-specialist`：管理员说明、用户协议与告警模板更新

## Task 1: 明确配置契约与状态模型

**Files:**
- Modify: `backend/internal/service/domain_constants.go`
- Modify: `backend/internal/service/settings_view.go`
- Modify: `backend/internal/service/setting_service.go`
- Create: `backend/internal/service/user_risk_models.go`
- Test: `backend/internal/service/setting_service_update_test.go`
- Test: `backend/internal/service/user_risk_models_test.go`

**Step 1: 写失败测试**

- 为风险设置默认值、阈值校验、模式切换、状态机转换规则补单测。

**Step 2: 运行失败测试**

Run: `cd backend && go test -tags=unit ./internal/service/...`
Expected: 与 `user_risk_*` 相关的测试失败，提示设置键或模型缺失。

**Step 3: 实现最小配置与模型**

- 增加 `user_risk_control_config` 设置读取 / 更新。
- 定义风险分数字段、状态枚举、事件类型枚举。

**Step 4: 重新跑测试**

Run: `cd backend && go test -tags=unit ./internal/service/...`
Expected: 配置与模型测试通过。

**Step 5: 提交**

```bash
git add backend/internal/service/domain_constants.go backend/internal/service/settings_view.go backend/internal/service/setting_service.go backend/internal/service/user_risk_models.go
git commit -m "feat: add user risk control settings contract"
```

## Task 2: 建立风险档案与事件表

**Files:**
- Create: `backend/ent/schema/user_risk_profile.go`
- Create: `backend/ent/schema/user_risk_event.go`
- Create: `backend/migrations/120_create_user_risk_profiles.sql`
- Create: `backend/migrations/121_create_user_risk_events.sql`
- Create: `backend/internal/repository/user_risk_repo.go`
- Test: `backend/internal/repository/user_risk_repo_integration_test.go`

**Step 1: 写失败测试**

- 覆盖 profile upsert、event append、按用户查询、解锁 / 豁免更新。

**Step 2: 运行失败测试**

Run: `cd backend && go test -tags=integration ./internal/repository/...`
Expected: 风险表不存在或仓储方法缺失。

**Step 3: 实现最小数据层**

- 新增 Ent schema 与 SQL migration。
- profile 表保存当前态，event 表保存审计轨迹。

**Step 4: 生成并验证**

Run: `cd backend && go generate ./ent && go test -tags=integration ./internal/repository/...`
Expected: 仓储层测试通过。

**Step 5: 提交**

```bash
git add backend/ent/schema backend/migrations backend/internal/repository/user_risk_repo.go
git commit -m "feat: add user risk persistence layer"
```

## Task 3: 采集可信 IP 与实时 overlap 证据

**Files:**
- Modify: `backend/internal/handler/gateway_handler.go`
- Modify: `backend/internal/server/middleware/api_key_auth.go`
- Modify: `backend/internal/server/middleware/api_key_auth_google.go`
- Modify: `backend/internal/pkg/ip/ip.go`
- Create: `backend/internal/service/user_risk_signal_service.go`
- Test: `backend/internal/service/user_risk_signal_service_test.go`
- Test: `backend/internal/server/middleware/api_key_auth_test.go`

**Step 1: 写失败测试**

- 覆盖可信 IP 提取、IPv6 `/64` 归并、私网 IP 忽略、并发多 IP overlap 事件去抖。

**Step 2: 运行失败测试**

Run: `cd backend && go test -tags=unit ./internal/service/... ./internal/server/middleware/...`
Expected: risk signal service 或 trusted IP 归并逻辑缺失。

**Step 3: 实现最小采集**

- 在请求入口采集 `risk_ip_key`。
- 用 Redis 记录“当前活跃 IP 集合”和 overlap evidence。
- 严禁继续用原始不可信头直接驱动自动封禁。

**Step 4: 重新跑测试**

Run: `cd backend && go test -tags=unit ./internal/service/... ./internal/server/middleware/...`
Expected: overlap 采集与 middleware 测试通过。

**Step 5: 提交**

```bash
git add backend/internal/handler/gateway_handler.go backend/internal/server/middleware backend/internal/pkg/ip/ip.go backend/internal/service/user_risk_signal_service.go
git commit -m "feat: capture trusted risk ip overlap signals"
```

## Task 4: 实现每日评估器与 0-5 分评分引擎

**Files:**
- Create: `backend/internal/service/user_risk_evaluator_service.go`
- Create: `backend/internal/service/user_risk_scoring.go`
- Modify: `backend/cmd/server/wire.go`
- Modify: `backend/internal/service/usage_service.go`
- Modify: `backend/internal/repository/usage_log_repo.go`
- Test: `backend/internal/service/user_risk_scoring_test.go`
- Test: `backend/internal/service/user_risk_evaluator_service_test.go`

**Step 1: 写失败测试**

- 覆盖高负载天判断、连续坏天计数、日恢复机制、warning / lock 阈值行为。

**Step 2: 运行失败测试**

Run: `cd backend && go test -tags=unit ./internal/service/...`
Expected: evaluator、评分函数或 usage 聚合辅助方法缺失。

**Step 3: 实现最小评估器**

- 每分钟触发一次，但只执行“到点的日评估”。
- 读取风险配置、汇总最近窗口证据、输出 profile + event 变更。
- 默认支持 `observe_only / warn_only / auto_lock`。

**Step 4: 重新跑测试**

Run: `cd backend && go test -tags=unit ./internal/service/...`
Expected: 评分和 evaluator 测试通过。

**Step 5: 提交**

```bash
git add backend/internal/service/user_risk_* backend/cmd/server/wire.go backend/internal/repository/usage_log_repo.go
git commit -m "feat: add daily user risk evaluator"
```

## Task 5: 告警邮件、宽限期与自动锁定链路

**Files:**
- Create: `backend/internal/service/user_risk_notification.go`
- Modify: `backend/internal/service/email_queue_service.go`
- Modify: `backend/internal/service/admin_service.go`
- Test: `backend/internal/service/user_risk_notification_test.go`
- Test: `backend/internal/service/admin_service_test.go`

**Step 1: 写失败测试**

- 覆盖 warning 邮件内容、连续 3 天未整改自动锁定、管理员手动解锁后状态清理。

**Step 2: 运行失败测试**

Run: `cd backend && go test -tags=unit ./internal/service/...`
Expected: warning / lock 动作实现缺失。

**Step 3: 实现最小动作层**

- 复用 `EmailQueueService.EnqueueCustomEmail` 发送警告邮件。
- 锁定时写入 risk profile，并同步失效认证缓存。
- 管理员解锁时清理 `locked` 状态与连续坏天计数。

**Step 4: 重新跑测试**

Run: `cd backend && go test -tags=unit ./internal/service/...`
Expected: 动作链路测试通过。

**Step 5: 提交**

```bash
git add backend/internal/service/user_risk_notification.go backend/internal/service/email_queue_service.go backend/internal/service/admin_service.go
git commit -m "feat: add risk warning and auto lock actions"
```

## Task 6: 在认证层返回明确违规提示

**Files:**
- Modify: `backend/internal/server/middleware/api_key_auth.go`
- Modify: `backend/internal/server/middleware/api_key_auth_google.go`
- Modify: `backend/internal/server/middleware/jwt_auth.go`
- Modify: `backend/internal/server/middleware/admin_auth.go`
- Test: `backend/internal/server/middleware/api_key_auth_test.go`
- Test: `backend/internal/server/middleware/jwt_auth_test.go`

**Step 1: 写失败测试**

- 覆盖已锁定用户的 API Key、JWT、自助页面访问返回 `USER_RISK_LOCKED` 与自定义提示。

**Step 2: 运行失败测试**

Run: `cd backend && go test -tags=unit ./internal/server/middleware/...`
Expected: 认证层仍只返回 `USER_INACTIVE`。

**Step 3: 实现最小拦截**

- 在用户状态检查前增加 risk profile 检查。
- 锁定提示包含“违反用户协议 / 联系管理员解锁”。

**Step 4: 重新跑测试**

Run: `cd backend && go test -tags=unit ./internal/server/middleware/...`
Expected: 锁定提示测试通过。

**Step 5: 提交**

```bash
git add backend/internal/server/middleware
git commit -m "feat: return explicit risk lock auth errors"
```

## Task 7: 管理后台配置、证据查看与解锁

**Files:**
- Modify: `backend/internal/server/routes/admin.go`
- Modify: `backend/internal/handler/admin/user_handler.go`
- Modify: `backend/internal/handler/dto/types.go`
- Modify: `backend/internal/handler/dto/mappers.go`
- Modify: `frontend/src/api/admin/users.ts`
- Modify: `frontend/src/types/index.ts`
- Modify: `frontend/src/views/admin/UsersView.vue`
- Modify: `frontend/src/i18n/locales/zh.ts`
- Modify: `frontend/src/i18n/locales/en.ts`
- Test: `frontend/src/views/admin/__tests__/UsersView.spec.ts`

**Step 1: 写失败测试**

- 覆盖用户列表风险 badge、详情事件拉取、管理员解锁 / 豁免动作。

**Step 2: 运行失败测试**

Run: `cd frontend && pnpm test`
Expected: 风险字段或交互接口不存在。

**Step 3: 实现最小管理面**

- 用户列表新增风险列。
- 用户详情或侧边抽屉展示事件摘要。
- 增加“发送警告 / 解锁 / 豁免”操作。

**Step 4: 重新跑测试**

Run: `cd frontend && pnpm run typecheck && pnpm test`
Expected: 风险管理 UI 测试通过。

**Step 5: 提交**

```bash
git add backend/internal/server/routes/admin.go backend/internal/handler/admin/user_handler.go backend/internal/handler/dto frontend/src/api/admin/users.ts frontend/src/types/index.ts frontend/src/views/admin/UsersView.vue frontend/src/i18n/locales
git commit -m "feat: add admin user risk management ui"
```

## Task 8: 文档、灰度与上线顺序

**Files:**
- Modify: `docs/` 中与管理员设置、用户协议、告警说明相关的非计划文档
- Modify: `README.md`
- Modify: `README_EN.md`
- Modify: `README_JA.md`
- Modify: `CHANGELOG.md`

**Step 1: 写验收清单**

- 明确 observe-only、warn-only、auto-lock 三阶段的启用条件和退出条件。

**Step 2: 补文档**

- 管理员如何配置阈值
- 如何解锁误伤用户
- 用户会收到什么邮件 / API 错误

**Step 3: 先灰度**

- 先以 `observe_only` 跑 7-14 天
- 校验误伤率
- 再开 `warn_only`
- 最后才开 `auto_lock`

**Step 4: 验证上线**

Run:

```bash
cd backend && go test -tags=unit ./...
cd frontend && pnpm run typecheck
cd frontend && pnpm run lint:check
```

Expected: 单元测试、类型检查、Lint 全通过。

**Step 5: 提交**

```bash
git add docs README.md README_EN.md README_JA.md CHANGELOG.md
git commit -m "docs: document user risk control rollout"
```

## 最终建议

- v1 不要追求“直接秒封”，而要追求“可解释、可回滚、可申诉”。
- 自动锁定必须建立在可信 IP 链路上，否则误伤和绕过都会很重。
- 最稳妥的落地顺序是：先观测，再告警，最后自动锁定。
