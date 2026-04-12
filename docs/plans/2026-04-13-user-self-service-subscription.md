# User Self-Service Subscription Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make ordinary users able to purchase and renew subscription plans through the built-in payment flow, with automatic fulfillment, strong server-side validation, and regression coverage.

**Architecture:** Keep the existing built-in payment order pipeline as the single fulfillment path instead of adding a parallel subscription-purchase endpoint. Harden the payment catalog and plan management so only valid subscription products can be sold, then verify the existing order -> webhook -> fulfillment chain with focused backend tests and align the docs/UI wording to the now-supported built-in workflow.

**Tech Stack:** Go, Gin, Ent, Vue 3, Pinia, pnpm, Go test

---

### Task 1: Lock Down Sellable Subscription Plans

**Files:**
- Modify: `backend/internal/service/payment_config_plans.go`
- Test: `backend/internal/service/payment_config_plans_test.go`

**Step 1: Write the failing test**

Add tests that prove:
- `ListPlansForSale` excludes plans whose groups are missing, inactive, or not `subscription` type.
- plan creation/update rejects non-subscription or inactive groups.

**Step 2: Run test to verify it fails**

Run: `cd backend && go test -tags=unit ./internal/service -run 'TestPaymentConfigService_(ListPlansForSale|CreatePlan|UpdatePlan)'`

Expected: FAIL because the current implementation returns all `for_sale` plans and does not validate target groups.

**Step 3: Write minimal implementation**

Implement server-side plan eligibility checks in `PaymentConfigService`:
- validate group existence
- require active status
- require `subscription` billing type
- filter `ListPlansForSale` using the same rule

**Step 4: Run test to verify it passes**

Run: `cd backend && go test -tags=unit ./internal/service -run 'TestPaymentConfigService_(ListPlansForSale|CreatePlan|UpdatePlan)'`

Expected: PASS

**Step 5: Commit**

```bash
git add backend/internal/service/payment_config_plans.go backend/internal/service/payment_config_plans_test.go
git commit -m "feat: harden sellable subscription plan validation"
```

### Task 2: Prove Checkout Only Exposes Real Subscription Products

**Files:**
- Modify: `backend/internal/handler/payment_handler.go`
- Test: `backend/internal/handler/payment_handler_test.go`

**Step 1: Write the failing test**

Add a handler-level test for `GetCheckoutInfo` that proves checkout payload only contains eligible plans and keeps group metadata consistent for user purchase UI.

**Step 2: Run test to verify it fails**

Run: `cd backend && go test -tags=unit ./internal/handler -run TestPaymentHandler_GetCheckoutInfo`

Expected: FAIL because checkout currently trusts raw `ListPlansForSale` output and does not explicitly guard invalid plan/group combinations.

**Step 3: Write minimal implementation**

Reuse the hardened plan-query behavior so checkout gets only valid subscription products, and preserve the existing response shape for the frontend.

**Step 4: Run test to verify it passes**

Run: `cd backend && go test -tags=unit ./internal/handler -run TestPaymentHandler_GetCheckoutInfo`

Expected: PASS

**Step 5: Commit**

```bash
git add backend/internal/handler/payment_handler.go backend/internal/handler/payment_handler_test.go
git commit -m "test: cover checkout subscription catalog exposure"
```

### Task 3: Verify Subscription Purchase Fulfillment End-to-End

**Files:**
- Modify: `backend/internal/service/payment_fulfillment_test.go`
- Modify: `backend/internal/service/payment_order_test.go`

**Step 1: Write the failing test**

Add focused tests that prove:
- a paid subscription order calls subscription fulfillment once
- retry/idempotent paths do not double-extend the same subscription
- invalid subscription orders are rejected before payment creation

**Step 2: Run test to verify it fails**

Run: `cd backend && go test -tags=unit ./internal/service -run 'TestPaymentService_(CreateOrderSubscription|ExecuteSubscriptionFulfillment)'`

Expected: FAIL because these behaviors are not fully covered today.

**Step 3: Write minimal implementation**

Only patch production code if tests expose real logic gaps while keeping the current payment order lifecycle intact.

**Step 4: Run test to verify it passes**

Run: `cd backend && go test -tags=unit ./internal/service -run 'TestPaymentService_(CreateOrderSubscription|ExecuteSubscriptionFulfillment)'`

Expected: PASS

**Step 5: Commit**

```bash
git add backend/internal/service/payment_fulfillment_test.go backend/internal/service/payment_order_test.go
git commit -m "test: cover self-service subscription purchase fulfillment"
```

### Task 4: Align Frontend Messaging With Built-In Subscription Flow

**Files:**
- Modify: `frontend/src/views/user/PaymentView.vue`
- Modify: `frontend/src/components/payment/SubscriptionPlanCard.vue`
- Modify: `frontend/src/i18n/locales/zh.ts`
- Modify: `frontend/src/i18n/locales/en.ts`

**Step 1: Write the failing test**

If there is existing frontend coverage for payment UI, add assertions around subscription-specific copy/state. If there is no meaningful test harness here, skip adding new frontend tests and verify via typecheck/build instead.

**Step 2: Run test to verify it fails**

Run the smallest relevant frontend check first.

**Step 3: Write minimal implementation**

Keep the current purchase page and state flow, but make the subscription path read clearly as a built-in self-service purchase/renewal flow and ensure success refresh behavior remains intact.

**Step 4: Run verification**

Run:
- `cd frontend && pnpm run typecheck`
- `cd frontend && pnpm run lint:check`

Expected: PASS

**Step 5: Commit**

```bash
git add frontend/src/views/user/PaymentView.vue frontend/src/components/payment/SubscriptionPlanCard.vue frontend/src/i18n/locales/zh.ts frontend/src/i18n/locales/en.ts
git commit -m "feat: polish built-in self-service subscription purchase UI"
```

### Task 5: Update Docs And Changelog

**Files:**
- Modify: `docs/PAYMENT.md`
- Modify: `docs/PAYMENT_CN.md`
- Modify: `CHANGELOG.md`

**Step 1: Write the failing check**

Identify and replace outdated documentation that still says subscription plans are “planned” or unsupported.

**Step 2: Run verification**

Search:

```bash
rg -n "planned|暂不支持|Subscription Plans" docs/PAYMENT.md docs/PAYMENT_CN.md CHANGELOG.md
```

Expected: outdated wording found before edits.

**Step 3: Write minimal implementation**

Document the built-in self-service subscription capability and add an `Unreleased` changelog entry.

**Step 4: Run verification**

Run the same `rg` command and confirm the unsupported wording is removed or updated.

**Step 5: Commit**

```bash
git add docs/PAYMENT.md docs/PAYMENT_CN.md CHANGELOG.md
git commit -m "docs: document built-in self-service subscriptions"
```

### Task 6: Final Verification

**Files:**
- Verify only

**Step 1: Run backend checks**

```bash
cd backend && go test -tags=unit ./internal/service ./internal/handler
```

**Step 2: Run frontend checks**

```bash
cd frontend && pnpm run typecheck
cd frontend && pnpm run lint:check
```

**Step 3: Run focused regression checks**

```bash
cd backend && go test -tags=unit ./... 
```

**Step 4: Review diff**

```bash
git diff --stat
git status --short
```

**Step 5: Close out**

Summarize:
- what changed
- how self-service subscription now works
- what was verified
- any residual risks
