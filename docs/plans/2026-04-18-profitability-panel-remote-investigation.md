# 盈利水平面板远程排查记录

日期：2026-04-18

## 背景

用户反馈管理后台“盈利水平”面板没有任何内容，页面中央显示“暂无数据”。

本次排查遵循两个约束：

- 不修改任何业务源码
- 仅基于根目录 `remote.env` 中的只读远程排查凭据做实地调查

## 先给结论

当前现象的直接原因不是前端没有请求，也不是前端把正常数据画丢了，而是远端后端接口 `/api/v1/admin/dashboard/profitability` 本身稳定返回 `500`。

前端在 `frontend/src/views/admin/DashboardView.vue` 里对这个接口失败采用了兜底空数组，因此图表组件最终只能显示“暂无数据”。

这意味着：

- “盈利水平”面板空白是后端接口失败导致的次生现象
- 前端摘要卡片显示 `¥0.0000` / `--`，也是因为失败后被清空为默认值

## 关键证据

### 1. 前端确实会加载这个面板

本地代码里这块面板会先请求：

- `GET /api/v1/admin/dashboard/profitability/bounds`
- `GET /api/v1/admin/dashboard/profitability`

对应位置：

- `frontend/src/views/admin/DashboardView.vue`
- `frontend/src/api/admin/dashboard.ts`
- `frontend/src/components/charts/ProfitabilityTrendChart.vue`

其中 `loadProfitabilityTrend()` 失败时会执行：

- `profitabilityTrend.value = []`

这正好解释了页面为什么落到“暂无数据”。

### 2. 远端 bounds 接口是正常的

使用 `remote.env` 中的只读管理员 API Key 实测：

`GET /api/v1/admin/dashboard/profitability/bounds`

返回：

```json
{"code":0,"message":"success","data":{"has_data":true,"earliest_date":"2026-04-13"}}
```

这说明远端后端自己也认为“盈利水平”相关数据从 2026-04-13 开始就存在。

### 3. 真正拉趋势时，远端接口稳定 500

实测以下请求全部失败：

- `GET /api/v1/admin/dashboard/profitability?granularity=day`
- `GET /api/v1/admin/dashboard/profitability?start_date=2026-04-13&end_date=2026-04-18&granularity=day`
- `GET /api/v1/admin/dashboard/profitability?start_date=2026-04-18&end_date=2026-04-18&granularity=hour`

统一返回：

```json
{"code":500,"message":"Failed to get profitability trend"}
```

示例请求 ID：

- `fd630eb0-3437-485b-84d0-d70e7ab159d3`
- `3c57c289-f03c-4a4c-bf55-694bb6c02c88`

### 4. 这不是“某一天脏数据”的偶发问题

我还额外请求了一个未来空时间段：

- `GET /api/v1/admin/dashboard/profitability?start_date=2030-01-01&end_date=2030-01-01&granularity=hour`

结果仍然是同样的 `500`。

这个点非常关键，因为它基本排除了“某几条业务数据把今天聚合打坏”的路径。即使查询范围内没有任何业务数据，接口仍然失败，说明问题更像是：

- 这条盈利聚合 SQL 在远端数据库上天然就会报错
- 或者远端数据库 schema 与当前代码的聚合假设不一致

### 5. 远端站点其他仪表盘接口是正常的

同一套远程凭据下，以下接口都能正常返回：

- `GET /api/v1/admin/dashboard/stats`
- `GET /api/v1/admin/dashboard/trend?granularity=day`
- `GET /api/v1/admin/payment/orders?page=1&page_size=10&order_type=subscription`
- `GET /api/v1/admin/usage?...`

这说明：

- 管理员鉴权是正常的
- 仪表盘并非整体故障
- 使用记录与订阅订单数据本身是可访问的

## 远端版本与实现对照

远端站点当前版本：

```json
{"code":0,"message":"success","data":{"version":"1.0.15"}}
```

本地仓库当前 HEAD 为：

- `edc4b1f6`
- `git describe` 为 `v1.0.15-2-gedc4b1f6`

我对比了以下文件在 `v1.0.15..HEAD` 间的差异：

- `backend/internal/repository/usage_log_repo.go`
- `backend/internal/service/dashboard_service.go`
- `backend/internal/handler/admin/dashboard_handler.go`
- `frontend/src/views/admin/DashboardView.vue`
- `frontend/src/views/admin/dashboardProfitability.ts`
- `frontend/src/components/charts/ProfitabilityTrendChart.vue`

结果是这些与盈利面板直接相关的文件在本地当前分支和 `v1.0.15` 之间没有差异。

这意味着：

- 远端并不是“线上没部署到最新盈利面板修复”
- 当前仓库里这条盈利趋势后端实现，本身就仍然带着这个问题

## 对后端根因的判断

### 已确认的根因层级

已经可以确认的层级：

- 前端只是症状承接方，不是根因
- 根因在后端 `/admin/dashboard/profitability`
- 且更靠近 repository SQL / 数据库 schema，而不是 handler 或前端

### 为什么不是前端问题

因为：

- bounds 接口返回 `has_data: true`
- profitability 趋势接口返回 `500`
- 前端失败分支明确会把趋势数组置空

所以页面空白只是“后端失败后被前端优雅降级”的结果。

### 为什么更像 schema / SQL 兼容问题

盈利趋势的核心实现位于：

- `backend/internal/repository/usage_log_repo.go`

它会聚合：

- `usage_logs.charged_amount_cny`
- `usage_logs.estimated_cost_cny`
- `payment_orders.amount`

而 `bounds` 接口只取最早时间，不涉及这些金额字段。

我还用临时 PostgreSQL 15 最小 schema 验证过：在“列类型符合当前代码预期”的情况下，这条 SQL 可以正常执行。

因此，当前最可信的解释不是“SQL 语法天然有错”，而是：

- 远端数据库里，盈利聚合依赖的某些列实际类型或兼容性状态，和代码假设不一致
- 这种不一致在 `/admin/usage` 这种原样读取场景下未必暴露
- 但在 `COALESCE(...)`、`SUM(...)` 这类聚合表达式中会立即触发数据库错误

### 当前最高可信的怀疑点

优先怀疑以下列在远端数据库的真实类型/状态与预期不一致：

- `usage_logs.charged_amount_cny`
- `usage_logs.estimated_cost_cny`

理由：

- 它们只在盈利聚合里被显式做数值运算
- `bounds` 接口完全不依赖它们
- 即使查未来空区间也会失败，符合“查询编译期/类型解析期就报错”的表现

## 本次排查里做过的排除

### 已排除：前端未请求

前端确实会请求，而且有明确失败兜底逻辑。

### 已排除：管理员凭据失效

同一凭据可以正常访问其他 admin 接口。

### 已排除：仪表盘整体异常

`stats`、`trend`、订单列表、usage 接口都正常。

### 已排除：某一天数据脏导致的偶发失败

未来空时间段仍然稳定 `500`。

### 已排除：SQL 在标准 PostgreSQL 15 上天然语法错误

本地最小 schema 下可正常执行。

## 仍然缺少的一块证据

因为本次排查严格限定为“基于 remote.env 的只读外部调查”，我没有拿到远端数据库的直接 schema 信息，也没有拿到后端进程日志里的原始 SQL 错误文本。

所以目前还不能把根因写成 100% 定论式的某一条数据库报错，例如：

- 某列是 `TEXT` 不是 `NUMERIC`
- 某个聚合表达式命中了特定 PostgreSQL 类型错误

但从证据链看，问题已经非常明确地收敛到：

- 远端后端盈利趋势查询
- 与数据库 schema/聚合兼容性有关

## 建议的下一步

如果下一轮允许继续排查，优先顺序建议是：

1. 在远端数据库直接执行 `\d+ usage_logs` 与 `\d+ payment_orders`，核对：
   - `charged_amount_cny`
   - `estimated_cost_cny`
   - `amount`
2. 在远端后端日志中抓取 `/api/v1/admin/dashboard/profitability` 的真实 SQL 错误文本
3. 对照 `backend/migrations/104_add_usage_fx_snapshot.sql` 与 `backend/migrations/105_add_balance_profitability_fields.sql`，确认生产库是否出现过“列已存在但类型不是 migration 期望类型”的漂移

在拿到那条原始 SQL 错误之前，我对本次结论的表述是：

- 结论级别：后端接口故障，已确认
- 根因级别：数据库 schema / 聚合兼容性问题，高可信但尚未拿到最终报错文本

