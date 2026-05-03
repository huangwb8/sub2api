# Subscription Upgrade Order Blocking Fix Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 修复用户升级现有订阅时被“该订阅已有未完成的升级订单”长期阻塞的问题，并保留已支付失败订单的人工重试安全边界。

**Architecture:** 订单阻塞判断继续集中在 `PaymentService.ensureNoPendingUpgradeOrder`，但要区分“确实已支付、需要履约重试”的失败升级订单和“余额已退回、可重新下单”的失败升级订单。余额支付升级履约失败且余额回滚成功时，应把订单显式标记为已回滚并清除 `paid_at`，让后续升级尝试不会被误判为未完成订单；历史已卡住订单只做可审计的人工修复，不做无法证明退款状态的自动放行。

**Tech Stack:** Go 1.26、Gin、Ent ORM、SQLite enttest 单元测试、现有 `PaymentService` / `SubscriptionUpgradeService` / `UserRepository` 分层。

**Minimal Change Scope:** 允许修改 `backend/internal/service/payment_order.go` 和 `backend/internal/service/subscription_upgrade_service_test.go`；如需历史数据修复，可新增 `docs/` 操作说明或 `backend/migrations/` 中只读安全注释型计划，但避免新增后台任务、接口重构、订单状态枚举变更和前端 UI 改造。

**Success Criteria:** 余额支付升级履约失败且余额回滚成功后，同一来源订阅可以再次创建升级订单；第三方支付 `PENDING`、`PAID`、`RECHARGING` 以及已支付 `FAILED` 升级订单仍会阻止重复升级；支付提供商创建失败生成的未支付 `FAILED` 升级订单不会阻止重新下单；管理员 `RetryFulfillment` 仍拒绝未支付订单并允许已支付失败订单重试。

**Verification Plan:** 运行 `cd backend && go test -tags=unit ./internal/service -run 'TestPaymentService_CreateOrder_WithBalanceSubscriptionUpgrade|TestSubscriptionUpgrade|TestPaymentService_RetryFulfillment'`；必要时补充 `cd backend && go test -tags=unit ./internal/service`。

---

## Root Cause

当前 `ensureNoPendingUpgradeOrder` 已经排除了 `FAILED` 且 `paid_at IS NULL` 的升级订单，因此“第三方支付创建失败后订单变为未支付 FAILED”不是主要根因。

真正高风险路径在余额支付升级：

- `createBalanceSubscriptionUpgradeOrderInTx` 创建订单时直接设置 `status = PAID` 和 `paid_at = now`，并扣除用户余额。
- `ExecuteSubscriptionUpgradeFulfillment` 履约失败时会把订单标记为 `FAILED`，但保留 `paid_at`。
- 调用方随后把用户余额退回，但升级路径当前没有记录 `BALANCE_PAYMENT_ROLLED_BACK`，也没有清除 `paid_at`。
- 下次升级时，`ensureNoPendingUpgradeOrder` 会把这条 `FAILED + paid_at != nil` 的订单视为“已支付失败、需要履约重试”的未完成升级订单，于是返回 `UPGRADE_ORDER_EXISTS`。

这解释了用户看到“该订阅已有未完成的升级订单，请先处理当前订单”但自己无法继续升级的现象。

## Non-Goals

- 不改变订单状态枚举。
- 不放宽第三方已支付失败升级订单的阻塞规则。
- 不自动恢复升级退款后的旧订阅，继续保持现有人工处理策略。
- 不对无法确认余额是否已回滚的历史订单做自动批量修改。

## Task 1: Add Regression Test For Rolled-Back Balance Upgrade

**Files:**
- Modify: `backend/internal/service/subscription_upgrade_service_test.go`

**Step 1: 构造失败场景**

新增测试 `TestPaymentService_CreateOrder_WithBalanceSubscriptionUpgradeRollbackAllowsNewUpgradeOrder`：

- 创建用户余额 `200`。
- 创建来源订阅 `sourceSub`，来源套餐 family 为 `openai-team`、rank 为 `10`。
- 创建第一个目标套餐 `targetPlanA`，rank 为 `20`。
- 预先创建用户在 `targetPlanA.GroupID` 下的活跃订阅，让升级履约中的 `AssignSubscription` 因目标组已有订阅而失败。
- 调用余额支付升级到 `targetPlanA`，预期返回错误。

**Step 2: 验证失败订单状态**

查询第一次订单，预期：

- `order_type = subscription_upgrade`
- `payment_type = balance`
- `status = FAILED`
- 用户余额已回到原值
- 来源订阅仍然是 `active`

修复前该订单通常还会保留 `paid_at`，并导致后续步骤失败。

**Step 3: 验证可以重新升级**

创建第二个目标套餐 `targetPlanB`，rank 为 `30`，且用户没有该目标组订阅。再次调用余额支付升级同一个 `sourceSub` 到 `targetPlanB`。

Expected before implementation: FAIL，返回 `UPGRADE_ORDER_EXISTS`。

Expected after implementation: PASS，订单进入 `COMPLETED`，来源订阅过期，目标订阅创建成功。

## Task 2: Mark Successful Balance Rollback As Not Paid

**Files:**
- Modify: `backend/internal/service/payment_order.go`

**Step 1: 添加小工具函数**

新增私有方法，专门用于余额支付已回滚后的订单标记：

```go
func (s *PaymentService) markBalancePaymentRolledBack(ctx context.Context, orderID int64, amount float64, cause error) {
    reason := psErrMsg(cause)
    s.writeAuditLog(ctx, orderID, "BALANCE_PAYMENT_ROLLED_BACK", "system", map[string]any{
        "amount": amount,
        "reason": reason,
    })
    if _, err := s.entClient.PaymentOrder.UpdateOneID(orderID).ClearPaidAt().Save(ctx); err != nil {
        s.writeAuditLog(ctx, orderID, "BALANCE_PAYMENT_ROLLBACK_MARK_FAILED", "system", map[string]any{
            "amount": amount,
            "reason": reason,
            "error":  err.Error(),
        })
    }
}
```

**Step 2: 接入升级余额回滚成功路径**

在 `createSubscriptionUpgradeOrder` 的余额支付分支中，`userRepo.UpdateBalance(ctx, req.UserID, amount)` 回滚成功后调用 `markBalancePaymentRolledBack`，再返回履约错误。

**Step 3: 保持回滚失败路径阻塞**

如果余额回滚失败，不要清除 `paid_at`，并补充审计日志 `BALANCE_PAYMENT_ROLLBACK_FAILED`。这类订单仍应被 `ensureNoPendingUpgradeOrder` 阻塞，等待管理员处理，避免用户余额与订单状态不一致时重复升级。

## Task 3: Preserve Paid Failed Retry Semantics

**Files:**
- Modify: `backend/internal/service/payment_order.go`
- Modify: `backend/internal/service/subscription_upgrade_service_test.go`

**Step 1: 保持阻塞查询的核心语义**

`ensureNoPendingUpgradeOrder` 继续阻塞：

- `PENDING`
- `PAID`
- `RECHARGING`
- `FAILED` 且 `paid_at IS NOT NULL`

余额回滚成功的新订单会因为 `paid_at` 被清除而自然不再命中阻塞条件。

**Step 2: 补充已支付失败订单仍阻塞测试**

新增或扩展测试：手工创建 `subscription_upgrade + FAILED + paid_at != nil` 订单，再尝试创建同来源升级订单，预期仍返回 `UPGRADE_ORDER_EXISTS`。

**Step 3: 补充未支付失败订单可重试测试**

手工创建 `subscription_upgrade + FAILED + paid_at IS NULL` 订单，再尝试创建同来源升级订单，预期不被该历史订单阻塞。

## Task 4: Historical Stuck Order Repair Guidance

**Files:**
- Create: `docs/订阅升级订单卡住处理指南.md` 或追加到 `docs/PAYMENT_CN.md`

**Step 1: 写明只读排查 SQL**

列出候选订单：

```sql
SELECT id, user_id, source_subscription_id, amount, pay_amount, status, payment_type, paid_at, failed_at, failed_reason
FROM payment_orders
WHERE order_type = 'subscription_upgrade'
  AND payment_type = 'balance'
  AND status = 'FAILED'
  AND paid_at IS NOT NULL
ORDER BY failed_at DESC;
```

**Step 2: 写明人工确认条件**

只有同时满足以下条件，才允许人工清除 `paid_at`：

- 确认订单没有 `UPGRADE_SUCCESS` 审计日志。
- 确认用户余额已退回，或管理员已补偿余额。
- 确认失败原因不是余额回滚失败。
- 确认来源订阅仍可升级，或管理员接受重新下单后的业务结果。

**Step 3: 写明修复 SQL 模板**

模板必须使用占位符，不写真实订单 ID：

```sql
UPDATE payment_orders
SET paid_at = NULL, updated_at = NOW()
WHERE id = :order_id
  AND order_type = 'subscription_upgrade'
  AND payment_type = 'balance'
  AND status = 'FAILED'
  AND paid_at IS NOT NULL;
```

同时建议插入一条人工审计记录：

```sql
INSERT INTO payment_audit_logs (order_id, action, detail, operator, created_at)
VALUES (:order_id_text, 'BALANCE_PAYMENT_ROLLED_BACK', '{"source":"manual_repair"}', 'admin', NOW());
```

## Review Checklist

- 确认所有新增测试先失败后通过。
- 确认没有把第三方已支付失败订单误放行。
- 确认 `RetryFulfillment` 对 `paid_at IS NULL` 的订单仍返回 `INVALID_STATUS`。
- 确认余额回滚成功才清除 `paid_at`，回滚失败仍保持阻塞。
- 确认没有修改前端文案、订单状态枚举或支付提供商逻辑。

