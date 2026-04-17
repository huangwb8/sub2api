# 订阅补差价升级实现计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 允许普通用户把当前较低档订阅升级为更高档订阅，按旧订阅剩余价值自动折抵，仅支付差额；支付方式默认优先余额，不足时可继续使用现有扫码/Stripe 支付链路。

**Architecture:** 复用现有内置支付订单链路，不新建平行支付系统，但要补齐两层关键建模。第一层是在订阅套餐上显式定义“升级梯度”，避免把 UI 排序误当业务等级；第二层是在用户订阅记录里持久化当前计费套餐快照，否则系统无法准确计算旧套餐剩余可抵扣价值。升级订单使用新的 `subscription_upgrade` 类型，在创建订单时冻结差价快照，支付完成后原子地停用旧订阅并开通新订阅；自动退款先按“禁止自动恢复旧订阅、仅允许人工处理”收口，优先保证正确性。

**Tech Stack:** Go, Gin, Ent ORM, PostgreSQL, Vue 3, TypeScript, Pinia, pnpm, Go test, Vitest

---

## 背景判断

### 当前实现为什么不能直接支持补差价升级

当前仓库已经支持：

- 普通用户自助购买订阅
- 同分组订阅续费
- 余额支付订阅
- 第三方扫码支付订阅

但还缺 3 个升级必需条件：

1. `user_subscriptions` 只记录 `group_id`，不记录用户当前到底买的是哪一个套餐。
2. `payment/orders` 只有 `balance` / `subscription` 两类订单，没有“升级订单”的来源订阅、折抵金额、目标套餐等快照。
3. 套餐“高低”目前没有业务字段，不能拿 `sort_order` 或页面顺序代替升级等级。

### 这一轮建议采用的升级语义

本计划采用下面这套更容易落地且和当前链路兼容的升级语义：

- 仅允许从低等级套餐升级到高等级套餐。
- 不允许反向降级。
- 仅允许在同一 `upgrade_family` 内升级。
- 升级成功后，旧订阅立即结束，新订阅从“支付完成时”开始按目标套餐完整有效期重新生效。
- 旧订阅未使用部分按剩余时间比例折抵到目标套餐价格中。
- 差价在“创建订单时”冻结，不随支付等待过程继续变化。

推荐折抵公式：

```text
remaining_ratio = clamp((source_expires_at - now) / (source_cycle_end - source_cycle_start), 0, 1)
credit_cny      = round(source_plan_price_cny * remaining_ratio, 2)
payable_cny     = max(round(target_plan_price_cny - credit_cny, 2), 0)
```

其中：

- `source_cycle_start` 不能再依赖模糊的历史推断，必须落库保存。
- `source_plan_price_cny` 必须来自订阅快照，不能事后去查当前套餐价格。

### 本期边界

本期明确支持：

- 普通用户把一个活跃订阅升级到同升级族内更高档套餐
- 余额支付差价
- 余额不足时改用现有扫码支付链路
- 零差价时走内部直接完成

本期明确不做：

- 反向降级
- 把多个低档订阅合并升级为一个高档订阅
- 自动把升级订单的退款恢复成原订阅状态
- 在无套餐快照的历史遗留订阅上强行开放升级

---

### Task 1: 给订阅套餐补上“升级梯度”元数据

**Files:**
- Modify: `backend/ent/schema/subscription_plan.go`
- Modify: `backend/internal/service/payment_config_service.go`
- Modify: `backend/internal/service/payment_config_plans.go`
- Modify: `backend/internal/handler/admin/payment_handler.go`
- Modify: `backend/internal/handler/payment_handler.go`
- Modify: `frontend/src/types/payment.ts`
- Modify: `frontend/src/views/admin/orders/AdminPaymentPlansView.vue`
- Test: `backend/internal/service/payment_config_plans_test.go`
- Test: `backend/internal/handler/payment_handler_test.go`
- Create: `backend/migrations/103_add_subscription_plan_upgrade_metadata.sql`

**Step 1: Write the failing tests**

补测试覆盖以下规则：

- 套餐可以显式配置 `upgrade_family` 与 `upgrade_rank`
- `upgrade_family` 为空时视为不参与升级链
- 同一升级链内，`upgrade_rank` 必须是非负整数
- 用户侧 checkout 返回升级字段，管理后台 CRUD 也会透传这些字段

建议新增断言示例：

```go
plan, err := svc.CreatePlan(ctx, CreatePlanRequest{
    GroupID:       activeSubGroup.ID,
    Name:          "Pro",
    Price:         199,
    ValidityDays:  30,
    ValidityUnit:  "day",
    UpgradeFamily: "openai-team",
    UpgradeRank:   20,
})
require.NoError(t, err)
require.Equal(t, "openai-team", plan.UpgradeFamily)
require.Equal(t, 20, plan.UpgradeRank)
```

**Step 2: Run test to verify it fails**

Run:

```bash
cd backend && go test -tags=unit ./internal/service ./internal/handler -run 'TestPayment(ConfigService|Handler).*Plan'
```

Expected: FAIL，因为当前 schema / handler / request 里都没有升级字段。

**Step 3: Write minimal implementation**

新增以下字段：

```go
field.String("upgrade_family").Default("")
field.Int("upgrade_rank").Default(0)
```

同时扩展：

- `CreatePlanRequest`
- `UpdatePlanRequest`
- 管理后台 `subscriptionPlanResponse`
- 用户侧 `checkoutPlan`
- 管理后台套餐编辑表单

重要约束：

- 不要复用 `sort_order` 代表升级高低
- `upgrade_family` 只表达“同一升级族”
- `upgrade_rank` 只表达“升级高低”

**Step 4: Run test to verify it passes**

Run:

```bash
cd backend && go test -tags=unit ./internal/service ./internal/handler -run 'TestPayment(ConfigService|Handler).*Plan'
```

Expected: PASS

**Step 5: Generate Ent code and commit**

Run:

```bash
cd backend && go generate ./ent
git add backend/ent backend/ent/schema/subscription_plan.go backend/internal/service/payment_config_service.go backend/internal/service/payment_config_plans.go backend/internal/handler/admin/payment_handler.go backend/internal/handler/payment_handler.go frontend/src/types/payment.ts frontend/src/views/admin/orders/AdminPaymentPlansView.vue backend/migrations/103_add_subscription_plan_upgrade_metadata.sql backend/internal/service/payment_config_plans_test.go backend/internal/handler/payment_handler_test.go
git commit -m "feat: add subscription upgrade ladder metadata"
```

---

### Task 2: 给用户订阅补上当前套餐快照，解决“差价无法精算”的根问题

**Files:**
- Modify: `backend/ent/schema/user_subscription.go`
- Modify: `backend/internal/service/user_subscription.go`
- Modify: `backend/internal/repository/user_subscription_repo.go`
- Modify: `backend/internal/service/subscription_service.go`
- Modify: `backend/internal/service/payment_fulfillment.go`
- Modify: `backend/internal/service/payment_order.go`
- Test: `backend/internal/service/payment_fulfillment_test.go`
- Test: `backend/internal/service/subscription_assign_idempotency_test.go`
- Create: `backend/migrations/104_add_user_subscription_plan_snapshot.sql`

**Step 1: Write the failing tests**

补测试覆盖以下行为：

- 用户购买订阅后，`user_subscriptions` 持久化当前套餐快照
- 同分组续费时，快照更新为最新购买套餐
- 非支付来源的管理员分配/兑换码分配仍可继续工作，快照允许为空

建议快照字段：

```go
CurrentPlanID            *int64
CurrentPlanName          string
CurrentPlanPriceCNY      *float64
CurrentPlanValidityDays  *int
CurrentPlanValidityUnit  string
BillingCycleStartedAt    *time.Time
```

**Step 2: Run test to verify it fails**

Run:

```bash
cd backend && go test -tags=unit ./internal/service -run 'TestPaymentService_(CreateOrderSubscription|ExecuteSubscriptionFulfillment)|TestSubscriptionService_'
```

Expected: FAIL，因为当前订阅模型没有任何套餐快照字段。

**Step 3: Write minimal implementation**

实现方向：

- 扩展 `AssignSubscriptionInput`，允许可选传入套餐快照
- `createSubscription()` 创建时写入快照
- `AssignOrExtendSubscription()` 续费时更新快照与 `billing_cycle_started_at`
- 普通购买 / 余额购买订阅时都把套餐快照传进去
- 管理员手动分配、兑换码等无套餐语义的路径，不强制要求快照

关键规则：

- 升级功能只对“有套餐快照的活跃订阅”开放
- 对历史遗留无快照订阅，前端展示“暂不支持升级，请联系客服/管理员手工迁移”

**Step 4: Run test to verify it passes**

Run:

```bash
cd backend && go test -tags=unit ./internal/service -run 'TestPaymentService_(CreateOrderSubscription|ExecuteSubscriptionFulfillment)|TestSubscriptionService_'
```

Expected: PASS

**Step 5: Generate Ent code and commit**

Run:

```bash
cd backend && go generate ./ent
git add backend/ent backend/ent/schema/user_subscription.go backend/internal/service/user_subscription.go backend/internal/repository/user_subscription_repo.go backend/internal/service/subscription_service.go backend/internal/service/payment_fulfillment.go backend/internal/service/payment_order.go backend/migrations/104_add_user_subscription_plan_snapshot.sql backend/internal/service/payment_fulfillment_test.go backend/internal/service/subscription_assign_idempotency_test.go
git commit -m "feat: persist subscription plan snapshot for upgrades"
```

---

### Task 3: 新增升级报价服务和用户侧预览接口

**Files:**
- Create: `backend/internal/service/subscription_upgrade_service.go`
- Create: `backend/internal/service/subscription_upgrade_service_test.go`
- Modify: `backend/internal/service/payment_service.go`
- Modify: `backend/internal/handler/subscription_handler.go`
- Modify: `backend/internal/server/routes/user.go`
- Modify: `frontend/src/api/subscriptions.ts`
- Modify: `frontend/src/stores/subscriptions.ts`
- Modify: `frontend/src/types/index.ts`

**Step 1: Write the failing tests**

新增服务级测试，覆盖：

- 同 `upgrade_family` 内只能从低 rank 升到高 rank
- 目标套餐必须 `for_sale`
- 源订阅必须活跃且带快照
- 已有目标分组活跃订阅时拒绝升级
- 计算出的 `credit_cny` 与 `payable_cny` 四舍五入到 2 位小数

建议服务接口：

```go
type SubscriptionUpgradeService interface {
    ListUpgradeOptions(ctx context.Context, userID, sourceSubscriptionID int64) ([]UpgradeOption, error)
    BuildUpgradeQuote(ctx context.Context, userID, sourceSubscriptionID, targetPlanID int64, now time.Time) (*UpgradeQuote, error)
}
```

**Step 2: Run test to verify it fails**

Run:

```bash
cd backend && go test -tags=unit ./internal/service -run 'TestSubscriptionUpgradeService'
```

Expected: FAIL，因为升级服务和接口尚不存在。

**Step 3: Write minimal implementation**

新增对外只读接口：

- `GET /api/v1/subscriptions/:id/upgrade-options`

推荐返回结构：

```json
{
  "source_subscription_id": 12,
  "source_group_id": 3,
  "source_plan_id": 21,
  "source_plan_name": "Basic",
  "remaining_ratio": 0.42,
  "credit_cny": 41.58,
  "options": [
    {
      "target_plan_id": 22,
      "target_group_id": 4,
      "target_plan_name": "Pro",
      "target_price_cny": 99,
      "payable_cny": 57.42,
      "default_payment_type": "balance"
    }
  ]
}
```

默认支付方式逻辑：

- 若用户余额 `>= payable_cny`，返回 `default_payment_type = balance`
- 否则只把它标成“推荐余额不可用”，由前端切换到外部支付方式

**Step 4: Run test to verify it passes**

Run:

```bash
cd backend && go test -tags=unit ./internal/service -run 'TestSubscriptionUpgradeService'
```

然后补一轮 handler 验证：

```bash
cd backend && go test -tags=unit ./internal/handler -run 'TestSubscriptionHandler'
```

Expected: PASS

**Step 5: Commit**

Run:

```bash
git add backend/internal/service/subscription_upgrade_service.go backend/internal/service/subscription_upgrade_service_test.go backend/internal/service/payment_service.go backend/internal/handler/subscription_handler.go backend/internal/server/routes/user.go frontend/src/api/subscriptions.ts frontend/src/stores/subscriptions.ts frontend/src/types/index.ts
git commit -m "feat: add subscription upgrade quote API"
```

---

### Task 4: 新增 `subscription_upgrade` 订单类型并冻结差价快照

**Files:**
- Modify: `backend/internal/payment/types.go`
- Modify: `backend/ent/schema/payment_order.go`
- Modify: `backend/internal/service/payment_service.go`
- Modify: `backend/internal/service/payment_order.go`
- Modify: `backend/internal/handler/payment_handler.go`
- Modify: `frontend/src/types/payment.ts`
- Modify: `frontend/src/api/payment.ts`
- Test: `backend/internal/service/payment_order_test.go`
- Test: `backend/internal/service/payment_balance_subscription_test.go`
- Create: `backend/migrations/105_add_subscription_upgrade_order_fields.sql`

**Step 1: Write the failing tests**

覆盖以下行为：

- `order_type=subscription_upgrade` 时必须同时提交 `plan_id` 与 `source_subscription_id`
- 创建订单时重新服务端计算差价，不能信任前端传入金额
- 订单金额应写入“冻结后的 payable_cny”
- 同一来源订阅存在未完成升级订单时拒绝重复下单
- 差价为 `0` 时允许生成内部免支付订单并直接完成

建议扩展请求：

```ts
export type OrderType = 'balance' | 'subscription' | 'subscription_upgrade'

export interface CreateOrderRequest {
  amount: number
  payment_type: string
  order_type: string
  plan_id?: number
  source_subscription_id?: number
}
```

建议扩展订单快照字段：

```go
field.Int64("source_subscription_id").Optional().Nillable()
field.Int64("source_plan_id").Optional().Nillable()
field.Float("upgrade_credit_cny").Optional().Nillable()
field.Float("upgrade_payable_cny").Optional().Nillable()
field.Float("upgrade_remaining_ratio").Optional().Nillable()
```

**Step 2: Run test to verify it fails**

Run:

```bash
cd backend && go test -tags=unit ./internal/service -run 'TestPaymentService_(CreateOrderSubscriptionUpgrade|CreateBalanceSubscriptionOrder)'
```

Expected: FAIL，因为当前订单模型和请求都不认识升级订单。

**Step 3: Write minimal implementation**

实现要求：

- 新增 `payment.OrderTypeSubscriptionUpgrade`
- 创建升级订单时忽略前端 `amount`，始终使用后端重算后的差价
- `payment_type` 默认仍可传 `balance`
- 若余额不足并且前端仍传 `balance`，返回明确错误而不是悄悄切别的支付方式
- 外部扫码支付走和普通订阅订单相同的 provider 选择逻辑

建议新增内部方法：

```go
func (s *PaymentService) createSubscriptionUpgradeOrder(ctx context.Context, req CreateOrderRequest, user *User, cfg *PaymentConfig) (*CreateOrderResponse, error)
```

**Step 4: Run test to verify it passes**

Run:

```bash
cd backend && go test -tags=unit ./internal/service -run 'TestPaymentService_(CreateOrderSubscriptionUpgrade|CreateBalanceSubscriptionOrder)'
```

Expected: PASS

**Step 5: Generate Ent code and commit**

Run:

```bash
cd backend && go generate ./ent
git add backend/ent backend/ent/schema/payment_order.go backend/internal/payment/types.go backend/internal/service/payment_service.go backend/internal/service/payment_order.go backend/internal/handler/payment_handler.go frontend/src/types/payment.ts frontend/src/api/payment.ts backend/internal/service/payment_order_test.go backend/internal/service/payment_balance_subscription_test.go backend/migrations/105_add_subscription_upgrade_order_fields.sql
git commit -m "feat: add subscription upgrade order creation"
```

---

### Task 5: 实现升级履约，原子地停旧开新

**Files:**
- Modify: `backend/internal/service/payment_fulfillment.go`
- Modify: `backend/internal/service/subscription_service.go`
- Modify: `backend/internal/repository/user_subscription_repo.go`
- Modify: `backend/internal/service/payment_refund.go`
- Test: `backend/internal/service/payment_fulfillment_test.go`
- Test: `backend/internal/service/payment_refund_test.go`

**Step 1: Write the failing tests**

新增履约测试覆盖：

- 升级订单支付成功后，旧订阅变为 `revoked` 或立即过期
- 新订阅按目标套餐完整时长创建
- 同一升级订单重试不会重复创建多个目标订阅
- 升级订单默认不走自动恢复型退款

建议新增履约分支：

```go
switch o.OrderType {
case payment.OrderTypeSubscriptionUpgrade:
    return s.ExecuteSubscriptionUpgradeFulfillment(ctx, oid)
case payment.OrderTypeSubscription:
    return s.ExecuteSubscriptionFulfillment(ctx, oid)
default:
    return s.ExecuteBalanceFulfillment(ctx, oid)
}
```

**Step 2: Run test to verify it fails**

Run:

```bash
cd backend && go test -tags=unit ./internal/service -run 'TestPaymentService_(ExecuteSubscriptionUpgradeFulfillment|RetryFulfillment|ExecuteRefund)'
```

Expected: FAIL，因为当前只有充值和普通订阅履约。

**Step 3: Write minimal implementation**

履约事务建议按下面顺序执行：

1. 锁定升级订单。
2. 读取并锁定来源订阅。
3. 再次验证来源订阅仍与订单快照一致。
4. 若已存在 `UPGRADE_SUCCESS` 审计记录，直接幂等完成。
5. 将来源订阅标记为 `revoked`，备注写入订单号和目标套餐。
6. 创建目标订阅，并写入目标套餐快照。
7. 记录 `UPGRADE_SUCCESS` 审计日志。
8. 标记订单 `COMPLETED`。

退款策略：

- `subscription_upgrade` 订单先禁止自动化“恢复旧订阅”退款
- 用户侧若有退款入口，升级订单直接返回“请联系管理员人工处理”
- 管理后台可以做金额退款，但不自动逆向恢复订阅状态

这样可以先避免“退款把旧订阅恢复错”的高风险逻辑。

**Step 4: Run test to verify it passes**

Run:

```bash
cd backend && go test -tags=unit ./internal/service -run 'TestPaymentService_(ExecuteSubscriptionUpgradeFulfillment|RetryFulfillment|ExecuteRefund)'
```

Expected: PASS

**Step 5: Commit**

Run:

```bash
git add backend/internal/service/payment_fulfillment.go backend/internal/service/subscription_service.go backend/internal/repository/user_subscription_repo.go backend/internal/service/payment_refund.go backend/internal/service/payment_fulfillment_test.go backend/internal/service/payment_refund_test.go
git commit -m "feat: fulfill subscription upgrade orders atomically"
```

---

### Task 6: 做用户侧升级 UI，默认余额支付，不足时切换扫码支付

**Files:**
- Modify: `frontend/src/views/user/SubscriptionsView.vue`
- Modify: `frontend/src/views/user/PaymentView.vue`
- Modify: `frontend/src/components/payment/SubscriptionPlanCard.vue`
- Modify: `frontend/src/stores/payment.ts`
- Modify: `frontend/src/stores/subscriptions.ts`
- Modify: `frontend/src/i18n/locales/zh.ts`
- Modify: `frontend/src/i18n/locales/en.ts`
- Test: `frontend/src/stores/__tests__/subscriptions.spec.ts`
- Test: `frontend/src/utils/__tests__/subscriptionPlan.spec.ts`

**Step 1: Write the failing test**

如果现有前端测试覆盖不足，至少补 store / util 层测试，覆盖：

- 升级选项接口结果能正确入库
- 当 `default_payment_type=balance` 且余额足够时，升级面板默认选余额
- 当余额不足时，自动切到第一个外部支付方式

**Step 2: Run test to verify it fails**

Run:

```bash
cd frontend && pnpm test -- --runInBand subscriptions subscriptionPlan
```

如果仓库当前测试脚本不支持精确过滤，就退而求其次先执行：

```bash
cd frontend && pnpm test
```

Expected: FAIL，至少因为升级类型和升级接口尚未接入。

**Step 3: Write minimal implementation**

推荐交互：

- 在 [订阅列表] 页面给每个可升级的活跃订阅显示 `升级套餐`
- 点击后弹出升级面板，展示：
  - 当前套餐
  - 剩余到期时间
  - 可升级目标套餐
  - 折抵金额
  - 需补差价
  - 默认支付方式
- 确认后直接创建 `subscription_upgrade` 订单

前端规则：

- 如果 `payable_cny === 0`，直接走内部完成态
- 默认优先余额
- 用户手动切到支付宝/微信/Stripe 时，沿用已有支付弹窗/二维码逻辑
- 成功后刷新余额和活跃订阅列表

**Step 4: Run verification**

Run:

```bash
cd frontend && pnpm run typecheck
cd frontend && pnpm run lint:check
cd frontend && pnpm test
```

Expected: PASS

**Step 5: Commit**

Run:

```bash
git add frontend/src/views/user/SubscriptionsView.vue frontend/src/views/user/PaymentView.vue frontend/src/components/payment/SubscriptionPlanCard.vue frontend/src/stores/payment.ts frontend/src/stores/subscriptions.ts frontend/src/i18n/locales/zh.ts frontend/src/i18n/locales/en.ts frontend/src/stores/__tests__/subscriptions.spec.ts frontend/src/utils/__tests__/subscriptionPlan.spec.ts
git commit -m "feat: add user subscription upgrade flow"
```

---

### Task 7: 补文档、接口说明与回归测试

**Files:**
- Modify: `docs/PAYMENT_CN.md`
- Modify: `README.md`
- Modify: `CHANGELOG.md`
- Modify: `backend/internal/server/api_contract_test.go`

**Step 1: Write the failing check**

先搜当前文档里是否提到升级限制或仍只写“购买/续费”：

```bash
rg -n "续费|购买订阅|升级|subscription_upgrade" docs/PAYMENT_CN.md README.md CHANGELOG.md backend/internal/server/api_contract_test.go
```

Expected: 能搜到旧描述，但没有升级差价语义。

**Step 2: Update docs**

补上以下文档点：

- 仅支持低升高，不支持降级
- 默认余额支付，不足时可扫码
- 历史无快照订阅可能暂不支持升级
- 升级退款需人工处理

**Step 3: Run verification**

Run:

```bash
cd backend && go test -tags=unit ./internal/server -run TestAPIContract
rg -n "subscription_upgrade|补差价|低升高|人工处理" docs/PAYMENT_CN.md README.md CHANGELOG.md
```

Expected: PASS

**Step 4: Commit**

Run:

```bash
git add docs/PAYMENT_CN.md README.md CHANGELOG.md backend/internal/server/api_contract_test.go
git commit -m "docs: describe subscription upgrade difference payment"
```

---

### Task 8: 最终联调与风险兜底

**Files:**
- Verify only

**Step 1: Run backend checks**

```bash
cd backend && go generate ./ent
cd backend && go test -tags=unit ./internal/service ./internal/handler ./internal/server
```

**Step 2: Run frontend checks**

```bash
cd frontend && pnpm run typecheck
cd frontend && pnpm run lint:check
cd frontend && pnpm test
```

**Step 3: Run focused manual scenarios**

至少手工验证以下场景：

1. 余额足够，用户从 Basic 升级到 Pro，直接扣余额成功。
2. 余额不足，用户从 Basic 升级到 Pro，切微信/支付宝扫码支付成功。
3. 用户试图从 Pro 升回 Basic，被明确拒绝。
4. 历史无套餐快照订阅，页面显示“暂不支持升级”。
5. 同一来源订阅重复创建升级订单，被拒绝。
6. 差价为 0 的升级直接完成，无第三方支付弹窗。

**Step 4: Review diff**

```bash
git diff --stat
git status --short
```

**Step 5: Close out**

最终总结必须明确：

- 升级规则是什么
- 差价怎么算
- 哪些历史订阅不能升级
- 为什么自动退款先收口为人工处理

这样后续执行的人就不会把升级逻辑做成“看起来能用、但账务不可追溯”的半成品。

