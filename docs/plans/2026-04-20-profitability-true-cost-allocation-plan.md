# 盈利面板真实成本分摊优化计划

**日期**：2026-04-20
**状态**：草案，待评审
**目标**：将盈利面板的"估算成本"从 token 标价折算改为基于账号真实成本的比例分摊，消除虚假亏损。

## 问题

当前盈利面板使用以下公式计算"估算成本"：

```
估算成本 = actual_cost(USD) × (actual_cost_cny / actual_cost_usage_usd)
```

其中：
- `actual_cost`：该次请求的 token 标价（USD），由模型定价表计算
- `actual_cost_cny`：管理员填写的账号真实成本（¥），如 ChatGPT Plus 月费 ¥150
- `actual_cost_usage_usd`：账号从上次重置到现在的累计 token 标价（USD）

**这个公式本质上是"按 token 标价的比例分摊账号真实成本"**——逻辑方向没问题，但存在一个关键缺陷：

### 累计值导致成本失真

`actual_cost_usage_usd` 是一个**随时间持续累加**的值。当管理员设置 `actual_cost_cny` 后，系统重置 `actual_cost_usage_usd = 0`，然后逐次请求累加 token 标价。问题是：

1. **初期单位成本偏高**：累加初期，`actual_cost_usage_usd` 很小，导致 `actual_cost_cny / actual_cost_usage_usd` 比值极大，单次请求的估算成本远超真实分摊
2. **跨周期混淆**：账号的真实成本（如月费 ¥150）对应的是一个固定周期（一个月），但 `actual_cost_usage_usd` 的累加周期与计费周期不同步
3. **Token 标价 ≠ 真实价值**：auth 号（如 ChatGPT Plus）的 token 标价可能远高于订阅费对应的真实价值。一个 ¥150/月的账号，一个月消耗的 token 标价可能达 $200+，但真实成本就是 ¥150

### 实际表现

远程站点 2026-04-14 至 2026-04-19 的数据：

| 指标 | 数值 |
|------|------|
| 订阅收入 | ¥360 |
| 估算成本 | ¥1,698 |
| 利润 | -¥1,338 |
| 盈利率 | -78.80% |

管理员看起来亏损 ¥1,338，但真实情况是：账号的月费远低于 ¥1,698，亏损是被 token 标价"算出来"的，不是真实亏损。

## 目标模型

### 核心公式

```
用户在某时段内的真实成本 = c × b / a
```

其中：
- **c**：账号真实成本（¥/周期），即管理员配置的 `actual_cost_cny`，如 ChatGPT Plus 月费 ¥150
- **a**：账号在对应周期内的总容量/总配额（token 或请求数）
- **b**：用户在对应时段内的实际消耗（token 或请求数）

### 举例

假设一个 ChatGPT Plus 账号：
- c = ¥150/月
- a = 该月总配额（假设 3000 次请求）
- 用户 A 本月用了 300 次请求：真实成本 = ¥150 × 300/3000 = ¥15
- 用户 B 本月用了 1500 次请求：真实成本 = ¥150 × 1500/3000 = ¥75
- 总计：¥90（两个用户合计的真实分摊成本），而非 token 标价折算的几百甚至上千元

## 数据基础盘点

系统**已有**丰富的账号配额和使用量数据，关键在于如何利用。

### 已有：账号真实成本 (c)

- 字段：`accounts.actual_cost_cny`
- 设置方式：管理员在后台手动填写
- 位置：`account.go:112-116`，前端 `EditAccountModal.vue:1069`

### 已有：账号配额信息 (a 的来源)

系统通过 `account_usage_service.go` 已能获取多种上游配额：

| 平台 | 账号类型 | 配额信息 | 数据来源 |
|------|----------|----------|----------|
| Anthropic | OAuth | 5h 窗口使用率、7d 窗口使用率 | `UsageInfo.FiveHour`, `UsageInfo.SevenDay` |
| Anthropic | Setup Token | 5h 窗口推算 | `session_window` 推算 |
| OpenAI | OAuth | 5h 窗口使用率、7d 窗口使用率 | Codex Usage API |
| Gemini | 各等级 | RPD/RPM 配额 | `gemini_quota.go` |
| Antigravity | 各等级 | 模型级使用率 | `antigravity_quota_fetcher.go` |

`UsageProgress` 结构已包含：
- `Utilization`：使用率（0-100%）
- `UsedRequests` / `LimitRequests`：已用/限额请求数
- `WindowStats`：窗口期统计（请求数、token 数、成本）

### 已有：用户消耗数据 (b 的来源)

`usage_logs` 表每条请求记录包含：
- `input_tokens`、`output_tokens`：token 级消耗
- `total_cost`：token 标价（USD）
- `actual_cost`：扣除后成本（USD）
- `account_id`：关联的账号

### 缺失：账号总容量 (a) 的持久化

当前配额信息是**实时查询**的（从上游 API 获取），没有持久化到数据库。要做按比例分摊，需要知道"这个账号在这个计费周期内总共有多少容量"。

## 方案设计

### 方案 A：周期对齐分摊法（推荐）

为每个账号引入"成本核算周期"概念，与上游计费周期对齐。

#### 新增字段

```
accounts 表：
  actual_cost_cny          -- 已有，账号真实成本（¥/周期）
  cost_billing_cycle       -- 新增，计费周期类型：monthly / weekly / daily
  cost_cycle_starts_at     -- 新增，当前周期开始时间
  cost_cycle_capacity      -- 新增，当前周期总容量（以请求数或 token 数计）
  cost_cycle_capacity_unit -- 新增，容量单位：requests / tokens / usd_notional
```

#### 计算流程

1. **周期开始时**（管理员设置或自动检测）：
   - 从上游 API 获取账号配额 → 写入 `cost_cycle_capacity`
   - 重置周期内累计使用量

2. **每次请求后**：
   - 在 `usage_logs` 中记录 token 消耗（已有）
   - 计算本次请求的真实成本分摊：`cost_per_unit = actual_cost_cny / cost_cycle_capacity`，`real_cost_cny = cost_per_unit × 本次消耗量`
   - 写入 `usage_logs.real_cost_cny`（新增字段）

3. **盈利面板查询时**：
   - 用 `SUM(real_cost_cny)` 替代当前的 `estimated_cost_cny`
   - 利润 = 收入 - 真实分摊成本
   - 盈利率 = 利润 / 真实分摊成本 × 100%

#### 容量获取策略

| 账号类型 | 容量获取方式 | 精确度 |
|----------|------------|--------|
| Anthropic OAuth | `LimitRequests` from usage API | 高（精确请求限制） |
| OpenAI OAuth | Codex usage 5h/7d limit 数据 | 中（需外推到月） |
| API Key | 按 token 标价限额（如 API 预算 $100） | 高（直接用） |
| Gemini | RPD × 天数外推 | 中 |
| 未知/无法获取 | 管理员手动填写 | 取决于管理员 |

#### 优点
- 成本分摊与真实账号成本对齐
- 不同用户之间的成本分摊公平合理
- 可准确反映盈亏状况

#### 缺点
- 需要额外字段和迁移
- 容量数据需要定期同步
- 对于无法获取精确配额的账号，需要管理员手动维护

---

### 方案 B：窗口滚动分摊法（轻量替代）

不引入"周期"概念，而是直接利用已有的 `UsageProgress` 中的 `Utilization` 数据。

#### 计算公式

```
某次请求的真实成本分摊 = actual_cost_cny × (本次消耗 / 窗口容量)
```

其中"窗口容量"取当前活跃窗口（5h 或 7d）的 `LimitRequests` 或 token 总量。

#### 优点
- 不需要新增字段，利用现有 `UsageProgress` 数据
- 实现简单，改动小

#### 缺点
- 窗口容量 ≠ 计费周期容量，分摊可能不准确
- 5h 窗口的数据可能不够稳定

---

### 方案 C：事后核算法（最简单）

不改实时计费逻辑，只在盈利面板查询时做一次"重新核算"。

#### 计算公式

```
真实分摊成本 = actual_cost_cny × (该用户 token 消耗 / 该账号在周期内总 token 消耗)
```

#### 实现方式
1. 盈利面板查询时，额外获取账号的 `actual_cost_cny`
2. 统计该账号在查询时段内的总 `actual_cost`（USD token 标价累计）
3. 按比例分摊：`用户真实成本 = actual_cost_cny × (用户 actual_cost / 账号总 actual_cost)`

#### 优点
- 零侵入：不改动 usage_logs 写入逻辑
- 只改盈利面板的查询 SQL
- 历史数据自动适配

#### 缺点
- 仍然是事后估算，不是精确分摊
- 如果管理员没有配置 `actual_cost_cny`，则无法计算

## 推荐路径

分两阶段实施：

### 阶段一：方案 C（快速修正，1-2 天）

先用事后核算法修正盈利面板，让面板数据反映真实成本分摊，消除虚假亏损。

**改动范围**：
- `usage_log_repo.go` 的 `GetProfitabilityTrend` SQL：新增 CTE，按账号分摊 `actual_cost_cny`
- `dashboardProfitability.ts`：无变化（接口返回格式不变）
- 前端图表：无变化

**关键 SQL 思路**：
```sql
-- 对每个账号，在查询时段内的总 token 标价作为分母
-- 按用户使用比例分摊 actual_cost_cny
WITH account_cost_allocation AS (
  SELECT
    ul.account_id,
    a.actual_cost_cny,
    SUM(ul.actual_cost) AS total_account_usage_usd
  FROM usage_logs ul
  JOIN accounts a ON a.id = ul.account_id
  WHERE ul.created_at >= $1 AND ul.created_at < $2
  GROUP BY ul.account_id, a.actual_cost_cny
)
SELECT
  ul.user_id,
  SUM(
    CASE WHEN aca.total_account_usage_usd > 0 AND aca.actual_cost_cny > 0
      THEN aca.actual_cost_cny * (ul.actual_cost / aca.total_account_usage_usd)
      ELSE 0
    END
  ) AS real_allocated_cost_cny
FROM usage_logs ul
JOIN account_cost_allocation aca ON aca.account_id = ul.account_id
-- ...
```

### 阶段二：方案 A（精确分摊，后续迭代）

在阶段一验证可行后，引入完整的周期对齐分摊模型。

**改动范围**：
- Account schema 新增字段
- 数据库迁移
- 前端账号编辑界面新增容量配置
- usage_logs 写入时实时计算 `real_cost_cny`
- 上游配额自动同步服务

## 影响范围

| 改动点 | 阶段一 | 阶段二 |
|--------|--------|--------|
| usage_log_repo.go SQL | ✏️ 修改查询 | ✏️ 修改查询 |
| balance_profitability.go | 不动 | ✏️ 改实时计算 |
| Account schema | 不动 | ➕ 新增字段 |
| usage_logs schema | 不动 | ➕ 新增字段 |
| 前端面板 | 不动 | 不动（接口格式不变） |
| 前端账号编辑 | 不动 | ✏️ 新增容量配置 |
| 数据库迁移 | 不需要 | ➕ 新迁移文件 |

## 待确认事项

1. **阶段一的"分摊"粒度**：是按账号整体分摊，还是按账号+模型分摊？（不同模型的 token 单价不同）
2. **API Key 类型账号**：这类账号是按量付费，token 标价接近真实成本，是否保持现有逻辑？
3. **多账号负载均衡**：一个用户的请求可能分散到多个账号，分摊时如何处理？
4. **管理员的 `actual_cost_cny` 不填**：目前已有 fallback 逻辑，保持不变？
5. **是否需要区分"真实成本分摊"和"token 标价折算"两个面板指标**，还是直接替换？
