# Payment Quota Critical Remediation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 修复支付、订阅购买和额度链路中可能导致扣款不到账、套餐未开通、续费丢失、重复发放或余额缓存不一致的关键问题。

**Architecture:** 以“订单状态与权益发放原子化、可重试、可幂等”为主线改造。余额充值继续保留兑换码幂等模型；订阅购买、续费和升级改为数据库事务内完成状态变更与权益变更，并用订单号/状态约束替代审计日志作为幂等来源。

**Tech Stack:** Go 1.26、Ent ORM、PostgreSQL、Redis billing cache、Gin、现有 `PaymentService` / `SubscriptionService` / repository 分层。

**Minimal Change Scope:** 允许修改 `backend/internal/service/`、`backend/internal/repository/`、`backend/internal/handler/admin/`、`backend/migrations/` 和对应测试；避免改动前端 UI、支付 provider 协议实现、无关网关转发逻辑、计划类文档以外的 docs。

**Success Criteria:** 余额支付订阅不会出现扣余额但订单无法恢复；同一订阅并发续费不会丢失有效期；第三方支付订阅回调重复/重试不会重复发放；升级失败订单不会永久阻塞新升级；余额支付套餐后缓存立即失效或扣减；外部 Admin 充值并发不会覆盖余额。

**Verification Plan:** 运行 `cd backend && go test -tags=unit ./internal/service ./internal/handler/admin`，运行 `cd backend && go test -tags=integration ./internal/repository -run 'TestUsageBillingRepositoryApply|TestUserSubscription'`，新增并运行针对支付/订阅的并发与失败恢复测试。

---

## Non-Goals

- 不重新设计支付产品形态。
- 不改支付供应商接口协议，除非测试证明当前 provider 行为阻塞修复。
- 不在本轮调整前端视觉或支付页交互。

## Risk Order

P0：
- 余额支付订阅扣款后订单卡在 `RECHARGING` 且不能重试。
- 第三方支付订阅发放成功但订单标记失败后，重试可能重复续期。
- 同一订阅并发续费丢失有效期。

P1：
- 订阅升级创建失败的 `FAILED` 未支付订单阻塞后续升级。
- 余额支付订阅/升级后余额缓存未失效，用户短时间内可能继续按旧余额使用。

P2：
- Admin 余额调整接口用于外部支付充值时，不同幂等键并发加款可能互相覆盖。

## Task 1: Add Failure-Recovery Tests for Balance-Paid Subscription

**Files:**
- Modify: `backend/internal/service/payment_balance_subscription_test.go`
- Test: `backend/internal/service/payment_balance_subscription_test.go`

**Step 1: Write failing tests**

Add tests that simulate:
- balance subscription order created and balance deducted, then fulfillment fails before completion;
- `RetryFulfillment` or equivalent recovery can complete or roll back the order;
- no second balance deduction occurs on retry.

**Step 2: Run focused tests**

Run:

```bash
cd backend
go test -tags=unit ./internal/service -run 'TestPaymentService_CreateOrder_WithBalancePayment'
```

Expected before implementation: at least one new recovery test fails because `RECHARGING` orders cannot be retried.

**Step 3: Implement recovery behavior**

Change fulfillment/retry logic so balance-paid subscription orders have a deterministic recovery path:
- either complete the subscription and mark order completed;
- or roll back deducted balance and mark failed;
- never leave a paid balance order stuck in an unrecoverable `RECHARGING` state.

**Step 4: Verify**

Run the focused test command again and confirm PASS.

## Task 2: Make Subscription Fulfillment Idempotent by Order State, Not Audit Log

**Files:**
- Modify: `backend/internal/service/payment_fulfillment.go`
- Modify: `backend/internal/service/subscription_service.go`
- Test: `backend/internal/service/payment_fulfillment_test.go`

**Step 1: Write failing test**

Create a test for this sequence:
- order is `PAID`;
- subscription extension succeeds;
- order completion/audit write fails or is interrupted;
- retry does not extend the subscription again.

**Step 2: Add durable fulfillment marker**

Prefer a database-backed marker that is part of the order/subscription mutation path. Conservative options:
- add a dedicated order fulfillment status/metadata field; or
- create a small fulfillment ledger keyed by `payment_order_id`; or
- encode a deterministic order marker in subscription notes only if combined with a unique database constraint.

Do not use `payment_audit_logs` as the sole idempotency source.

**Step 3: Execute subscription mutation and completion in one transaction**

For third-party paid subscription orders:
- lock the order row by status;
- apply subscription create/extend once;
- mark order `COMPLETED`;
- commit together.

**Step 4: Verify**

Run:

```bash
cd backend
go test -tags=unit ./internal/service -run 'TestPaymentService_ExecuteSubscriptionFulfillment|TestPaymentService_RetryFulfillment'
```

## Task 3: Make Subscription Renewal Atomic Under Concurrency

**Files:**
- Modify: `backend/internal/service/subscription_service.go`
- Modify: `backend/internal/repository/user_subscription_repo.go`
- Test: `backend/internal/repository/user_subscription_repo_integration_test.go`
- Test: `backend/internal/service/subscription_assign_idempotency_test.go`

**Step 1: Write concurrent renewal test**

Create a real database integration test:
- seed one active subscription with `expires_at = T`;
- run two concurrent `AssignOrExtendSubscription(..., ValidityDays: 30)` calls;
- assert final `expires_at` is `T + 60 days`, not `T + 30 days`.

**Step 2: Add repository method for atomic extension**

Use SQL/Ent update semantics that do not read-modify-write in memory:
- active subscription: `expires_at = LEAST(expires_at + interval, MaxExpiresAt)`;
- expired subscription: reset from `NOW() + interval`;
- update status, notes and plan snapshot in the same transaction.

**Step 3: Wire service to atomic method**

Replace the current `GetByUserIDAndGroupID` -> compute -> `Update` flow for renewals.

**Step 4: Verify**

Run:

```bash
cd backend
go test -tags=integration ./internal/repository -run 'TestUserSubscriptionRepoSuite/TestConcurrent'
go test -tags=unit ./internal/service -run 'TestSubscription'
```

## Task 4: Fix Upgrade Order Blocking and Recovery

**Files:**
- Modify: `backend/internal/service/payment_order.go`
- Modify: `backend/internal/service/payment_fulfillment.go`
- Test: `backend/internal/service/subscription_upgrade_service_test.go`

**Step 1: Write failing tests**

Cover:
- payment provider create fails after upgrade order creation;
- order becomes `FAILED` but has no `paid_at`;
- user can create a new upgrade order for the same source subscription.

**Step 2: Narrow unfinished upgrade check**

Change `ensureNoPendingUpgradeOrder` so unpaid `FAILED` orders do not block new upgrade attempts. Keep blocking for:
- `PENDING`;
- `PAID`;
- `RECHARGING`;
- paid `FAILED` orders that need fulfillment retry.

**Step 3: Preserve paid failed upgrade retry**

Ensure `RetryFulfillment` still works for `FAILED` upgrade orders with `paid_at != nil`.

**Step 4: Verify**

Run:

```bash
cd backend
go test -tags=unit ./internal/service -run 'TestSubscriptionUpgrade|TestPaymentService'
```

## Task 5: Invalidate Balance Cache After Balance-Paid Subscription and Upgrade

**Files:**
- Modify: `backend/internal/service/payment_order.go`
- Test: `backend/internal/service/payment_balance_subscription_test.go`
- Test: `backend/internal/service/billing_cache_service_test.go`

**Step 1: Write failing cache test**

Seed a user balance cache, buy a subscription with wallet balance, then assert the cache is either invalidated or reduced.

**Step 2: Add cache invalidation helper**

After committed DB balance deductions in balance-paid subscription and upgrade flows:
- invalidate auth cache by user ID if available;
- invalidate billing balance cache;
- avoid async-only behavior when immediate correctness matters.

**Step 3: Verify**

Run:

```bash
cd backend
go test -tags=unit ./internal/service -run 'TestPaymentService_CreateOrder_WithBalancePayment|TestBillingCache'
```

## Task 6: Make Admin Balance Adjustment Atomic for External Payment Recharge

**Files:**
- Modify: `backend/internal/service/admin_service.go`
- Modify: `backend/internal/repository/user_repo.go`
- Test: `backend/internal/service/admin_service_update_balance_test.go`
- Test: `backend/internal/repository/user_repo_integration_test.go`

**Step 1: Write concurrent add test**

Simulate two different idempotency keys adding balance to the same user at the same time. Final balance must equal old balance plus both additions.

**Step 2: Add atomic repository operation**

For `operation=add/subtract`, use database atomic `AddBalance(delta)` with non-negative guard for subtract. Keep `set` as explicit set operation.

**Step 3: Preserve balance history**

Record the exact delta in redeem-code history after the atomic update. Ensure the returned user reflects the post-update balance.

**Step 4: Verify**

Run:

```bash
cd backend
go test -tags=unit ./internal/service -run 'TestAdminService_UpdateUserBalance'
go test -tags=integration ./internal/repository -run 'TestUserRepoSuite/TestUpdateBalance|TestUserRepoSuite/TestConcurrent'
```

## Task 7: Full Regression Matrix

**Files:**
- Test only unless failures expose missing coverage.

**Step 1: Run backend unit tests**

```bash
cd backend
go test -tags=unit ./...
```

**Step 2: Run focused integration tests**

```bash
cd backend
go test -tags=integration ./internal/repository -run 'TestUsageBillingRepositoryApply|TestUserSubscription|TestUserRepo'
```

**Step 3: Run lint when local toolchain supports it**

```bash
cd backend
golangci-lint run ./...
```

**Step 4: Manual audit checklist**

- No paid order can remain permanently unrecoverable.
- No balance deduction happens outside a path that invalidates balance cache.
- No subscription extension depends only on audit logs for idempotency.
- No read-modify-write balance update remains in external recharge paths.
- Existing ordinary balance recharge through redeem code remains idempotent.

## Rollback Notes

- Database migration additions must be backward compatible and nullable/defaulted.
- Keep existing order statuses stable for frontend compatibility.
- If introducing a fulfillment ledger, deploy migration before application rollout.
- If a hotfix is needed before full refactor, prioritize allowing safe retry/recovery of `RECHARGING` balance-paid subscription orders and excluding unpaid `FAILED` upgrade orders from the blocker query.
