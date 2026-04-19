# 盈利面板成本口径修复 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 修复管理后台“盈利水平”面板在远端部署环境中“估算成本恒为 0、额外盈利率视觉失真”的问题，让订阅收入、订阅使用成本、余额收入和余额成本落在同一套可解释口径下。

**Architecture:** 远端实测表明，当前 `/api/v1/admin/dashboard/profitability` 已不再报错，但它只聚合 `usage_logs.estimated_cost_cny` 作为成本；而订阅计费链路不会给 usage log 写入这个字段，导致“有订阅收入、有非管理员 usage、但估算成本仍为 0”。修复需要分三层推进：先用测试锁定远端复现，再补齐 usage 写入与历史回填，最后收敛前端展示，避免全空盈利率序列被 Chart.js 渲染成误导性的 `0% ~ 1%` 右轴。

**Tech Stack:** Go / Gin / Ent / PostgreSQL / Vue 3 / TypeScript / Vitest

---

## 已确认发现

### 远端接口当前不是 500，而是成功返回了“错误口径的数据”

- `GET /api/v1/admin/dashboard/profitability/bounds` 返回 `{"has_data": true, "earliest_date": "2026-04-13"}`
- `GET /api/v1/admin/dashboard/profitability?start_date=2026-04-13&end_date=2026-04-19&granularity=day` 返回：
  - `2026-04-13` 订阅收入 `¥270`、估算成本 `¥0`
  - `2026-04-15` 订阅收入 `¥90`、估算成本 `¥0`
  - 其余日期利润与收入相等，因为成本始终为 `0`

### 远端真实存在非管理员 usage，但这些 usage 没有被 profitability 面板计入成本

- `GET /api/v1/admin/payment/orders?page=1&page_size=100&order_type=subscription` 显示：
  - 2026-04-13 有 3 笔订阅订单，金额共 `¥270`
  - 2026-04-15 有 1 笔订阅订单，金额 `¥90`
- `GET /api/v1/admin/usage?start_date=2026-04-13&end_date=2026-04-19&page_size=100...` 显示：
  - 管理员 `user_id=1` 在 `group_id=2` 有大量余额计费 usage，`charged_amount_cny > 0`
  - 非管理员 `user_id=4`、`user_id=7` 在 `group_id=6` 有大量订阅 usage
  - 这些非管理员订阅 usage 的共同特征是：`charged_amount_cny = 0`，`estimated_cost_cny = null`

### 远端账号配置本身不是空的

- `GET /api/v1/admin/accounts?page=1&page_size=50` 显示远端 OpenAI 账号已配置：
  - `actual_cost_cny`
  - `actual_cost_usage_usd`
  - `actual_cost_updated_at`
- 同一批账号同时绑定了：
  - `group_id=2`：`GPT-Usage-Based`
  - `group_id=6`：`GPT-Standard`
  - `group_id=7`：`GPT-Premium`

这说明“订阅 usage 没法估成本”不是因为远端完全没有账号实际成本配置，而是代码没有把这套成本快照落到订阅 usage log 上。

## 根因判断

### 根因 1：后端只在非订阅计费分支写 `estimated_cost_cny`

直接代码证据：

- [backend/internal/service/gateway_service.go](/Volumes/2T01/Github/sub2api/backend/internal/service/gateway_service.go)
- [backend/internal/service/openai_gateway_service.go](/Volumes/2T01/Github/sub2api/backend/internal/service/openai_gateway_service.go)
- [backend/internal/service/balance_profitability.go](/Volumes/2T01/Github/sub2api/backend/internal/service/balance_profitability.go)

当前逻辑只有在 `!isSubscriptionBilling` 且 `resolveStandardBalanceCharge(...)` 成功时，才会给 `usageLog.EstimatedCostCNY` 赋值。  
一旦 usage 属于订阅计费，`profitabilityCharge` 就不会执行，`estimated_cost_cny` 只能保持空值。

### 根因 2：profitability SQL 只认 `estimated_cost_cny`，不会为订阅 usage 做成本兜底

直接代码证据：

- [backend/internal/repository/usage_log_repo.go](/Volumes/2T01/Github/sub2api/backend/internal/repository/usage_log_repo.go)

`GetProfitabilityTrend()` 当前成本来源只有：

- `ul.estimated_cost_cny`

而订阅收入来自：

- `payment_orders.amount`

这就形成了一个明显的不对称：

- 收入把订阅订单算进来了
- 成本却没有把订阅 usage 算进来

结果就是远端当前看到的：

- `subscription revenue > 0`
- `estimated cost = 0`
- `profit = revenue`

### 根因 3：前端在“全空盈利率序列”时仍渲染右轴，导致看起来像 `0% ~ 1%`

直接代码证据：

- [frontend/src/views/admin/dashboardProfitability.ts](/Volumes/2T01/Github/sub2api/frontend/src/views/admin/dashboardProfitability.ts)
- [frontend/src/components/charts/ProfitabilityTrendChart.vue](/Volumes/2T01/Github/sub2api/frontend/src/components/charts/ProfitabilityTrendChart.vue)

当所有点的 `extra_profit_rate_percent` 都是 `null` 时：

- 摘要卡片会显示 `--`
- 但图表右轴仍然存在
- Chart.js 会把空序列自动收缩成近似 `0 ~ 1` 的默认刻度

所以用户看到的“额外盈利率一直是 0% ~ 1%”并不是后端真的算出了 `0% ~ 1%`，而是“没有可计算盈利率时的默认坐标轴假象”。

## 实施假设

- 假设当前面板应继续保留“订阅收入”这一维度，而不是把面板改成纯余额计费面板。
- 在这个假设下，修复方向应当是“把订阅 usage 成本补齐”，而不是“把订阅收入从面板删掉”。
- 如果后续产品决定该面板只讨论标准余额计费，那么 Task 3 应改成删掉 `subscription_orders` 聚合，并同步改文案。

## Task 1: 固化远端复现为失败测试

**Files:**
- Modify: `backend/internal/service/openai_gateway_record_usage_test.go`
- Modify: `backend/internal/service/gateway_record_usage_test.go`
- Modify: `backend/internal/repository/usage_log_repo_profitability_test.go`
- Modify: `frontend/src/views/admin/__tests__/dashboardProfitability.spec.ts`

**Step 1: 写服务层失败测试，锁定“订阅 usage 不写 estimated_cost_cny”**

- 在 `openai_gateway_record_usage_test.go` 新增用例：
  - 条件：`subscription != nil`、`group.IsSubscriptionType() == true`
  - 条件：`account.actual_cost_cny` 与 `account.actual_cost_usage_usd` 已配置
  - 期望：`usageRepo.lastLog.EstimatedCostCNY != nil`
  - 期望：订阅计费依然不扣用户余额，不影响 `BillingTypeSubscription`
- 在 `gateway_record_usage_test.go` 为通用网关链路补同类用例，避免两个实现分叉。

**Step 2: 写 repository 失败测试，锁定“订阅收入进来了但订阅成本没进来”**

- 在 `usage_log_repo_profitability_test.go` 新增用例：
  - 构造一个时间桶内同时存在：
    - 订阅订单收入
    - 订阅 usage 成本
  - 期望：返回的 `estimated_cost_cny > 0`
  - 期望：`profit_cny = revenue - cost`

**Step 3: 写前端失败测试，锁定“全空 rate 序列不应显示 0%~1% 视觉假象”**

- 在 `dashboardProfitability.spec.ts` 新增用例：
  - 输入：所有点 `extra_profit_rate_percent = null`
  - 期望：构图逻辑不输出 rate dataset，或明确标记为不可渲染
  - 期望：摘要仍显示 `--`

**Step 4: 运行测试确认它们先失败**

Run:

```bash
cd backend && go test -tags=unit ./internal/service -run 'Test(OpenAIGatewayServiceRecordUsage_Subscription|GatewayServiceRecordUsage_Subscription)'
```

Expected:

- FAIL，表现为订阅 usage log 的 `EstimatedCostCNY` 仍为 `nil`

Run:

```bash
cd backend && go test -tags=unit ./internal/repository -run TestUsageLogRepositoryGetProfitabilityTrend
```

Expected:

- FAIL，表现为订阅 usage 成本没有进入 profitability 聚合

Run:

```bash
cd frontend && pnpm test:run dashboardProfitability.spec.ts
```

Expected:

- FAIL，表现为前端仍尝试渲染全空 rate 轴

## Task 2: 抽离统一的“盈利面板成本快照”写入逻辑

**Files:**
- Modify: `backend/internal/service/balance_profitability.go`
- Modify: `backend/internal/service/gateway_service.go`
- Modify: `backend/internal/service/openai_gateway_service.go`

**Step 1: 抽出只负责“估算成本”的 helper**

- 在 `balance_profitability.go` 中提炼一个不依赖计费类型的 helper，例如：
  - 输入：`account`、`totalCostUSD`
  - 输出：`estimatedCostCNY`
- 这个 helper 只做一件事：
  - 根据账号实际成本配置，把 usage 的 USD 成本换算成 CNY 成本快照

**Step 2: 让余额计费与订阅计费共用这份成本快照**

- 余额计费：
  - 继续使用现有 `resolveStandardBalanceCharge()` 生成收费快照
  - 但它内部要复用新的成本 helper，而不是自己单独算一遍
- 订阅计费：
  - 即使不向用户扣余额，也要把 `estimated_cost_cny` 写到 usage log

**Step 3: 保持现有账务行为不变**

- 不改变：
  - 订阅计费的余额扣减逻辑
  - `BillingTypeSubscription`
  - 订阅 usage 的 `charged_amount_cny`
- 只新增：
  - 面向盈利面板统计的成本快照写入

**Step 4: 运行服务层测试直到通过**

Run:

```bash
cd backend && go test -tags=unit ./internal/service -run 'Test(OpenAIGatewayServiceRecordUsage_Subscription|GatewayServiceRecordUsage_Subscription)'
```

Expected:

- PASS，且订阅 usage log 持有非空 `EstimatedCostCNY`

## Task 3: 修正 profitability 聚合口径，并补历史数据

**Files:**
- Modify: `backend/internal/repository/usage_log_repo.go`
- Create: `backend/migrations/109_backfill_subscription_profitability_cost.sql`
- Modify: `backend/internal/repository/migrations_schema_integration_test.go`
- Modify: `backend/internal/repository/usage_log_repo_profitability_test.go`

**Step 1: 给 profitability 查询增加“订阅成本兜底”**

- 在 repository 查询里增加清晰的优先级：
  - 第一优先级：`usage_logs.estimated_cost_cny`
  - 第二优先级：对历史订阅行做保守兜底
- 推荐兜底方式：
  - 对 `billing_type = subscription` 且 `estimated_cost_cny IS NULL` 的 usage
  - 基于 `usage_logs.actual_cost` 与 `accounts.actual_cost_cny / accounts.actual_cost_usage_usd` 推导一个临时成本
- 这样做的目的：
  - 在历史回填尚未执行或未完全覆盖时，面板也不会继续显示 0 成本

**Step 2: 写一条一次性 backfill migration，填平历史空洞**

- 新建 `109_backfill_subscription_profitability_cost.sql`
- 范围：
  - `usage_logs.billing_type = subscription`
  - `estimated_cost_cny IS NULL`
  - `actual_cost > 0`
  - 账号实际成本配置可用
- 结果：
  - 把历史订阅 usage 的 `estimated_cost_cny` 落库

**Step 3: 在测试里锁定“有历史脏数据也能查对”**

- repository 测试增加两类场景：
  - 新数据：`estimated_cost_cny` 已经写好
  - 老数据：`estimated_cost_cny` 为空，只能走 SQL 兜底

**Step 4: 运行 repository 与 migration 相关测试**

Run:

```bash
cd backend && go test -tags=unit ./internal/repository -run TestUsageLogRepositoryGetProfitabilityTrend
```

Expected:

- PASS，订阅收入与订阅成本同桶聚合后成本非零

Run:

```bash
cd backend && go test -tags=integration ./internal/repository -run TestMigrationsSchema
```

Expected:

- PASS，新 migration 不破坏现有 schema 与数据读取

## Task 4: 收敛前端展示语义，消除“0%~1% 空轴假象”

**Files:**
- Modify: `frontend/src/views/admin/dashboardProfitability.ts`
- Modify: `frontend/src/components/charts/ProfitabilityTrendChart.vue`
- Modify: `frontend/src/views/admin/DashboardView.vue`
- Modify: `frontend/src/i18n/locales/zh.ts`
- Modify: `frontend/src/i18n/locales/en.ts`
- Modify: `frontend/src/views/admin/__tests__/dashboardProfitability.spec.ts`

**Step 1: 只在存在可计算盈利率点时渲染 rate dataset / yRate 轴**

- 如果当前区间所有点都是 `extra_profit_rate_percent = null`
  - 不渲染紫色折线
  - 不渲染右侧百分比轴

**Step 2: 让摘要卡片和图表语义一致**

- 摘要仍显示 `--`
- 图表不再给出 `0% ~ 1%` 的假刻度

**Step 3: 校正文案，说明这是“面板统计口径中的利润率”，不是分组配置表里的静态额外盈利率**

- 当前文案会让人误以为这是“组配置的额外盈利率(%)”直接回显
- 修复时至少要在描述里说明：
  - 它是按面板收入/成本实时计算出来的
  - 没有成本时不显示

**Step 4: 运行前端测试与类型检查**

Run:

```bash
cd frontend && pnpm test:run dashboardProfitability.spec.ts
```

Expected:

- PASS，不再出现全空 rate 轴

Run:

```bash
cd frontend && pnpm run typecheck
```

Expected:

- PASS

## Task 5: 远端回归验证

**Files:**
- No source file changes in this task

**Step 1: 回放当前远端复现窗口**

重点验证：

- `GET /api/v1/admin/dashboard/profitability?start_date=2026-04-13&end_date=2026-04-19&granularity=day`

**Step 2: 观察两个关键断言**

- `estimated_cost_cny` 不再整段为 `0`
- 至少在 `2026-04-17` 与 `2026-04-18` 这类存在大量非管理员订阅 usage 的日期，成本应为正数

**Step 3: 页面肉眼验收**

- 顶部卡片的“估算成本”不再固定为 `¥0.0000`
- 图表右轴不再在“无盈利率数据”时显示 `0% ~ 1%`

**Step 4: 全量检查**

Run:

```bash
make test
```

Expected:

- 通过；若时间过长，至少分别完成 backend unit、repository integration、frontend test/typecheck

## 风险与回滚点

- 历史订阅 usage 的 `estimated_cost_cny` 回填如果使用当前账号成本配置，理论上会有“历史口径近似而非原始快照”的风险。
- 为降低这个风险，Task 3 同时要求：
  - 先补写未来数据快照
  - 再对历史数据做一次性回填
  - 查询层保留订阅历史兜底，避免面板再次掉回 0 成本
- 如果产品决定该面板不应混合订阅收入与余额盈利率，需要把 Task 3 改成“删除 subscription revenue 聚合”，那将是另一条更窄但更保守的方案。

## 完成定义

- 远端 profitability 接口在 `2026-04-13 ~ 2026-04-19` 窗口内返回非零成本
- 订阅 usage 写入链路能稳定填 `estimated_cost_cny`
- 历史缺口已通过 migration/backfill 或查询兜底补齐
- 前端不再把“全空 rate 序列”误画成 `0% ~ 1%`
- 后端与前端相关测试全部通过
