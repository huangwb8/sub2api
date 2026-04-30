# 住宅 IP 流量估算整改 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 将 sub2api 当前“住宅 IP 流量按 token 近似估算”的粗口径升级为“语义明确、可校准、可回溯、能和供应商账单持续对账”的估算体系，使套餐定价与站点真实基础设施成本都能得到合理反映。

**Architecture:** 本次整改分四层推进。第一层先冻结当前误差与语义，避免后续实现目标漂移；第二层补齐 usage 侧的代理流量归因与真实字节观测基础；第三层将当前单一估算器重构为“定价口径”和“站点口径”双视图，并引入可校准的 `effective_bytes_per_token` 与误差监控；第四层通过后台 API、前端展示和供应商账单对账流程，把估算结果变成可运营、可持续修正的系统能力。

**Tech Stack:** Go, Gin, PostgreSQL, Ent ORM, SQL migration, Vue 3, TypeScript, Pinia, pnpm, Go test, Vitest

**Minimal Change Scope:** 允许修改 `backend/internal/service/dashboard_oversell_service.go`、相关测试、`frontend/src/types/index.ts`、`frontend/src/api/admin/dashboard.ts`、`frontend/src/views/admin/DashboardView.vue`、`backend/ent/schema/usage_log.go`、新增 `backend/migrations/*`、以及与 usage log 写入直接相关的服务代码。避免改动代理调度策略、账号 failover 策略、非住宅 IP 成本逻辑、以及与本计划无关的 Dashboard 面板。

**Success Criteria:** 1. 管理后台能同时区分“套餐定价口径”和“站点真实住宅 IP 成本口径”。 2. 住宅 IP 估算结果明确展示是否包含管理员、失败请求、探活流量、校准因子与采样窗口。 3. 近 5 天供应商账单样本可被测试固化，并且系统内可显示估算值、账单值与误差率。 4. `usage_logs` 或聚合层能稳定追踪“本次请求是否走代理、走了哪个代理、估算/实测了多少住宅流量字节”。 5. 后续实现完成后，住宅 IP 估算误差可以通过后台监控持续收敛，而不是长期依赖人工导出 CSV 排查。

**Verification Plan:** `cd backend && go test -tags=unit ./internal/service -run 'TestDashboardOversell|TestResidentialIP'`；`cd backend && go test -tags=unit ./internal/server -run 'Test.*Oversell.*'`；`cd frontend && pnpm test -- DashboardView.spec.ts`；使用管理员只读接口核对 `/api/v1/admin/dashboard/oversell-calculator` 返回字段；用 decodo 账单样本做一次人工对账，确认“估算 GB / 账单 GB / 误差率”可见且可解释。

---

## 背景与发现

### 已确认的问题

1. `2026-04-26` 到 `2026-04-30` 的 decodo 供应商账单真实双向流量为 **9.08 GB**，总花费 **$29.52**。
2. 同一时间窗内，站点只读统计显示总 usage 约为 **13,776 次请求**、**1,373,947,575 tokens**。
3. 按当前代码固定的 `4 Bytes/token` 折算，5 天流量仅约 **5.12 GB**，只覆盖真实账单的约 **56%**。
4. 反推得到的隐含系数约为 **7.10 Bytes/token**，说明当前 `4 Bytes/token` 常量明显偏乐观。
5. 当前后台住宅 IP 估算接口还会排除管理员流量，并默认只看最近最多 14 天、经代理账号产生的成功 usage，因此对“站点真实住宅 IP 成本”会继续低估。
6. 当前算法通过“账号当前是否挂代理”来推断历史 usage 是否经代理，遇到代理 failover、人工换绑或跨节点迁移时，历史归因会失真。
7. 14 天窗口均摊会稀释最近几天的上升流量，导致 run-rate 滞后，不适合做当前成本判断。

### 根因分解

1. **固定系数错误**：当前常量 `dashboardOversellEstimatedBytesPerToken = 4.0` 过低。
2. **口径混杂**：套餐定价口径和站点真实成本口径被混成一个“住宅 IP 成本”概念。
3. **观测缺失**：系统没有记录真实代理字节，只能用 tokens 代理流量。
4. **历史归因不足**：usage log 没有持久化 `proxy_id` 或“是否经住宅代理”的历史快照。
5. **校准闭环缺失**：没有“系统估算 vs 供应商账单”的长期误差看板。

### 非目标

1. 本计划不重做整个 Dashboard 成本体系。
2. 本计划不修改代理健康探测、自动 failover、账号调度策略本身。
3. 本计划不引入新的供应商结算系统。
4. 本计划不直接解决账号真实采购成本分摊问题，只聚焦住宅 IP 流量估算。

---

### Task 1: 冻结当前误差样本与估算语义

**Files:**
- Create: `backend/internal/service/dashboard_oversell_residential_ip_test.go`
- Modify: `backend/internal/service/dashboard_oversell_service_test.go`
- Modify: `backend/internal/server/api_contract_test.go`
- Modify: `frontend/src/types/index.ts`

**Step 1: 写失败测试，固化当前 decodo 误差样本**

新增至少 3 个测试：

```go
func TestResidentialIPEstimate_FiveDaySupplierSampleGap(t *testing.T) {}
func TestResidentialIPEstimate_DistinguishesPricingAndSiteScopes(t *testing.T) {}
func TestResidentialIPEstimate_ReportsCalibrationMetadata(t *testing.T) {}
```

固定以下样本：

- 账单窗口：`2026-04-26` 到 `2026-04-30`
- 真实账单：`9.08 GB`
- 当前 token 折算：约 `5.12 GB`
- 当前误差倍率：约 `1.77x`

**Step 2: 收紧 API 契约，禁止“只有一个住宅 IP 总量字段”**

在 contract test 中要求住宅 IP 估算返回至少包含：

```json
{
  "scope": "pricing|site",
  "includes_admin": true,
  "includes_failed_requests": false,
  "effective_bytes_per_token": 7.1,
  "calibration_source": "supplier_reconciliation|static_default",
  "estimated_total_traffic_gb": 9.08
}
```

**Step 3: 运行测试确认当前实现失败**

Run:

```bash
cd backend && go test -tags=unit ./internal/service -run 'TestResidentialIPEstimate|TestDashboardOversell'
cd backend && go test -tags=unit ./internal/server -run 'Test.*Oversell.*'
```

Expected:

- 当前实现因没有 scope、校准元数据、误差样本校验而失败

**Step 4: 提交测试基线**

```bash
git add backend/internal/service/dashboard_oversell_residential_ip_test.go backend/internal/service/dashboard_oversell_service_test.go backend/internal/server/api_contract_test.go frontend/src/types/index.ts
git commit -m "test: lock residential ip estimation gap and semantics"
```

---

### Task 2: 为 usage_logs 补齐代理流量归因快照

**Files:**
- Modify: `backend/ent/schema/usage_log.go`
- Modify: `backend/internal/service/usage_log.go`
- Modify: `backend/internal/service/usage_service.go`
- Modify: `backend/internal/repository/usage_log_repo.go`
- Create: `backend/migrations/110_add_usage_log_proxy_traffic_fields.sql`
- Modify: `backend/internal/service/gateway_service.go`
- Modify: `backend/internal/service/openai_gateway_service.go`

**Step 1: 为 usage log 增加代理归因字段**

新增建议字段：

```go
ProxyID                *int64
UsedResidentialProxy   bool
ProxyTrafficInputBytes *int64
ProxyTrafficOutputBytes *int64
ProxyTrafficOverheadBytes *int64
ProxyTrafficEstimateSource *string
```

约束：

- 不要求首版所有字段都是真实值
- 允许先写入估算值，但必须标记来源
- 历史行允许为空，避免强制回填阻塞上线

**Step 2: 在 usage 创建请求里同步扩展快照结构**

扩展 `CreateUsageLogRequest`，保证 usage 写入时可携带：

```go
ProxyID              *int64 `json:"proxy_id"`
UsedResidentialProxy bool   `json:"used_residential_proxy"`
```

如果请求未走代理，必须显式写 `UsedResidentialProxy=false`，避免后续统计把“未知”误判为“未走代理”。

**Step 3: 在网关记录 usage 时落历史 proxy 快照**

规则：

- 只要请求实际走过住宅代理，就把当时的 `proxy_id` 落入 usage log
- 后续账号换绑代理，不影响历史 usage 的代理归因
- 若请求在执行中发生代理 failover，首版至少落“最终成功代理”；后续可扩展为链路级多段记录

**Step 4: 运行数据库与 repository 测试**

Run:

```bash
cd backend && go test -tags=unit ./internal/repository -run 'Test.*UsageLog.*'
cd backend && go test -tags=unit ./internal/service -run 'Test.*RecordUsage.*'
```

Expected:

- schema、migration、repository、usage 写入路径全部通过

**Step 5: 提交 usage 归因基础设施**

```bash
git add backend/ent/schema/usage_log.go backend/internal/service/usage_log.go backend/internal/service/usage_service.go backend/internal/repository/usage_log_repo.go backend/internal/service/gateway_service.go backend/internal/service/openai_gateway_service.go backend/migrations/110_add_usage_log_proxy_traffic_fields.sql
git commit -m "feat: persist residential proxy attribution in usage logs"
```

---

### Task 3: 引入双口径住宅 IP 估算器

**Files:**
- Modify: `backend/internal/service/dashboard_oversell_service.go`
- Create: `backend/internal/service/residential_ip_estimator.go`
- Create: `backend/internal/service/residential_ip_estimator_test.go`
- Modify: `backend/internal/server/api_contract_test.go`

**Step 1: 把当前单一估算改为双 scope**

定义两个明确口径：

```go
type ResidentialIPScope string

const (
    ResidentialIPScopePricing ResidentialIPScope = "pricing"
    ResidentialIPScopeSite    ResidentialIPScope = "site"
)
```

语义要求：

- `pricing`：用于套餐测算，默认排除 admin，只统计面向用户的成功请求
- `site`：用于站点真实基础设施成本，包含 admin，并可按配置决定是否纳入失败请求、探活和重试

**Step 2: 把 `4 Bytes/token` 常量替换成可校准配置**

引入：

```go
type ResidentialIPCalibration struct {
    EffectiveBytesPerToken float64
    Source                 string
    LastCalibratedAt       *time.Time
}
```

首版策略：

- 默认值从 `4.0` 提升为配置项
- 若存在供应商对账结果，则优先使用最近有效校准值
- 无校准值时才回退默认常量

**Step 3: 输出更完整的解释字段**

返回值至少包含：

```go
type ResidentialIPEstimate struct {
    Scope                    string
    IncludesAdmin            bool
    IncludesFailedRequests   bool
    IncludesProbeTraffic     bool
    ActualDays               int
    InvolvedUsers            int
    EstimatedTotalTrafficGB  float64
    EstimatedMonthlyTrafficGB float64
    EffectiveBytesPerToken   float64
    CalibrationSource        string
    TrafficBasis             string
}
```

这样前端不会再把一个没有上下文的 `residential_ip_total_traffic_gb` 当成“绝对真实值”。

**Step 4: 运行 estimator 与 oversell 测试**

Run:

```bash
cd backend && go test -tags=unit ./internal/service -run 'TestResidentialIPEstimate|TestDashboardOversell'
```

Expected:

- `pricing` 与 `site` 口径可以并存
- 当前误差样本可被 estimator 测试覆盖

**Step 5: 提交估算器重构**

```bash
git add backend/internal/service/dashboard_oversell_service.go backend/internal/service/residential_ip_estimator.go backend/internal/service/residential_ip_estimator_test.go backend/internal/server/api_contract_test.go
git commit -m "feat: split residential ip estimation into pricing and site scopes"
```

---

### Task 4: 补齐真实字节观测与降级策略

**Files:**
- Modify: `backend/internal/service/gateway_service.go`
- Modify: `backend/internal/service/openai_gateway_service.go`
- Modify: `backend/internal/service/gateway_forward_as_chat_completions.go`
- Modify: `backend/internal/service/gateway_forward_as_responses.go`
- Modify: `backend/internal/service/openai_gateway_images.go`
- Create: `backend/internal/service/residential_ip_traffic_meter.go`
- Create: `backend/internal/service/residential_ip_traffic_meter_test.go`

**Step 1: 设计“真实字节优先、token 折算兜底”的观测顺序**

统一顺序：

1. 如果本次请求有真实代理入/出字节，就直接使用真实字节
2. 如果没有真实字节，但有 usage tokens，则使用 `effective_bytes_per_token` 折算
3. 如果两者都没有，则明确标记为 `unknown`，不能静默记为 0

**Step 2: 对 streaming / SSE / 图片生成单独处理**

必须单独覆盖：

- `/v1/responses`
- `/v1/chat/completions`
- 图片生成路径
- 长连接 / 流式输出路径

至少补如下测试：

```go
func TestResidentialIPTrafficMeter_StreamingResponseCountsOutputBytes(t *testing.T) {}
func TestResidentialIPTrafficMeter_FallsBackToTokenBasedEstimate(t *testing.T) {}
func TestResidentialIPTrafficMeter_UnknownDoesNotSilentlyBecomeZero(t *testing.T) {}
```

**Step 3: 运行网关 usage 记录测试**

Run:

```bash
cd backend && go test -tags=unit ./internal/service -run 'TestResidentialIPTrafficMeter|TestOpenAIRecordUsage|TestGatewayRecordUsage'
```

Expected:

- streaming、图片、普通文本三类路径都可写入住宅 IP 流量观测结果

**Step 4: 提交字节观测与降级逻辑**

```bash
git add backend/internal/service/gateway_service.go backend/internal/service/openai_gateway_service.go backend/internal/service/gateway_forward_as_chat_completions.go backend/internal/service/gateway_forward_as_responses.go backend/internal/service/openai_gateway_images.go backend/internal/service/residential_ip_traffic_meter.go backend/internal/service/residential_ip_traffic_meter_test.go
git commit -m "feat: meter residential proxy traffic with raw-byte fallback"
```

---

### Task 5: 前端与 API 展示改成“可解释的住宅 IP 成本”

**Files:**
- Modify: `frontend/src/types/index.ts`
- Modify: `frontend/src/api/admin/dashboard.ts`
- Modify: `frontend/src/views/admin/DashboardView.vue`
- Modify: `frontend/src/views/admin/__tests__/DashboardView.spec.ts`
- Modify: `frontend/src/i18n/locales/zh.ts`
- Modify: `frontend/src/i18n/locales/en.ts`

**Step 1: 前端类型不再假定只有一套住宅 IP 字段**

把单组字段升级为明确结构，例如：

```ts
export interface ResidentialIpEstimateView {
  scope: 'pricing' | 'site'
  includes_admin: boolean
  includes_failed_requests: boolean
  effective_bytes_per_token: number
  estimated_total_traffic_gb: number
  estimated_monthly_traffic_gb: number
  calibration_source: string
  traffic_basis: string
}
```

**Step 2: 管理后台面板同时展示“套餐定价口径”和“站点口径”**

展示要求：

- 套餐定价口径：用于回答“给用户定价时该算多少住宅 IP 成本”
- 站点口径：用于回答“站点真实基础设施近期到底烧了多少住宅流量”
- 两者差异要有文字解释，不能只给两个数字

**Step 3: 在 UI 上暴露误差与校准状态**

至少展示：

- 当前 `effective_bytes_per_token`
- 校准来源
- 最近对账误差率
- 是否包含 admin

**Step 4: 运行前端测试**

Run:

```bash
cd frontend && pnpm test -- DashboardView.spec.ts
cd frontend && pnpm run typecheck
```

Expected:

- 面板能区分两种口径
- 字段变更通过类型检查

**Step 5: 提交前端展示改造**

```bash
git add frontend/src/types/index.ts frontend/src/api/admin/dashboard.ts frontend/src/views/admin/DashboardView.vue frontend/src/views/admin/__tests__/DashboardView.spec.ts frontend/src/i18n/locales/zh.ts frontend/src/i18n/locales/en.ts
git commit -m "feat: explain residential ip cost with dual-scope dashboard views"
```

---

### Task 6: 建立供应商账单对账与自动校准闭环

**Files:**
- Create: `backend/internal/service/residential_ip_reconciliation_service.go`
- Create: `backend/internal/service/residential_ip_reconciliation_service_test.go`
- Create: `docs/住宅 IP 流量对账与校准说明.md`
- Modify: `docs/订阅套餐限额推算方法与参考.md`

**Step 1: 落一个最小可运行的对账服务**

职责：

- 读取供应商账单样本
- 对照同窗系统估算值
- 计算误差率
- 输出建议校准系数

最低输出：

```go
type ResidentialIPReconciliationResult struct {
    WindowStart          time.Time
    WindowEnd            time.Time
    SupplierTrafficGB    float64
    EstimatedTrafficGB   float64
    RelativeErrorRate    float64
    SuggestedCalibration float64
}
```

**Step 2: 文档同步更新**

把现有文档中的这句旧口径：

```text
最近 14 天经代理账号的成功请求 token 总量，按约 4 Bytes/token 折算双向流量
```

改为“默认可校准估算口径”，并补充“定价口径 / 站点口径”的区别、误差来源、何时使用哪套数据。

**Step 3: 运行文档与服务测试**

Run:

```bash
cd backend && go test -tags=unit ./internal/service -run 'TestResidentialIPReconciliation'
```

Expected:

- 可输出对账结果与建议校准值
- 文档口径与代码新语义一致

**Step 4: 提交闭环与文档**

```bash
git add backend/internal/service/residential_ip_reconciliation_service.go backend/internal/service/residential_ip_reconciliation_service_test.go docs/住宅\ IP\ 流量对账与校准说明.md docs/订阅套餐限额推算方法与参考.md
git commit -m "docs: document residential ip reconciliation and calibration workflow"
```

---

## 风险与决策点

### 主要风险

1. **真实字节采集成本**：若要在代理层拿到精确字节，可能需要额外 hook 或 transport 包装，首版应允许“估算 + 来源标记”并行存在。
2. **历史数据不可回填**：旧 usage_logs 没有 `proxy_id` 快照，历史报表必须承认“部分数据只能按当前账号关系近似回溯”。
3. **不同上游协议差异大**：SSE、长连接、图片生成、WS 模式的字节开销结构不同，不能用一个无上下文常量强行统一。
4. **前端误读风险**：如果 UI 仍只给一个数字，管理员会继续把“定价估算”误当成“真实站点成本”。

### 推荐实施顺序

1. 先做 Task 1 和 Task 3，快速把语义和 API 形状纠正过来。
2. 再做 Task 2 和 Task 4，补齐观测与历史归因基础。
3. 最后做 Task 5 和 Task 6，把对账和校准闭环接入运营流程。

### 上线策略

1. 第一阶段允许保留旧字段，但新字段必须先可读、可观测。
2. 第二阶段前端默认展示新结构，旧字段仅做兼容。
3. 第三阶段在完成 1 到 2 周账单对账后，再考虑废弃旧的单一 `4 Bytes/token` 文案。

## 验收矩阵

| 验收项 | 检查方式 | 通过标准 |
|---|---|---|
| 误差样本被测试固化 | Go unit tests | 5 天 decodo 样本能在测试中复现当前 gap |
| 口径分离 | API contract + UI | 能明确区分 `pricing` 与 `site` |
| 历史归因 | usage log 数据抽样 | usage log 能看到 `proxy_id` / `used_residential_proxy` |
| 真实字节优先 | service tests | 有原始字节时不再退回 token 折算 |
| 校准闭环 | reconciliation service | 能输出误差率与建议校准值 |
| 文档对齐 | docs review | 不再出现“固定 4 Bytes/token 就是真实流量”的误导表述 |
