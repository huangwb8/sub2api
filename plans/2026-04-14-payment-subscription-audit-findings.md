# 支付与订阅购买链路审计发现

审计时间：2026-04-14 23:10:57 CST

审计范围：
- 用户充值下单
- 用户购买订阅套餐
- 支付回调验签
- 余额到账 / 订阅生效
- 支付实例负载均衡与限额

审计方式：
- 只读源码审查
- 未修改任何源代码
- 结合现有测试与文档交叉核对

## 结论

当前代码中存在会严重影响业务的支付/额度问题。

其中最危险的是多实例直连支付场景：
- 多实例 Stripe 可能直接导致前端支付无法完成
- 多实例 Stripe / 支付宝官方 / 微信官方可能导致 webhook 验签失败，用户已支付但系统不能及时到账或开通订阅

如果线上始终只有单实例支付，这两类问题不一定已经爆发；但代码缺陷客观存在。一旦启用多实例，风险很高。

## 重大问题

### 1. 多实例直连支付的 webhook 实例定位错误

严重级别：Critical

问题描述：
- webhook 处理前会尝试根据 `out_trade_no` 找到原始订单实例，然后用该实例的密钥验签
- 但当前实现只对 EasyPay 从回调体里提取了 `out_trade_no`
- 对 Stripe、支付宝官方、微信官方，`extractOutTradeNo()` 直接返回空字符串
- 一旦返回空字符串，后续会退回注册表里“任意一个同 providerKey 的实例”
- 多实例时，这很容易拿错验签凭证

业务影响：
- 用户实际已支付，但回调验签失败
- 订单无法及时从 `PENDING`/`EXPIRED` 进入 `PAID`/`COMPLETED`
- 充值不到账
- 订阅套餐不生效或延迟生效

为什么我判断它是实质性业务风险：
- Stripe 验签依赖每个实例自己的 `webhookSecret`
- 支付宝验签依赖每个实例自己的应用公私钥配置
- 微信支付验签依赖每个实例自己的商户配置与公钥
- 因此“同一种 providerKey 下任意挑一个实例来验签”在多实例下不成立

关键源码证据：
- `backend/internal/handler/payment_webhook_handler.go`
- `backend/internal/service/payment_service.go`
- `backend/internal/payment/registry.go`
- `backend/internal/payment/provider/stripe.go`
- `backend/internal/payment/provider/alipay.go`
- `backend/internal/payment/provider/wxpay.go`

关键代码点：
- `extractOutTradeNo()` 仅处理 EasyPay
- `GetWebhookProvider()` 在 `outTradeNo == ""` 时回退到 `registry.GetProviderByKey(providerKey)`
- `GetProviderByKey()` 只返回“第一个匹配 providerKey 的 provider”

### 2. 多实例 Stripe 使用了错误的 publishable key

严重级别：Critical

问题描述：
- 下单时，Stripe 订单会通过负载均衡被分配到某一个具体 Stripe 实例
- 该实例生成的 `client_secret` 只能与同一 Stripe 账号的 `publishableKey` 配对使用
- 但前端支付页拿到的 `publishableKey` 不是订单所属实例的 key
- 后端当前只返回“第一个启用的 Stripe 实例”的 `publishableKey`
- 前端随后把这个全局 key 与当前订单返回的 `client_secret` 组合起来初始化 Stripe Elements / confirm payment

业务影响：
- 用户创建了 Stripe 订单，但前端支付流程直接失败
- 表现为充值失败、购买订阅套餐失败、Stripe 支付页报错

为什么我判断它不是边缘问题：
- 这是 Stripe 的账号绑定规则，不是可兼容行为
- 多 Stripe 实例配置下，只要负载均衡把订单分配到的实例不是“第一个启用实例”，就有概率失败

关键源码证据：
- `backend/internal/service/payment_config_service.go`
- `backend/internal/handler/payment_handler.go`
- `backend/internal/service/payment_order.go`
- `backend/internal/payment/provider/stripe.go`
- `frontend/src/views/user/PaymentView.vue`
- `frontend/src/components/payment/StripePaymentInline.vue`
- `frontend/src/views/user/StripePaymentView.vue`

关键代码点：
- `getStripePublishableKey()` 通过 `Limit(1)` 取首个启用 Stripe 实例
- `GetCheckoutInfo()` 把这个全局 key 返回给前端
- 前端 `StripePaymentInline` 使用 `checkout.stripe_publishable_key`
- 但订单实际 `client_secret` 来自 `invokeProvider()` 选中的具体实例

## 重要问题

### 3. “每日充值限额”被错误应用到了“购买订阅套餐”

严重级别：Important

问题描述：
- 创建订单事务里，无论是普通充值还是购买订阅套餐，都会统一调用 `checkDailyLimit()`
- 该限制文案和文档语义都是“每日累计充值上限”
- 但当前实现把购买订阅套餐也算进去了
- 余额支付买订阅的路径也同样受这个限制影响

业务影响：
- 用户当天充值较多后，可能被错误阻止继续购买订阅套餐
- 用户当天买了多个套餐，可能收到“daily recharge limit reached”这类不符合实际语义的错误

关键源码证据：
- `backend/internal/service/payment_order.go`
- `docs/PAYMENT_CN.md`

关键代码点：
- `createOrderInTx()` 中对所有第三方支付订单调用 `checkDailyLimit()`
- `createBalanceSubscriptionOrderInTx()` 中对余额支付订阅也调用 `checkDailyLimit()`
- 文档把该设置项定义为“每日累计充值上限”

### 4. 实例限额在“全部实例都超限”时会失效

严重级别：Important

问题描述：
- 负载均衡会先根据实例的单笔最小/最大金额、每日限额过滤候选实例
- 但如果过滤后一个实例都不剩，代码不会拒单
- 它会直接回退到“完整候选集”，继续选择一个实例发起支付

业务影响：
- 已超限的实例仍然可能继续接单
- 与文档“自动跳过超出限额的实例”不一致
- 可能导致上游商户侧风控、拒单、异常失败或运营误判

关键源码证据：
- `backend/internal/payment/load_balancer.go`
- `docs/PAYMENT_CN.md`

关键代码点：
- `SelectInstance()` 在 `len(available) == 0` 时执行 `available = candidates`

### 5. `enabled_payment_types` 看起来没有真正约束用户支付方式

严重级别：Important

问题描述：
- 系统配置里有 `enabled_payment_types`
- 管理后台也会读写这项配置
- 但在用户支付入口、checkout 聚合、下单校验里，我没有找到它被真正用于拦截用户支付方式
- 当前支付可用性更像是由“启用的 provider instance”和实例支持类型决定

业务影响：
- 管理员以为关闭了某支付方式，但用户端可能仍然看到或继续使用
- 配置语义与实际行为可能不一致

说明：
- 这一条是基于全仓搜索后的结论
- 我没有看到直接消费 `EnabledTypes` 的用户侧校验链路，因此先标为 Important

关键源码证据：
- `backend/internal/service/payment_config_service.go`
- `backend/internal/handler/payment_handler.go`
- `backend/internal/service/payment_order.go`

## 风险排序

建议优先级：
1. 修复多实例 webhook 实例定位问题
2. 修复多实例 Stripe publishable key 绑定问题
3. 明确并修正“每日充值限额”是否应作用于订阅购买
4. 修正实例限额全部超限时的兜底逻辑
5. 核实并补齐 `enabled_payment_types` 的实际生效链路

## 测试覆盖观察

现状：
- `backend/internal/payment` 相关单测可运行
- `backend/internal/handler` 相关单测可运行
- `backend/internal/service` 整包单测当前存在既有 import cycle，无法直接整包验证

明显缺口：
- 没有看到覆盖“多实例 Stripe + client_secret/publishableKey 对应关系”的测试
- 没有看到覆盖“多实例直连支付 webhook 必须使用订单原始实例凭证验签”的测试
- 没有看到覆盖“全部实例超限时应拒单而不是回退继续下单”的测试

## 总判断

是，当前仓库中存在一些重大的支付/额度错误，并且足以严重影响业务，特别是：
- 用户充值
- 用户购买订阅套餐

最需要警惕的是多实例支付部署场景。只要线上用了多实例 Stripe、支付宝官方或微信官方，这些问题就非常值得按线上事故级别处理。
