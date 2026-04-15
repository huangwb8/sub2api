# 货币语义统一优化计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 统一系统内“支付金额/用户余额”和“上游 API 成本/额度限制”的货币语义，避免人民币金额被显示为美元，降低运营和用户理解风险。

**Architecture:** 当前系统实际存在两套货币语义。支付链路明确按 `CNY` 处理，而额度、限额、API 成本链路明确按 `USD` 处理；但前端多个页面把这两套金额混用为统一的 `$` 展示，造成语义漂移。改造应坚持“语义分层而不是全量替换”：充值、支付、订单、余额统一到 `CNY/¥`，API usage、quota、rate limit、订阅组 `*_usd` 字段继续保留 `USD/$`，并在展示层建立统一格式化入口，禁止继续散落硬编码货币符号。

**Tech Stack:** Vue 3, TypeScript, Go, Gin, Ent, Vitest, Go test

---

## 先回答：有没有必要改？

有必要改，而且建议改，但不建议“一刀切把所有 `$` 改成 `¥`”。

原因：

- 当前支付链路已经明确按人民币工作，代码证据包括：
  - `backend/internal/payment/types.go`
  - `backend/internal/payment/amount.go`
  - `backend/internal/payment/provider/wxpay.go`
  - `backend/internal/service/payment_order.go`
- 当前 API 成本/额度链路又明确按美元建模，代码证据包括：
  - `backend/ent/schema/api_key.go`
  - `frontend/src/i18n/locales/zh.ts`
  - `frontend/src/views/user/KeysView.vue`
  - `frontend/src/views/user/SubscriptionsView.vue`
- 前端显示层存在明显不一致：
  - 同一系统里，充值页和余额页大量硬编码 `$`
  - 支付结果页又显示 `¥`
  - 工具函数 `frontend/src/utils/format.ts` 默认货币为 `USD`

如果不改，会持续带来这些问题：

- 用户把人民币余额误解成美元余额
- 管理员在后台调余额时误判金额单位
- 后续开发者继续复制 `$` 硬编码，扩大不一致范围
- 支付金额、套餐价格、余额、额度、成本之间的业务语义越来越难维护

## 目标边界

这次优化不追求“系统只保留一种货币”，而是追求“每种金额只表达一种货币语义”。

### 应统一为 CNY/人民币 的对象

- 用户余额 `balance`
- 充值金额 `amount` / `pay_amount` 在支付与订单语境下的展示
- 支付页金额、手续费、实付金额
- 支付成功页、订单页中的支付金额
- 管理后台的手动加减余额
- 兑换码里“余额类”金额
- 订阅套餐价格 `plan.price` / `original_price`，前提是当前运营口径确实按人民币售卖

### 应继续保留 USD/美元 的对象

- API Key `quota`
- `quota_used`
- `rate_limit_5h` / `rate_limit_1d` / `rate_limit_7d`
- `usage_5h` / `usage_1d` / `usage_7d`
- Group / Subscription 的 `daily_limit_usd` / `weekly_limit_usd` / `monthly_limit_usd`
- usage 页面里的 `today_cost` / `actual_cost` / `total_cost`
- 所有直接对应上游模型计费的数据

### 本次不做的事

- 不引入汇率换算
- 不把数据库里的 `*_usd` 字段改名
- 不修改历史订单金额
- 不尝试让一个字段同时支持“按配置切换 USD/CNY”
- 不做多币种完整国际化系统

## 核心设计原则

### 原则 1：业务语义优先于展示样式

- 先定义金额属于哪条业务链路，再决定用什么符号显示
- 禁止凭页面感觉直接写 `$` 或 `¥`

### 原则 2：统一入口，禁止散落硬编码

- 前端所有货币展示必须经过公共格式化函数
- 模板中直接写 `$` / `¥` 应视为待清理项，除非是纯文案或 SVG 图标

### 原则 3：保持向后兼容

- 后端现有 `*_usd` API 字段先不改
- 优先改“显示层”和“前端命名解释”，避免破坏前后端协议

### 原则 4：先收口再扩展

- 第一阶段先统一用户可见的重要页面
- 第二阶段再清理长尾页面和后台表单
- 最后补文档和测试，避免回归

## 推荐实施方案

### Task 1: 建立货币语义清单

**Files:**
- Inspect: `frontend/src/utils/format.ts`
- Inspect: `frontend/src/views/user/PaymentView.vue`
- Inspect: `frontend/src/views/user/PaymentResultView.vue`
- Inspect: `frontend/src/views/user/ProfileView.vue`
- Inspect: `frontend/src/components/admin/user/UserBalanceModal.vue`
- Inspect: `frontend/src/views/user/KeysView.vue`
- Inspect: `frontend/src/views/user/SubscriptionsView.vue`
- Inspect: `frontend/src/views/user/RedeemView.vue`
- Inspect: `frontend/src/views/admin/GroupsView.vue`
- Inspect: `frontend/src/views/admin/orders/AdminPaymentPlansView.vue`

**Step 1: 全仓扫描硬编码符号**

Run:

```bash
cd /Volumes/2T01/Github/sub2api
rg -n '\$\{\{|\\$|&#165;|¥|￥' frontend/src
```

Expected:

- 找到所有直接输出货币符号的位置
- 将每个位置归类为 `CNY` / `USD` / `待确认`

**Step 2: 形成金额语义台账**

建议在执行时产出一份临时表，至少包含：

- 文件路径
- 字段名
- 当前符号
- 实际语义
- 是否用户可见
- 优先级

**Step 3: 优先处理用户最容易误解的页面**

优先级建议：

1. 用户余额页
2. 充值页
3. 支付结果页
4. 用户订单页
5. 管理后台余额操作
6. 兑换码余额记录
7. 订阅套餐页
8. 长尾后台展示页

### Task 2: 设计统一的前端货币格式化 API

**Files:**
- Modify: `frontend/src/utils/format.ts`
- Inspect: `frontend/src/i18n/index.ts`

**Step 1: 新增明确语义的格式化函数**

不要继续只保留一个默认 `USD` 的 `formatCurrency(amount, currency='USD')` 调用方式。

建议至少拆成：

```ts
export function formatCNY(amount: number | null | undefined): string
export function formatUSD(amount: number | null | undefined): string
export function formatMoney(
  amount: number | null | undefined,
  currency: 'CNY' | 'USD'
): string
```

**Step 2: 明确 locale 策略**

- 中文界面下 `CNY` 可显示为 `¥` 或 `￥`
- 英文界面下 `CNY` 仍建议保留 `CNY` 或 `¥`
- `USD` 继续使用 `$`

推荐做法：

- 默认走 `Intl.NumberFormat`
- 对 `CNY` 在中文 locale 下验证输出是否符合预期
- 对极小值继续保留现有小数位逻辑

**Step 3: 提供语义别名，减少误用**

例如：

```ts
export const formatBalanceAmount = formatCNY
export const formatUsageCost = formatUSD
```

如果项目里能接受更显式的命名，这一步值得做，因为它能降低后续误用概率。

### Task 3: 第一阶段只修正“余额/充值/支付”链路

**Files:**
- Modify: `frontend/src/views/user/ProfileView.vue`
- Modify: `frontend/src/components/user/dashboard/UserDashboardStats.vue`
- Modify: `frontend/src/views/user/PaymentView.vue`
- Modify: `frontend/src/views/user/PaymentResultView.vue`
- Modify: `frontend/src/components/payment/AmountInput.vue`
- Modify: `frontend/src/components/admin/user/UserBalanceModal.vue`
- Modify: `frontend/src/components/admin/user/UserBalanceHistoryModal.vue`
- Modify: `frontend/src/views/user/RedeemView.vue`
- Modify: `frontend/src/views/user/UserOrdersView.vue`

**Step 1: 用户余额统一改为 CNY**

- `ProfileView`
- dashboard 的 balance 卡片
- 充值页顶部 current balance
- 管理员余额弹窗 current balance / new balance

**Step 2: 充值金额和实付金额统一改为 CNY**

- `PaymentView` 中的金额、手续费、实付金额
- `AmountInput` 输入框前缀
- 下单按钮金额
- 订阅套餐价格，仅在确认其运营价格确实为人民币后同步改为 CNY

**Step 3: 支付结果和订单支付金额统一改为 CNY**

- `PaymentResultView`
- 用户订单页中的充值订单金额
- 管理后台订单详情中的支付金额

**Step 4: 兑换码中 balance 类型金额统一改为 CNY**

- `RedeemView`
- 余额变化记录里的 balance / admin_balance

### Task 4: 第二阶段保持 usage/quota/limit 继续显示 USD

**Files:**
- Modify: `frontend/src/views/user/KeysView.vue`
- Modify: `frontend/src/views/user/SubscriptionsView.vue`
- Modify: `frontend/src/views/KeyUsageView.vue`
- Modify: `frontend/src/views/admin/GroupsView.vue`
- Modify: `frontend/src/views/admin/orders/AdminPaymentPlansView.vue`
- Modify: `frontend/src/i18n/locales/zh.ts`
- Modify: `frontend/src/i18n/locales/en.ts`

**Step 1: 把这些页面显式接入 `formatUSD()`**

- Key quota
- rate limit
- subscription group 限额
- usage cost
- admin group 限额

**Step 2: 补齐文案提示**

对用户容易混淆的页面，必要时把文案从“金额/额度”改成更明确的：

- `余额（CNY）`
- `额度限制（USD）`
- `今日成本（USD）`

注意：

- 不是每个位置都要加括号
- 只在语义可能冲突的地方加说明

### Task 5: 评估订阅套餐价格的语义归属

**Files:**
- Inspect: `backend/internal/handler/payment_handler.go`
- Inspect: `backend/internal/service/payment_order.go`
- Inspect: `backend/ent/schema/subscription_plan.go`
- Inspect: `frontend/src/views/user/PaymentView.vue`
- Inspect: `docs/PAYMENT_CN.md`

**Step 1: 确认 `plan.price` 的运营口径**

关键问题：

- 订阅套餐价格是否与充值、支付通道共用人民币支付链路
- 运营是否把套餐价格理解为“人民币售价”
- 有没有任何页面或文档把套餐价格声明为 USD

**Step 2: 按确认结果处理**

- 如果套餐价格本质是人民币售价，则套餐价格统一改为 `CNY`
- 如果套餐价格本质是美元定价但通过人民币渠道支付，则不能直接只改符号，必须先补清晰的换算与展示策略

基于当前代码线索，我更倾向于它现在实际被当成“人民币售价”，但正式实施前仍应确认一次。

### Task 6: 增加测试，防止后续再次混用

**Files:**
- Create: `frontend/src/utils/__tests__/formatCurrency.spec.ts`
- Modify: 相关 Vue 组件测试文件

**Step 1: 为格式化函数补单测**

覆盖至少这些场景：

- `formatCNY(12.5)` 输出人民币格式
- `formatUSD(12.5)` 输出美元格式
- 小额小数位保留
- `null/undefined` 默认值
- `zh` / `en` locale 差异

**Step 2: 为关键页面补渲染断言**

至少覆盖：

- 余额组件显示 `¥`
- usage/quota 组件显示 `$`
- 支付结果页显示 `¥`

**Step 3: 增加 lint/静态约束**

如果团队接受，可以补一个轻量约束：

- 在 code review 规则中约定“模板里禁止直接硬编码 `$` 或 `¥`”
- 或增加一个简单的 grep 校验脚本，只对白名单文件放行

### Task 7: 同步文档与口径

**Files:**
- Modify: `README.md`
- Modify: `README_EN.md`
- Modify: `README_JA.md`
- Modify: `docs/PAYMENT_CN.md`
- Modify: `docs/PAYMENT.md`
- Modify: `CHANGELOG.md`

**Step 1: 在支付文档里明确说明**

建议新增一句口径：

- 支付、充值、用户余额按人民币计价与展示
- API 使用成本、额度限制、订阅组的 `*_usd` 限额按美元语义保留

**Step 2: README 只补必要说明**

- 不需要在 README 展开很多细节
- 只需避免 README 与实际系统展示口径冲突

**Step 3: 更新 CHANGELOG**

- 记录这次“货币语义统一”的用户可见变更

## 推荐实施顺序

1. 先做金额语义台账
2. 再抽公共格式化函数
3. 第一阶段修正余额/充值/支付链路为 CNY
4. 第二阶段把 usage/quota/limit 显式固定为 USD
5. 最后补测试、文档、CHANGELOG

## 验收标准

### 用户侧验收

- 用户在余额相关页面不再看到被误解为美元的金额
- 充值页、支付页、支付成功页金额符号一致
- 订阅套餐价格若按人民币运营，则全链路符号一致

### 管理侧验收

- 管理员加减余额页面与用户余额页面语义一致
- 后台订单金额与支付结果金额一致
- Group/API Key 的 USD 限额页面仍保持美元语义

### 技术验收

- 前端关键货币显示不再散落硬编码
- 新增格式化函数有单测覆盖
- 不修改数据库 schema
- 不破坏现有前后端 API 契约

## 风险与防呆

### 风险 1：误把 USD usage 页面改成 CNY

防呆：

- 只要字段名带 `*_usd`，默认按 USD 处理
- usage/quota/rate limit 页面统一复核

### 风险 2：订阅价格语义判断错误

防呆：

- 在正式改前确认一次运营口径
- 没确认前，不要批量把所有套餐价直接替换为 `¥`

### 风险 3：旧页面漏改

防呆：

- 用 `rg` 建立硬编码清单
- 改完后再跑一次全仓扫描

## 建议结论

建议改，但按“分语义统一”来改，而不是“全站统一一种货币符号”。

最合理的落地方式是：

- `余额/充值/支付/订单金额` 统一为 `CNY`
- `成本/额度/限额` 继续为 `USD`
- 前端统一收口到公共格式化函数
- 用测试和文档把这套口径固定下来

这样既能解决当前“人民币显示成美元”的误导问题，又不会把本来合理存在的美元成本体系误伤掉。
