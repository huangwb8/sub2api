# 帐号管理页用量统计弹窗性能优化计划

## 背景

- 排查时间：2026-04-28（Asia/Shanghai）
- 用户反馈：管理员“帐号管理”界面中，点击“用量/统计”窗口后，信息显示明显偏慢
- 本次工作边界：**不修改业务源码**，仅基于现有前后端实现做根因分析，并输出后续优化计划
- 远程验证限制：尝试基于 `remote.env` 做只读接口测量时，请求被站点前置防护直接拦截为 `403 Forbidden`，因此本次结论以本地源码链路分析为主，未包含线上真实耗时样本

## 先给结论

当前慢点不在“弹窗壳子”本身，而在**弹窗打开后才触发的一次性重型统计请求**。

问题主要集中在三层：

1. 前端把“摘要卡片 + 趋势图 + 模型分布 + 入站端点分布 + 上游端点分布”绑在同一个首次请求上，任何一块慢，整窗都只能继续转圈。
2. 后端 `GET /api/v1/admin/accounts/:id/stats?days=30` 为了拼出这一个弹窗，顺序执行多次 `usage_logs` 聚合查询，而且都围绕同一账号、同一 30 天时间窗反复扫描。
3. 这条链路没有列表页那样的批量统计缓存、也没有弹窗级结果缓存，更没有“先显示摘要，再懒加载图表”的分层返回策略。

换句话说，这个窗口现在更像“临时小型分析页”，而不是“快速详情弹窗”。

## 关键证据

### 前端是“打开弹窗后现拉全量统计”

- `frontend/src/views/admin/AccountsView.vue` 里点击 `@stats` 后仅设置 `showStats=true` 与 `statsAcc`，没有预取或缓存复用。
- `frontend/src/components/admin/account/AccountStatsModal.vue` 里监听 `props.show`，一旦弹窗打开就调用 `adminAPI.accounts.getStats(props.account.id, 30)`。
- 弹窗 UI 不是只显示几个数字，而是同屏等待以下数据全部返回：
  - 30 天历史趋势 `history`
  - 汇总 `summary`
  - 模型分布 `models`
  - 入站端点分布 `endpoints`
  - 上游端点分布 `upstream_endpoints`

这意味着首屏体验被最慢的那部分统计绑定。

### 后端接口是“一个弹窗，五类聚合”

`backend/internal/repository/usage_log_repo.go` 中的 `GetAccountUsageStats(...)` 目前至少会做这些事：

1. 先按天聚合 30 天历史：请求数、tokens、标准成本、账号口径成本、用户口径成本。
2. 再单独查一次 `AVG(duration_ms)`。
3. 再查一次模型维度聚合 `GetModelStatsWithFilters(...)`。
4. 再查一次入站端点维度聚合 `GetEndpointStatsWithFilters(...)`。
5. 再查一次上游端点维度聚合 `GetUpstreamEndpointStatsWithFilters(...)`。

其中后 3 步都是对同一批 `usage_logs` 做新的 `GROUP BY` 聚合。也就是说，同一个弹窗请求会对同一时间窗重复做多轮统计扫描。

### 当前链路没有缓存，也没有分段加载

- `frontend/src/api/admin/accounts.ts` 的 `getStats()` 只是直接 GET，不带 ETag/缓存层。
- `backend/internal/handler/admin/account_handler.go` 的 `GetStats` 也没有像 `today-stats/batch` 那样做快照缓存。
- `backend/internal/service/account_usage_service.go` 只是透传到 repository，没有做结果复用。

对比同页“今日统计”链路：

- 帐号列表页用的是 `POST /api/v1/admin/accounts/today-stats/batch`
- 该接口有批量 SQL 聚合、返回值复用和内存快照缓存

也就是说，同一页面内已经存在“快路径”，但弹窗没有沿用类似思路。

### 现有索引不是主要缺口，问题更像查询组织方式

仓库里已经存在：

- `usage_logs(account_id)`
- `usage_logs(created_at)`
- `usage_logs(account_id, created_at)`

因此这次排查没有证据支持“根因只是缺少账号时间复合索引”。更大的问题是：

- 单次弹窗请求聚合维度过多
- 同一数据窗被重复扫描
- 返回 payload 超出弹窗首屏所需
- 前端没有把“必须先看到的摘要”和“可以稍后补到的图表”分层

## 根因判断

### 根因 1：接口职责过重

当前 `/admin/accounts/:id/stats` 同时承担：

- 摘要卡片数据
- 30 天趋势图
- 模型分布
- 入站端点分布
- 上游端点分布

这对弹窗来说过重，导致“任何一个维度慢，整个窗口都慢”。

### 根因 2：同一时间窗重复聚合

Repository 里虽然每条 SQL 单独看都合理，但组合起来是对同一账号 30 天 usage 做多轮独立扫描和排序。账号 usage 越大，放大效应越明显。

### 根因 3：缺少弹窗场景的性能策略

当前实现缺少以下任何一种：

- 服务端短 TTL 缓存
- 前端同账号结果缓存
- 先摘要后图表的渐进加载
- Top N 裁剪
- 汇总接口与明细接口拆分

因此这个弹窗在交互层面天然会给人“反应慢”的感受。

### 根因 4：图表数据没有首屏优先级控制

弹窗中的 `Line`、`ModelDistributionChart`、`EndpointDistributionChart` 都依赖完整统计返回后才能渲染。即使用户最关心的只是“最近 30 天大概花了多少、今天用了多少”，也必须等完整分析结果回来。

## 非根因或次要因素

- 没有证据表明慢点来自弹窗开关动画、基础样式或简单的 Vue 状态更新。
- 没有证据表明是缺少 `account_id + created_at` 复合索引导致的纯数据库索引缺失问题。
- Chart.js 的渲染有一定成本，但从现有结构看，它更像“放大体感”的次要因素，不像首要根因。

## 优化目标

把“统计弹窗”从一次性重型分析请求，改成：

- 先在 200ms 量级内打开窗口与骨架屏
- 先展示最关键摘要
- 再按需加载趋势与分布图
- 同账号重复打开时尽量命中缓存
- 把后端对 `usage_logs` 的重复扫描次数降下来

## 优化方案总览

建议分三阶段推进，先低风险止痛，再做结构优化。

## 阶段一：补观测，先量化慢在哪里

### 任务

- 为账号统计接口补充分段耗时埋点
- 记录 payload 大小与各维度返回条数
- 区分摘要查询、历史趋势、模型聚合、端点聚合的单段耗时

### 建议改动范围

- `backend/internal/handler/admin/account_handler.go`
- `backend/internal/service/account_usage_service.go`
- `backend/internal/repository/usage_log_repo.go`
- 如果项目已有 ops/日志埋点入口，则同步接入该入口

### 预期收益

- 避免只凭体感优化
- 能快速判断究竟是“摘要慢”还是“端点分布慢”
- 为后续拆接口和缓存 TTL 提供依据

### 验收标准

- 服务端日志或指标中能看到：
  - `account_id`
  - `days`
  - `history_query_ms`
  - `avg_duration_query_ms`
  - `model_stats_query_ms`
  - `endpoint_stats_query_ms`
  - `upstream_endpoint_stats_query_ms`
  - `response_payload_bytes`
  - `models_count`
  - `endpoints_count`
  - `upstream_endpoints_count`

## 阶段二：先把弹窗首屏变快

### 任务 A：拆分“首屏摘要”和“重图表数据”

建议把当前单接口拆成两层，至少做到：

- `summary`：总成本、今日概览、最高消耗日、最高请求日、平均值、活跃天数
- `details`：history / models / endpoints / upstream_endpoints

可选方案：

1. 新增两个接口
2. 保持一个接口，但增加 `include=` 参数，例如 `include=summary`、`include=history,models,endpoints`

推荐优先采用第 2 种，改动面更小。

### 任务 B：前端改为渐进加载

弹窗打开后建议按这个顺序：

1. 立即展示弹窗框架与账号头部
2. 先请求 `summary`
3. 摘要出来后立即渲染卡片
4. 趋势图和两个分布图延后并行加载
5. 图表区域各自有 skeleton，不阻塞整个弹窗

### 任务 C：前端增加同账号短期缓存

建议在弹窗层或页面层对 `accountId + days` 做短期内存缓存，例如 30 秒到 2 分钟。

适用场景：

- 管理员频繁开关同一账号弹窗
- 在多个账号之间来回对比后再次打开前一个账号

### 预期收益

- 用户会先看到关键信息，而不是长时间空转
- 即使图表仍慢，体感也会显著改善
- 重复打开同一账号时延迟可显著下降

## 阶段三：收敛后端查询成本

### 任务 A：减少重复扫描

目标不是“把 SQL 写得更炫”，而是减少同一时间窗的重复聚合次数。建议按优先级选择：

1. 把 `summary` 与 `avg_duration` 合并为更少的查询
2. 对 `history` 保持单独查询
3. 对 `models / endpoints / upstream_endpoints` 视需要并行或重构为更高复用的聚合路径

如果现阶段不适合重写为复杂 CTE，至少也应该优先把最核心摘要与最重图表分离，避免全量串行等待。

### 任务 B：对分布图结果做 Top N 裁剪

弹窗里的饼图不需要无上限返回所有模型和端点。建议：

- 模型分布：只返回前 8 到 12 项，其余合并为 `Others`
- 端点分布：只返回前 8 到 12 项，其余合并为 `Others`

这样可以同时降低：

- SQL 排序/扫描后的返回体积
- JSON payload 大小
- 前端 Doughnut 图渲染开销
- 表格行数导致的视觉噪音

### 任务 C：增加服务端短 TTL 缓存

对管理员账号统计这类“读多写多但允许轻微延迟”的场景，建议增加短 TTL 缓存：

- Key：`account_stats:{accountID}:{days}:{shape}`
- TTL：30 秒到 120 秒

缓存策略建议保守起步：

- 先只缓存 `summary`
- 图表维度单独缓存
- 先不做复杂失效，短 TTL 足够降低重复打开压力

### 预期收益

- 单次请求数据库压力下降
- 热门账号被反复查看时不再重复做全量统计
- 后续即使 usage 数据持续增长，弹窗性能也更稳定

## 最小实施顺序

为了控制风险，建议按下面顺序落地：

1. 补接口分段耗时观测
2. 拆 `summary` 与 `details`
3. 前端改为先摘要后图表
4. 给前端加短期缓存
5. 给后端 `summary` 加短 TTL 缓存
6. 对模型/端点分布做 Top N 裁剪
7. 再评估是否需要继续压缩 SQL 次数或重构聚合查询

这个顺序的好处是：前 4 步已经足以显著改善体感，而且不会一开始就把 repository 复杂化。

## 风险与权衡

### 风险 1：数据从“强实时”变成“近实时”

如果加服务端缓存，弹窗里最近几十秒的数据可能不是绝对最新。

权衡建议：

- 管理后台统计弹窗优先保证响应速度
- 对实时性要求极高的值可以保留“今日统计”单独实时接口

### 风险 2：接口拆分后前端请求数变多

从 1 个接口拆成 2 到 4 个接口后，请求数会上升。

但只要改成并行且分层渲染，总体体感通常更好，因为首屏不再等待最重的分布图。

### 风险 3：Top N 裁剪会隐藏长尾明细

如果管理员确实需要看所有端点，弹窗不是最佳载体。

建议：

- 弹窗只保留 Top N
- 完整分析跳转到更适合的独立统计页或导出页

## 验证计划

代码改造阶段完成后，建议至少验证以下内容：

### 前端体验验证

- 打开弹窗后是否立即显示头部与摘要 skeleton
- 摘要是否先于图表出现
- 同账号二次打开是否明显更快
- 图表区域失败时是否不会把整个弹窗拖成空白

### 后端性能验证

- 单次打开弹窗的 SQL 聚合次数是否下降
- `summary` 请求的 P50 / P95 是否明显低于当前全量接口
- payload 大小是否因 Top N 与拆分而下降

### 回归验证

- 汇总数字与旧接口口径一致
- 30 天趋势图数据不丢失
- 模型与端点分布在 Top N 策略下仍能反映主要结构

## 建议涉及文件

第一轮实现预计会涉及：

- `frontend/src/views/admin/AccountsView.vue`
- `frontend/src/components/admin/account/AccountStatsModal.vue`
- `frontend/src/api/admin/accounts.ts`
- `backend/internal/handler/admin/account_handler.go`
- `backend/internal/service/account_usage_service.go`
- `backend/internal/repository/usage_log_repo.go`

如果需要补测试，还应覆盖：

- 前端弹窗加载行为测试
- 后端账号统计接口/聚合逻辑测试

## 最终判断

这次问题的核心不在于“某一行代码特别慢”，而在于**当前交互模型把一个分析型接口直接塞进了弹窗首屏**。

最值得优先做的不是盲目改 SQL，而是先把弹窗从“全量等待”改成“摘要优先、图表延后、结果可缓存”。在这个基础上，再逐步压缩后端重复聚合成本，收益会更稳、风险也更低。
