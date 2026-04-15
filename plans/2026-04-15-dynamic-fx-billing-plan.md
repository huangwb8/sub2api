# 动态汇率按量计费优化计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 为非订阅按量计费链路引入“在线汇率 + 本地保守兜底”的人民币扣费机制，让用户继续以人民币充值，但系统内部按美元计算 usage 成本，并在最终扣减余额时安全、可追溯地换算成人民币。

**Architecture:** 当前系统的 usage 成本、quota、rate limit 明确按 `USD` 建模，但余额、支付、订单语义已经被统一到 `CNY`。下一步不应再让 `actual_cost_usd` 直接数值扣减 `balance`，而是要引入一层汇率结算服务：先按上游模型价格得到 `actual_cost_usd`，再通过“在线汇率 / 最近成功汇率 / 固定底线汇率”得到 `effective_rate`，计算 `charged_amount_cny`，最后只从人民币余额账本扣人民币，并把汇率快照落到 usage 记录里，确保可审计、可复算、可解释。

**Tech Stack:** Go, Gin, Ent ORM, PostgreSQL, req/v3, Vue 3, TypeScript, Vitest, Go test

---

## 背景判断

### 当前问题

当前仓库里，按量扣费链路会把 `ActualCost` 直接从 `balance` 里减掉，而不是先做美元到人民币换算：

- `backend/internal/service/usage_service.go`
- `backend/internal/service/gateway_service.go`
- `backend/internal/service/openai_gateway_record_usage_test.go`

与此同时，以下语义已经比较明确：

- 用户余额、充值金额、订单支付金额：按 `CNY`
- usage cost、quota、rate limit、`*_usd`：按 `USD`

这意味着目前系统存在一个危险的“半统一状态”：

- 前台把余额展示成人民币
- 后台却可能用美元数值直接扣同一个 `balance`

这个状态不适合继续放大，否则会带来：

- 用户对账困难
- 退款和补偿难以解释
- 财务与运营口径不一致
- 汇率波动时平台利润不可控

## 设计目标

### 必须达到的目标

1. 用户充值和余额账本始终使用人民币。
2. usage 成本和额度限制继续使用美元，不破坏现有模型定价体系。
3. 只有在实际扣款时，才把 `actual_cost_usd` 转为 `charged_amount_cny`。
4. 汇率获取失败时，系统仍能稳定运行。
5. 平台默认采取“偏保守”的结算策略，优先降低因汇率波动造成的损失。
6. 每笔扣费都能回溯当时采用的汇率、来源和换算结果。

### 明确不做的事

- 不把 usage / quota / rate limit 全部改成人民币
- 不引入多币种用户钱包
- 不重写整套支付系统
- 不修改历史 usage 的美元成本字段含义
- 不在这一轮把所有前端展示都做成动态汇率实时波动展示

## 推荐结算策略

### 单一记账原则

- `balance` 只表示 `CNY`
- `actual_cost` / `total_cost` 继续表示 `USD`
- 新增 `charged_amount_cny` 作为最终扣减余额的记账金额

### 汇率决策规则

建议使用以下保守结算规则：

```text
effective_rate = max(live_rate, last_success_rate, 7.2) * (1 + safety_margin)
```

建议默认值：

- `fallback_floor_rate = 7.2`
- `safety_margin = 0.02`
- `cache_ttl = 10m`
- `request_timeout = 3s`

### 这样做的原因

- `live_rate`：尽量贴近市场
- `last_success_rate`：避免在线接口短暂故障时掉到过低汇率
- `7.2`：提供硬底线保护
- `safety_margin`：覆盖支付摩擦、汇兑损耗和短时波动

如果你最关心“尽量别亏”，这个方案比“失败就固定 7.2”更稳。

## 建议实现路径

### Task 1：定义汇率结算领域模型

**Files:**

- Modify: `backend/internal/service/domain_constants.go`
- Modify: `backend/internal/service/setting_service.go`
- Modify: `backend/internal/handler/dto/settings.go`
- Modify: `backend/internal/service/settings_view.go`
- Test: `backend/internal/service/setting_service_update_test.go`
- Test: `backend/internal/service/setting_service_public_test.go`

**目标：**

在系统设置中新增并统一管理以下参数：

- `billing_fx_enabled`
- `billing_fx_provider`
- `billing_fx_fallback_rate`
- `billing_fx_cache_ttl_seconds`
- `billing_fx_timeout_ms`
- `billing_fx_safety_margin`
- `billing_fx_last_success_rate`
- `billing_fx_last_success_at`

**说明：**

- `last_success_rate` / `last_success_at` 可以先放 settings 表，优先保证实现简单稳定
- 后续如果需要更强运维能力，再抽成独立运行时缓存或专门表

### Task 2：新增汇率获取与缓存服务

**Files:**

- Create: `backend/internal/service/exchange_rate_service.go`
- Create: `backend/internal/service/exchange_rate_service_test.go`
- Modify: `backend/internal/service/setting_service.go`
- Modify: `backend/internal/service/usage_billing.go`
- Modify: `backend/internal/setup/` 下对应 Wire 装配文件

**目标：**

提供一个单一入口，例如：

```go
type ExchangeRateService interface {
    ResolveUSDCNYRate(ctx context.Context) (*ResolvedExchangeRate, error)
}
```

建议返回结构：

```go
type ResolvedExchangeRate struct {
    LiveRate        *float64
    LastSuccessRate *float64
    FloorRate       float64
    SafetyMargin    float64
    EffectiveRate   float64
    Source          string
    FetchedAt       time.Time
}
```

**实现要求：**

- 带进程内 TTL 缓存，避免每次请求都打外部接口
- 带 singleflight，避免缓存击穿
- 外部 HTTP 请求有短超时
- 在线失败时自动回退到最近成功值或底线值
- 若在线值成功，应刷新 `last_success_rate`

**Provider 策略建议：**

- 先做一个可插拔 provider 接口
- 第一阶段只接一个默认 provider 即可
- 不在这一轮绑定太多第三方

### Task 3：改造按量扣费命令，区分 USD 成本与 CNY 扣款

**Files:**

- Modify: `backend/internal/service/usage_billing.go`
- Modify: `backend/internal/service/gateway_service.go`
- Modify: `backend/internal/service/usage_service.go`
- Modify: `backend/internal/service/openai_gateway_service.go`
- Test: `backend/internal/service/openai_gateway_record_usage_test.go`
- Test: `backend/internal/service/gateway_record_usage_test.go`

**目标：**

把按量扣费链路拆成两个语义层：

- `ActualCostUSD`
- `BalanceDeductCNY`

建议方向：

- `UsageBillingCommand.BalanceCost` 不再表达“美元成本”
- 改成显式区分：
  - `BalanceCostUSD`
  - `BalanceCostCNY`
  - `ExchangeRateUsed`
  - `ExchangeRateSource`

如果暂时不想大改命令结构，至少要保证：

- 最终传给 `DeductBalance` 的值只能是 `CNY`
- quota / rate limit 仍继续走 `USD`

### Task 4：为 usage log 增加汇率与人民币扣款快照

**Files:**

- Modify: `backend/ent/schema/usage_log.go`
- Modify: `backend/internal/service/usage_log.go`
- Modify: `backend/internal/service/usage_service.go`
- Modify: `backend/internal/handler/dto/mappers.go`
- Modify: `backend/internal/handler/dto/types.go`
- Modify: `backend/internal/repository/usage_log_repo.go`
- Create: `backend/migrations/XXX_add_usage_fx_snapshot.sql`
- Run: `cd backend && go generate ./ent`

**建议新增字段：**

- `actual_cost_usd`
- `charged_amount_cny`
- `fx_rate_usd_cny`
- `fx_rate_source`
- `fx_fetched_at`
- `fx_safety_margin`

**兼容策略：**

- 若当前 `actual_cost` 已被外部依赖为美元，可保留它并新增 `charged_amount_cny`
- 不建议直接改老字段语义，避免报表和前端静默错位

### Task 5：补齐后台和前台的解释性展示

**Files:**

- Modify: `frontend/src/views/user/UsageView.vue`
- Modify: `frontend/src/views/user/ProfileView.vue`
- Modify: `frontend/src/views/user/UserOrdersView.vue`
- Modify: `frontend/src/components/admin/payment/AdminOrderDetail.vue`
- Modify: `frontend/src/components/admin/user/UserBalanceHistoryModal.vue`
- Modify: `frontend/src/utils/format.ts`
- Modify: `frontend/src/types/` 下与 usage / order / settings 对应的类型文件
- Test: `frontend/src/views/user/__tests__/UsageView.spec.ts`

**目标：**

前台仍维持现有原则：

- 余额显示 `CNY`
- usage cost 显示 `USD`

但在需要解释扣费的地方增加清晰说明，例如：

- “本次按量成本：$0.42”
- “按结算汇率 7.36 扣减：¥3.15”

管理后台可额外显示：

- 汇率来源
- 结算时间
- 安全加价比例

### Task 6：提供安全的默认值与降级行为

**Files:**

- Modify: `backend/internal/config/config.go`
- Modify: `backend/internal/service/setting_service.go`
- Modify: `backend/internal/service/payment_config_service.go`
- Test: `backend/internal/config/config_test.go`
- Test: `backend/internal/service/setting_service_update_test.go`

**目标：**

即使管理员完全没配置汇率功能，系统也不应该中断。

推荐默认行为：

- `billing_fx_enabled = true`
- `billing_fx_provider = "default"`
- `billing_fx_fallback_rate = 7.2`
- `billing_fx_safety_margin = 0.02`
- `billing_fx_cache_ttl_seconds = 600`
- `billing_fx_timeout_ms = 3000`

如果在线接口持续失败：

- 继续用 `max(last_success_rate, fallback_rate)` 结算
- 打 warning 日志
- 保持请求可继续完成，不阻断用户调用

### Task 7：测试与验收

**Files:**

- Test: `backend/internal/service/exchange_rate_service_test.go`
- Test: `backend/internal/service/openai_gateway_record_usage_test.go`
- Test: `backend/internal/service/gateway_record_usage_test.go`
- Test: `backend/internal/repository/usage_log_repo_integration_test.go`
- Test: `frontend/src/views/user/__tests__/UsageView.spec.ts`

**必须覆盖的场景：**

1. 在线汇率获取成功，按在线值结算
2. 在线获取失败，回退到最近成功汇率
3. 在线与最近成功都不可用，回退到 `7.2`
4. 在线汇率低于 `7.2`，仍使用 `7.2`
5. 安全加价被正确应用
6. quota/rate limit 仍按美元累计，不受人民币扣款影响
7. 用户余额扣减的是人民币值，而不是美元值
8. usage log 能正确记录美元成本与人民币扣款快照
9. 前端展示不会把 usage cost 错误显示成人民币

## 推荐实施顺序

### 第一阶段：先把账本语义修正过来

- 加汇率服务
- 在扣费链路里从“美元直扣余额”改成“美元转人民币后扣余额”
- 不急着先改很多 UI

### 第二阶段：补 usage log 汇率快照

- 做迁移
- 把每次扣费的汇率细节记录下来
- 补回归测试

### 第三阶段：补前台解释性展示和后台运维可见性

- 前台给用户明确“美元成本”和“人民币扣款”区别
- 后台可查看汇率来源与兜底状态

## 风险与注意事项

### 风险 1：历史字段语义漂移

如果直接把 `actual_cost` 改成“人民币扣款”，会破坏已有报表和前端逻辑。  
建议保留 `actual_cost` 的美元语义，新增人民币快照字段。

### 风险 2：幂等扣费与汇率不一致

同一请求重试时，不能因为汇率在几秒内变化就导致重复请求扣出不同人民币金额。  
建议在第一次生成 usage billing command 时就固化 `effective_rate` 和 `charged_amount_cny`，后续幂等重试直接复用。

### 风险 3：过度依赖外部汇率接口

在线汇率服务只能是增强项，不能成为单点故障。  
所以必须有：

- TTL 缓存
- 最近成功值
- 固定底线值

### 风险 4：前端误导

如果用户只能看到“余额减少了 ¥3.15”，但看不到“这次 usage 是 $0.42 按 7.36 汇率结算”，仍然可能产生疑问。  
建议在 usage 明细或账单详情里提供解释。

## 验收标准

满足以下条件即可认为方案达标：

1. 用户充值与余额全程按人民币记账。
2. 模型成本、quota、rate limit 继续按美元工作。
3. 按量请求不会再用美元数值直接扣人民币余额。
4. 汇率在线失败时系统仍可稳定扣费。
5. 平台默认采用偏保守汇率策略，不会因为临时失败掉到明显偏低汇率。
6. 每笔 usage 都能追溯当时使用的汇率和最终人民币扣款金额。
7. 不破坏现有订阅计费、支付下单、额度限制和 usage 报表主流程。

## 我建议的最终方案

如果只选一个最优方案，我建议：

- 人民币余额账本
- 美元 usage 成本
- `effective_rate = max(live_rate, last_success_rate, 7.2) * 1.02`
- 每笔 usage 落汇率快照
- 汇率服务失败不阻断请求

这套方案在实现复杂度、可解释性和防亏能力之间比较平衡，适合现在这套系统继续演进。
