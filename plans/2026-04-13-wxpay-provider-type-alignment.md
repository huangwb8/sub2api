# Payment Provider Type Alignment Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 修复官方直连支付服务商已配置但用户下单时仍报 `payment method (...) is not configured` 的命名不一致问题，至少覆盖微信和支付宝，并确认 Stripe 链路不存在同类缺陷。

**Architecture:** 当前问题的根因是“用户侧支付方式键名”和“后端 provider registry 注册键名”不一致。前端和后台配置体系把官方直连支付分别暴露为 `wxpay` / `alipay`，但后端 `Wxpay` / `Alipay` provider 在 registry 中注册的是 `wxpay_direct` / `alipay_direct`，导致下单时 `registry.GetProviderKey("wxpay")` 或 `registry.GetProviderKey("alipay")` 取不到 provider。修复方案应优先统一到现有 UI/配置语义，也就是让官方直连 provider 在后端分别注册为 `wxpay` / `alipay`，并补充测试覆盖；Stripe 维持现有 “用户选择 `stripe`，实例子方式再映射到 `card/alipay/wechat_pay/link`” 的模型。

**Tech Stack:** Go, Ent, Gin, Vue 3, TypeScript, Vitest, Go test

---

## 背景与根因

- 报错抛出位置：
  - `backend/internal/service/payment_order.go`
  - `invokeProvider()` 调用 `s.registry.GetProviderKey(req.PaymentType)`，拿不到时直接返回 `payment method (%s) is not configured`
- 用户侧下单传参来源：
  - `frontend/src/views/user/PaymentView.vue`
  - 选中的 `selectedMethod` 原样作为 `payment_type` 发送
- 微信官方 provider 的前端配置语义：
  - `frontend/src/components/payment/providerConfig.ts`
  - `wxpay` provider 当前暴露 `['wxpay']`
- 支付宝官方 provider 的前端配置语义：
  - `frontend/src/components/payment/providerConfig.ts`
  - `alipay` provider 当前暴露 `['alipay']`
- 后端 provider 实际注册语义：
  - `backend/internal/payment/provider/wxpay.go`
  - `ProviderKey() == wxpay`
  - `SupportedTypes() == []{"wxpay_direct"}`
  - `backend/internal/payment/provider/alipay.go`
  - `ProviderKey() == alipay`
  - `SupportedTypes() == []{"alipay_direct"}`
- Stripe 当前语义：
  - `frontend/src/components/payment/providerConfig.ts` 中 provider 暴露 `['card','alipay','wxpay','link']` 仅用于实例子方式配置
  - 用户页可见方法由 `checkout-info.methods` 提供，Stripe 在后端被聚合为单一用户侧方法 `stripe`
  - `backend/internal/payment/provider/stripe.go` 的 `SupportedTypes() == []{"stripe"}`
- 结论：
  - 当前不是部署环境变量缺失导致。
  - 微信和支付宝都存在“同一个支付能力在不同层使用了两套名字”的问题。
  - Stripe 目前未见同类 registry 映射问题，但仍需做一次回归审计，确认不会把 `alipay` / `wxpay` 误当作顶层 provider 键使用。

## 修复原则

- 优先保持现有管理后台和用户前端语义不变，避免数据库已有配置、后台表单、用户已启用支付方式都被迫迁移。
- 让微信官方直连在后端 registry 中按 `wxpay` 注册。
- 只做最小闭环修复，不同时引入“重新定义 wxpay / wxpay_direct 双语义”的大改动。
- 增补测试，覆盖“registry 注册”“下单路由”“可用方式展示”三层。

## 开发前检查

### Task 1: 固化当前行为证据

**Files:**
- Inspect: `backend/internal/service/payment_order.go`
- Inspect: `backend/internal/payment/provider/wxpay.go`
- Inspect: `backend/internal/payment/registry.go`
- Inspect: `frontend/src/components/payment/providerConfig.ts`
- Inspect: `frontend/src/views/user/PaymentView.vue`

**Step 1: 记录当前根因**

- 确认 `payment_order.go` 中的报错条件就是 `registry.GetProviderKey(req.PaymentType) == ""`
- 确认前端提交的支付方式值是 `wxpay`
- 确认 `wxpay.go` 的 `SupportedTypes()` 返回 `wxpay_direct`

**Step 2: 保存复现证据**

建议在计划执行时记录一份简要证据到提交说明或 PR 描述：

- 用户已配置 `provider_key=wxpay`
- 前端展示微信支付按钮
- 点击下单时后端仍返回 `payment method (wxpay) is not configured`

**Step 3: 不修改数据库结构**

- 本问题不需要 migration
- 不新增 payment type 常量
- 不改历史订单表字段

**Step 4: Commit**

此任务只做确认，不提交。

### Task 2: 写失败测试，锁定官方直连 provider 的 registry 键名不一致

**Files:**
- Modify: `backend/internal/payment/provider/wxpay_test.go`
- Modify: `backend/internal/payment/provider/alipay_test.go`
- Inspect: `backend/internal/payment/registry_test.go`

**Step 1: 写微信失败测试**

在 `backend/internal/payment/provider/wxpay_test.go` 新增一个测试，断言：

```go
func TestWxpaySupportedTypes_ShouldRegisterWxpay(t *testing.T) {
    p, err := NewWxpay("test-instance", map[string]string{
        "appId": "wx123",
        "mchId": "1900000000",
        "privateKey": "-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----",
        "apiV3Key": "12345678901234567890123456789012",
        "publicKey": "-----BEGIN PUBLIC KEY-----\n...\n-----END PUBLIC KEY-----",
        "publicKeyId": "pub-key-id",
        "certSerial": "SERIAL123",
    })
    if err != nil {
        t.Fatalf("NewWxpay() error = %v", err)
    }
    got := p.SupportedTypes()
    if len(got) != 1 || got[0] != payment.TypeWxpay {
        t.Fatalf("SupportedTypes() = %v, want [wxpay]", got)
    }
}
```

说明：
- 这里不需要真实可用密钥，只要能通过构造函数的必填校验即可。
- 如果现有测试工具里已经有更简洁的 helper，优先复用。

**Step 2: 写支付宝失败测试**

在 `backend/internal/payment/provider/alipay_test.go` 新增一个测试，断言：

```go
func TestAlipaySupportedTypes_ShouldRegisterAlipay(t *testing.T) {
    p, err := NewAlipay("test-instance", map[string]string{
        "appId": "2026000000000000",
        "privateKey": "dummy-private-key",
        "publicKey": "dummy-public-key",
    })
    if err != nil {
        t.Fatalf("NewAlipay() error = %v", err)
    }
    got := p.SupportedTypes()
    if len(got) != 1 || got[0] != payment.TypeAlipay {
        t.Fatalf("SupportedTypes() = %v, want [alipay]", got)
    }
}
```

**Step 3: 跑单测确认失败**

Run:

```bash
cd /Volumes/2T01/Github/sub2api/backend
go test ./internal/payment/provider -run 'TestWxpaySupportedTypes_ShouldRegisterWxpay|TestAlipaySupportedTypes_ShouldRegisterAlipay' -v
```

Expected:

- FAIL
- 微信实际返回 `wxpay_direct`
- 支付宝实际返回 `alipay_direct`

**Step 4: 可选补一个 registry 层测试**

如果测试结构允许，在 `backend/internal/payment/registry_test.go` 新增一个最小测试：

- 注册一个 `Wxpay` provider，断言 `GetProviderKey(payment.TypeWxpay)` 能拿到 `"wxpay"`
- 注册一个 `Alipay` provider，断言 `GetProviderKey(payment.TypeAlipay)` 能拿到 `"alipay"`

**Step 5: Commit**

```bash
git add backend/internal/payment/provider/wxpay_test.go backend/internal/payment/registry_test.go
git add backend/internal/payment/provider/alipay_test.go
git commit -m "test: cover direct payment provider registry type mapping"
```

### Task 3: 实施最小修复

**Files:**
- Modify: `backend/internal/payment/provider/wxpay.go`
- Modify: `backend/internal/payment/provider/alipay.go`

**Step 1: 修改微信 provider 注册类型**

把：

```go
func (w *Wxpay) SupportedTypes() []payment.PaymentType {
    return []payment.PaymentType{payment.TypeWxpayDirect}
}
```

改成：

```go
func (w *Wxpay) SupportedTypes() []payment.PaymentType {
    return []payment.PaymentType{payment.TypeWxpay}
}
```

**Step 2: 保持其他语义不变**

- 不修改 `ProviderKey()`
- 不改 `CreatePayment()` 的 H5 / Native 分流逻辑
- 不改 webhook 路径
- 不新增新的支付方式常量

**Step 3: 修改支付宝 provider 注册类型**

把：

```go
func (a *Alipay) SupportedTypes() []payment.PaymentType {
    return []payment.PaymentType{payment.TypeAlipayDirect}
}
```

改成：

```go
func (a *Alipay) SupportedTypes() []payment.PaymentType {
    return []payment.PaymentType{payment.TypeAlipay}
}
```

**Step 4: 运行针对性测试**

Run:

```bash
cd /Volumes/2T01/Github/sub2api/backend
go test ./internal/payment/provider ./internal/payment ./internal/service -run 'Wxpay|Alipay|Registry|PaymentConfig|LoadBalancer' -v
```

Expected:

- 新增测试 PASS
- 现有 payment 相关测试不回归

**Step 5: Commit**

```bash
git add backend/internal/payment/provider/wxpay.go backend/internal/payment/provider/alipay.go
git commit -m "fix: align direct payment provider registry types with checkout methods"
```

### Task 4: 校验用户侧展示与后端返回是否仍一致

**Files:**
- Inspect: `frontend/src/components/payment/providerConfig.ts`
- Inspect: `frontend/src/views/user/PaymentView.vue`
- Inspect: `backend/internal/service/payment_config_limits.go`
- Inspect: `backend/internal/handler/payment_handler.go`

**Step 1: 确认不需要改前端枚举**

- `providerConfig.ts` 继续保留 `wxpay: ['wxpay']`
- `providerConfig.ts` 继续保留 `alipay: ['alipay']`
- `PaymentView.vue` 继续发送 `payment_type=wxpay`
- `PaymentView.vue` 继续发送 `payment_type=alipay`

**Step 2: 校验 checkout-info / limits 语义**

- `GetAvailableMethodLimits()` 仍会把启用的微信官方实例聚合到 `methods["wxpay"]`
- `GetAvailableMethodLimits()` 仍会把启用的支付宝官方实例聚合到 `methods["alipay"]`
- 用户页按钮、限额、下单值三者保持一致

**Step 3: 如发现 `wxpay_direct` 残留展示，做最小清理**

仅在确实出现用户可见混乱时才改：

- 删除不再使用的 `wxpay_direct` 用户侧展示项
- 删除不再使用的 `alipay_direct` 用户侧展示项
- 或在注释中说明其保留仅为兼容历史逻辑

**Step 4: Commit**

如果无需改动，则不提交。

### Task 5: 处理次要配置校验不一致风险

**Files:**
- Inspect: `frontend/src/components/payment/providerConfig.ts`
- Inspect: `backend/internal/payment/provider/wxpay.go`

**Step 1: 核对可选字段与后端必填字段是否冲突**

当前存在不一致：

- 前端把 `publicKeyId`、`certSerial` 标成 optional
- 后端 `NewWxpay()` 却把它们列入 required

**Step 2: 做决策**

二选一，优先 A：

1. A 方案：前端改为必填，和后端保持一致
2. B 方案：后端放宽其中一个或两个字段的必填校验

推荐 A 的原因：

- 当前实现里 `publicKeyId` 和 `certSerial` 都实际参与客户端初始化
- 把它们继续显示成“可选”会制造新的“已配置但实际不可用”问题

**Step 3: 如果执行 A 方案，补前端校验**

- 更新 `providerConfig.ts`
- 若有表单校验组件，也同步更新提示文字

**Step 4: 如果执行 B 方案，先确认微信 SDK 初始化是否允许缺失**

- 没有充分证据前不要贸然放宽后端校验

**Step 5: Commit**

```bash
git add frontend/src/components/payment/providerConfig.ts backend/internal/payment/provider/wxpay.go
git commit -m "fix: align wxpay required fields between admin form and backend"
```

### Task 6: 加一条服务层回归测试

**Files:**
- Modify: `backend/internal/service/payment_order_test.go` 或现有最接近的 payment service 测试文件
- Inspect: `backend/internal/testutil/`

**Step 1: 写微信失败测试**

目标：

- 构造一个启用的 `provider_key=wxpay` 实例
- 确保 `CreateOrder` 使用 `payment_type=wxpay` 时不会再因为 registry 缺 provider 而失败

建议测试断言：

- 不再返回 `payment method (wxpay) is not configured`
- 如果 mock provider/配置不完整，可以在更后面的 provider 调用阶段失败，但必须跨过 registry 映射这一关

**Step 2: 写支付宝失败测试**

目标：

- 构造一个启用的 `provider_key=alipay` 实例
- 确保 `CreateOrder` 使用 `payment_type=alipay` 时不会再因为 registry 缺 provider 而失败

**Step 3: 跑测试确认在修复前失败、修复后通过**

Run:

```bash
cd /Volumes/2T01/Github/sub2api/backend
go test ./internal/service -run 'TestCreateOrder_WxpayMappedProvider|TestCreateOrder_AlipayMappedProvider' -v
```

Expected:

- 修复前 FAIL
- 修复后 PASS

**Step 4: Commit**

```bash
git add backend/internal/service/payment_order_test.go
git commit -m "test: prevent direct payment checkout registry regressions"
```

### Task 7: 审计 Stripe 是否存在同类命名风险

**Files:**
- Inspect: `backend/internal/payment/provider/stripe.go`
- Inspect: `backend/internal/service/payment_config_limits.go`
- Inspect: `frontend/src/components/payment/providerConfig.ts`
- Inspect: `frontend/src/views/user/PaymentView.vue`

**Step 1: 确认 Stripe 顶层用户方法**

- `SupportedTypes()` 返回 `stripe`
- `pcGroupByPaymentType()` 将 Stripe 所有子方式聚合到 `stripe`
- 用户页下单顶层方法应为 `stripe`，而不是 `alipay` / `wxpay`

**Step 2: 确认 Stripe 子方式只在 provider 内部解析**

- `resolveStripeMethodTypes(req.InstanceSubMethods)` 负责把实例支持的子方式翻译成 Stripe `payment_method_types`
- 这一步不依赖 registry 顶层键名

**Step 3: 补一条审计型测试**

如果已有合适位置，新增断言：

- Stripe provider 注册后 `GetProviderKey(payment.TypeStripe) == "stripe"`
- 不要求 `GetProviderKey(payment.TypeAlipay)` 或 `GetProviderKey(payment.TypeWxpay)` 指向 Stripe

**Step 4: 记录结论**

在提交说明或计划执行记录里明确：

- Stripe 当前未见与官方直连同类的 registry 键名错配问题
- 但若未来产品要把 Stripe 的 `alipay` / `wechat_pay` 暴露成用户独立按钮，则必须重新设计顶层 payment type 语义

**Step 5: Commit**

如无代码改动，则不提交。

### Task 8: 本地联调验证

**Files:**
- Inspect: `deploy/` 相关启动文件
- Inspect: 本地管理员支付配置

**Step 1: 启动本地环境**

Run:

```bash
cd /Volumes/2T01/Github/sub2api
docker compose up -d
```

或使用项目既有启动方式。

**Step 2: 在后台确认配置**

- 支付系统已启用
- `payment_enabled_types` 包含 `wxpay`
- `payment_enabled_types` 包含 `alipay`
- 存在启用中的 `provider_key=wxpay` 服务商实例
- 存在启用中的 `provider_key=alipay` 服务商实例
- 服务商 `supported_types` 包含 `wxpay`
- 服务商 `supported_types` 包含 `alipay`

**Step 3: 从用户页发起一笔微信支付订单**

验证点：

- 不再出现 `payment method (wxpay) is not configured`
- PC 端获得二维码或移动端跳转 H5
- 订单表中 `payment_type=wxpay`
- 订单表中 `provider_instance_id` 被成功写入

**Step 4: 从用户页发起一笔支付宝订单**

验证点：

- 不再出现 `payment method (alipay) is not configured`
- 返回支付跳转链接或二维码
- 订单表中 `payment_type=alipay`
- 订单表中 `provider_instance_id` 被成功写入

**Step 5: 可选验证 Stripe**

- 顶层方法显示为 `stripe`
- 进入 Stripe 支付弹窗后可看到其内部子方式
- 不出现把 Stripe 子方式误当顶层 provider 的报错

**Step 6: 记录验证结果**

建议记录：

- 使用的浏览器/终端
- 管理后台服务商配置截图或关键字段
- 实际订单返回结果

**Step 7: Commit**

联调验证不必单独提交。

### Task 9: 文档与兼容性说明

**Files:**
- Modify: `docs/PAYMENT_CN.md`
- Modify: `docs/PAYMENT.md`
- Optional: `CHANGELOG.md`

**Step 1: 更新文档措辞**

明确说明：

- 微信官方直连在用户侧支付方式标识为 `wxpay`
- 支付宝官方直连在用户侧支付方式标识为 `alipay`
- 管理后台服务商类型也是 `wxpay`
- 管理后台服务商类型也是 `alipay`
- `wxpay_direct` / `alipay_direct` 不应再作为用户侧配置入口暴露

**Step 2: 在变更日志记录修复**

建议加入一条简短说明：

- 修复微信官方支付实例已配置但用户下单仍提示未配置的问题
- 修复支付宝官方支付实例已配置但用户下单仍提示未配置的问题

**Step 3: Commit**

```bash
git add docs/PAYMENT_CN.md docs/PAYMENT.md CHANGELOG.md
git commit -m "docs: clarify wxpay direct checkout mapping"
```

## 回归检查清单

- [ ] `payment_type=wxpay` 下单不再命中 “is not configured”
- [ ] `payment_type=alipay` 下单不再命中 “is not configured”
- [ ] EasyPay 的 `wxpay` 不受影响
- [ ] EasyPay 的 `alipay` 不受影响
- [ ] Stripe 的 `wechat_pay` / `wxpay` 聚合逻辑不受影响
- [ ] Stripe 的 `alipay` 聚合逻辑不受影响
- [ ] `/api/v1/payment/checkout-info` 返回的 `methods` 仍包含 `wxpay`
- [ ] `/api/v1/payment/checkout-info` 返回的 `methods` 仍包含 `alipay`
- [ ] `/api/v1/payment/checkout-info` 对 Stripe 仍只返回单一顶层 `stripe`
- [ ] webhook 路径 `/api/v1/payment/webhook/wxpay` 不变
- [ ] webhook 路径 `/api/v1/payment/webhook/alipay` 不变
- [ ] 管理后台编辑微信官方服务商时，不会再出现“表单显示可选但后端实际必填”的误导
- [ ] 管理后台编辑支付宝官方服务商时，字段必填语义和后端保持一致

## 风险说明

- 如果历史数据里真的存在依赖 `wxpay_direct` / `alipay_direct` 的前端或第三方调用，改 registry 映射后需要额外确认是否还保留兼容路径。
- 如果 `publicKeyId` / `certSerial` 的必填策略不统一，即便修复了本次 registry 问题，仍可能出现“服务商看起来已启用，但 provider 初始化失败”的第二类问题。
- Stripe 当前没有同类 registry 问题，但它内部的 `alipay` / `wechat_pay` 是子方式，不应和官方直连的顶层 `alipay` / `wxpay` 混为一谈；后续若产品要拆成独立按钮，会涉及更大范围的语义重构。
- `PaymentService` 采用 lazy provider loading，若管理后台更新服务商配置后未触发 `RefreshProviders()`，运行时还可能遇到旧缓存问题；执行时应顺手确认配置更新路径是否会刷新 registry。

## 推荐实施顺序

1. 先做 Task 2 和 Task 3，快速修复主 bug。
2. 再做 Task 6，防止微信和支付宝回归。
3. 然后处理 Task 5，解决直连配置字段不一致。
4. 做 Task 7，确认 Stripe 没有同类顶层命名问题。
5. 最后做 Task 8 和 Task 9，完成联调与文档闭环。
