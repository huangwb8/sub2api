# 盈利水平面板指标计算分析

**日期**：2026-04-20
**目的**：分析盈利水平面板中各指标的计算逻辑，并结合远程站点真实数据验证截图数值的来源。

## 截图中的数据

截图展示的是**非管理员用户**在"所有时间"范围的盈利水平面板：

| 指标 | 截图数值 |
|------|----------|
| 余额收入 | ¥0.000 |
| 订阅收入 | ¥360.00 |
| 估算成本 | ¥1.70K |
| 利润 | -¥1.34K |
| 额外盈利率 | -78.80% |

## 远程站点真实数据

通过 API（`GET /api/v1/admin/dashboard/profitability`）获取 2026-04-13 至 2026-04-20 按天粒度数据：

| 日期 | 余额收入 | 订阅收入 | 估算成本 | 利润 | 额外盈利率 |
|------|----------|----------|----------|------|-----------|
| 04-13 | 0 | 270.00 | 12.34 | 257.66 | 2088.29% |
| 04-14 | 0 | 0 | 475.36 | -475.36 | -100.00% |
| 04-15 | 0 | 90.00 | 421.13 | -331.13 | -78.63% |
| 04-16 | 0 | 0 | 489.18 | -489.18 | -100.00% |
| 04-17 | 0 | 0 | 128.74 | -128.74 | -100.00% |
| 04-18 | 0 | 0 | 159.18 | -159.18 | -100.00% |
| 04-19 | 0 | 0 | 12.49 | -12.49 | -100.00% |

### 汇总验证

前端的 `summarizeProfitabilityTrend()` 函数将每日数据累加后取整：

| 指标 | 计算过程 | 汇总值 | 截图显示 | 匹配 |
|------|----------|--------|----------|------|
| 余额收入 | 0+0+0+0+0+0+0 | **¥0** | ¥0.000 | ✅ |
| 订阅收入 | 270+0+90+0+0+0+0 | **¥360** | ¥360.00 | ✅ |
| 估算成本 | 12.34+475.36+421.13+489.18+128.74+159.18+12.49 | **¥1698.41** | ¥1.70K | ✅（千位缩写） |
| 利润 | 257.66-475.36-331.13-489.18-128.74-159.18-12.49 | **-¥1338.41** | -¥1.34K | ✅ |
| 额外盈利率 | (-1338.41 / 1698.41) × 100 | **-78.80%** | -78.80% | ✅ |

所有截图数值与 API 返回的原始数据完全吻合。

## 五个指标的计算逻辑

### 余额收入

**定义**：非管理员用户通过余额（按量付费）模式支付的总金额，以人民币（CNY）计价。

**数据来源**：`usage_logs` 表的 `charged_amount_cny` 字段。

**计算过程**（请求时）：
1. 获取上游实际成本（USD）：`totalCostUSD = input_cost + output_cost + cache_cost`
2. 获取账号成本单价：`unitCostCNYPerUSD = account.ActualCostCNY / account.ActualCostUsageUSD`
3. 计算估算成本：`estimatedCostCNY = totalCostUSD × unitCostCNYPerUSD`
4. 应用额外盈利率：`chargedAmountCNY = estimatedCostCNY × (1 + extraProfitRate / 100)`
5. 记录 `charged_amount_cny = chargedAmountCNY`

**SQL 查询**（`usage_log_repo.go:3010-3021`）：
```sql
-- balance_usage CTE
SELECT charged_amount_cny AS revenue_balance_cny
FROM usage_logs ul
LEFT JOIN accounts a ON a.id = ul.account_id
INNER JOIN users u ON u.id = ul.user_id
WHERE u.role <> 'admin'
```

**关键文件**：
- 请求时计算：`backend/internal/service/balance_profitability.go:32-79`
- SQL 查询：`backend/internal/repository/usage_log_repo.go:3010-3021`

---

### 订阅收入

**定义**：非管理员用户通过购买订阅套餐支付的总金额，以人民币（CNY）计价。

**数据来源**：`payment_orders` 表的 `amount` 字段。

**SQL 查询**（`usage_log_repo.go:3023-3036`）：
```sql
-- subscription_orders CTE
SELECT po.amount AS revenue_subscription_cny
FROM payment_orders po
INNER JOIN users u ON u.id = po.user_id
WHERE u.role <> 'admin'
  AND po.order_type = 'subscription'
  AND po.status IN ('completed', 'paid', 'recharging')
```

**时间戳选取**：使用 `COALESCE(completed_at, paid_at, created_at)` 作为订单归属时间。

---

### 估算成本

**定义**：根据账号实际成本配置估算的上游 API 调用成本，以人民币（CNY）计价。

**数据来源**：`usage_logs` 表的 `estimated_cost_cny` 字段，对订阅计费用户使用回退计算。

**SQL 查询**（`usage_log_repo.go:2987-3006`）：
```sql
CASE
  WHEN estimated_cost_cny > 0 THEN estimated_cost_cny
  WHEN billing_type = 1                              -- 订阅计费
    AND actual_cost > 0
    AND a.actual_cost_cny > 0
    AND a.actual_cost_usage_usd > 0
  THEN ROUND(actual_cost * (a.actual_cost_cny / a.actual_cost_usage_usd), 8)
  ELSE 0
END
```

**计算逻辑**：
- **余额计费用户**：直接使用请求时计算好的 `estimated_cost_cny`（基于账号成本单价 × USD 成本）
- **订阅计费用户**：如果 `estimated_cost_cny` 未记录，则回退计算：`actual_cost × (account.actual_cost_cny / account.actual_cost_usage_usd)`

**关键文件**：
- 账号成本配置：`backend/ent/schema/account.go:96-108`
- 成本单价函数：`balance_profitability.go:14-30`

---

### 利润

**定义**：总收入（余额收入 + 订阅收入）减去估算成本。

**计算公式**：
```
利润 = 余额收入 + 订阅收入 - 估算成本
```

**后端计算**（`usage_log_repo.go:3080`）：
```go
ProfitCNY = round((revenueBalanceCNY + revenueSubscriptionCNY - estimatedCostCNY) * 1e8) / 1e8
```

**前端汇总**（`dashboardProfitability.ts:137`）：
```typescript
summary.profitCNY += point.profit_cny
```

---

### 额外盈利率

**定义**：利润占估算成本的百分比。仅当估算成本 > 0 时计算。

**计算公式**：
```
额外盈利率 = (利润 / 估算成本) × 100%
```

**后端计算**（`usage_log_repo.go:3081-3084`）：
```go
if estimatedCostCNY > 0 {
    rate = round((profitCNY / estimatedCostCNY * 100) * 1e4) / 1e4
    ExtraProfitRatePercent = &rate
}
```

**前端汇总**（`dashboardProfitability.ts:154-156`）：
```typescript
if (summary.estimatedCostCNY > 0) {
  summary.extraProfitRatePercent = roundTo((summary.profitCNY / summary.estimatedCostCNY) * 100, 4)
}
```

**配置来源**：`groups` 表的 `extra_profit_rate_percent` 字段（`decimal(10,4)`），定义用户分组级别的加价比例。

**在请求计费中的应用**：
```
chargedAmountCNY = estimatedCostCNY × (1 + extraProfitRatePercent / 100)
```
即先估算上游成本，再乘以 (1 + 盈利率) 得到向用户收取的金额。

## 数据流总览

```
用户请求 → 计算USD成本 → 获取账号成本单价 → 估算成本(CNY)
                                              ↓
                              应用额外盈利率 → 收费金额(CNY)
                                              ↓
                              记录到 usage_logs 表

定时汇总 → GetProfitabilityTrend SQL 查询
        → 后端计算 profit_cny 和 extra_profit_rate_percent
        → 前端 normalize + summarize
        → 卡片 + 趋势图展示
```

## 远程站点现状分析

根据 2026-04-13 至 2026-04-19 的数据：

- **收入结构**：全部来自订阅收入（¥360），无余额收入
- **成本结构**：估算成本 ¥1698.41，主要消耗集中在 04-14 至 04-16（日均 ¥460+）
- **盈利状况**：整体亏损 ¥1338.41，盈利率 -78.80%
- **核心问题**：4月14日后订阅用户消耗远超其订阅套餐价值，仅 04-13 和 04-15 有新增订阅收入（¥270 + ¥90），但 7 天累计估算成本达 ¥1698.41
- **04-13 正利润**：当天订阅收入 ¥270 远高于估算成本 ¥12.34，主要因为 04-13 可能是订阅首次激活日，实际使用量较低
