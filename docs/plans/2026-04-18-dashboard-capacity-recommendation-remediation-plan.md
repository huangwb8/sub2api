# 管理控制台加号推荐整改计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 将管理控制台“加号推荐”从“按订阅分组/套餐机械补号”重构为“按全站真实容量池推荐”，避免共享账号池被重复推荐、闲置套餐稀释基线、以及前端展示语义夸大建议量。

**Architecture:** 保留现有 `/api/v1/admin/dashboard/recommendations` 入口，但重做其数据语义。后端先基于全站活跃订阅分组、`account_groups` 共享关系、平台和账号类型推导“容量池”，再输出“全站摘要 + 容量池推荐项”；套餐名称仅作为负载来源说明，不再作为推荐主维度。推荐基线只使用近 30 天有真实活动的样本，零活跃样本保留展示但不参与人数/成本基线，前端面板同步改为展示“可调度账号缺口”，避免把“总账号数”和“可调度账号数”混成一个“建议新增”。

**Tech Stack:** Go, Gin, Ent ORM, SQL, Vue 3, TypeScript, Pinia, pnpm, Go test, Vitest

---

## 背景判断

### 当前面板的三个核心问题

1. 当前推荐对象是“订阅分组/套餐”，而不是“真实容量池”。
2. 当前平台基线会把零活跃但仍有 schedulable 账号的分组纳入分母，导致 `daily_cost_per_schedulable` 被压低，成本口径的补号建议被放大。
3. 前端把 `recommended_total_accounts - current_schedulable_accounts` 直接显示为“建议新增 X 个号”，会把“已有但暂时不可调度的账号”和“真需要新补的账号”混成一个数字。

### 这次整改的目标语义

- 主推荐对象：容量池，而不是套餐。
- 主摘要视角：全站。
- 套餐角色：解释哪些套餐/分组贡献了当前负载，不再承担主推荐维度。
- 主缺口字段：推荐可调度账号缺口，而不是模糊的“新增账号”。

### 本期边界

本期明确做：

- 以“全站 + 容量池”替换“按分组/套餐”的推荐模型
- 排除零活跃样本对基线的稀释
- 调整前端面板文案与字段语义
- 补齐后端与前端回归测试

本期明确不做：

- 新增数据库 schema 存储固定 `capacity_pool_id`
- 改动账号调度本身
- 引入运营级自动扩容
- 重写整个 Dashboard 统计体系

---

### Task 1: 先用测试冻结当前错误语义和目标语义

**Files:**
- Modify: `backend/internal/service/dashboard_recommendation_service_test.go`
- Create: `backend/internal/service/dashboard_recommendation_pool_test.go`
- Modify: `frontend/src/views/admin/__tests__/DashboardView.spec.ts`
- Modify: `frontend/src/types/index.ts`

**Step 1: 写后端失败测试，覆盖“闲置分组稀释基线”和“共享账号池只应推荐一次”**

至少补下面 3 类用例：

```go
func TestComputePoolBaseline_IgnoreIdleGroupsForCostBaseline(t *testing.T) {}
func TestBuildCapacityPools_SharedAccountsMergeIntoOnePool(t *testing.T) {}
func TestRecommendByPool_DoesNotDuplicateAcrossPlans(t *testing.T) {}
```

断言重点：

- 同平台下，一个零活跃分组不能把 `daily_cost_per_schedulable` 压低。
- 两个共享同一批账号的订阅分组，只生成一个推荐项。
- 推荐项中的套餐/分组名称只出现在 `contributors` 或 `plan_names` 一类解释字段中，不再决定推荐数量。

**Step 2: 写前端失败测试，覆盖新面板语义**

补一个 Dashboard 视图测试，至少验证：

```ts
expect(wrapper.text()).toContain('全站建议新增')
expect(wrapper.text()).toContain('容量池')
expect(wrapper.text()).not.toContain('评估 2 个订阅分组')
```

以及：

- 推荐项展示“当前可调度 / 当前总账号”
- 推荐项展示“建议补充 X 个可调度账号”
- 套餐名称只作为附属说明

**Step 3: 运行测试确认当前实现失败**

Run:

```bash
cd backend && go test -tags=unit ./internal/service -run 'Test(ComputePoolBaseline|BuildCapacityPools|RecommendByPool)'
cd frontend && pnpm test -- DashboardView.spec.ts
```

Expected:

- 后端失败，因为当前只有“按分组”推荐，没有容量池建模
- 前端失败，因为当前文案和字段仍是“订阅分组 / 建议新增 X 个号”

**Step 4: 提交测试基线**

```bash
git add backend/internal/service/dashboard_recommendation_service_test.go backend/internal/service/dashboard_recommendation_pool_test.go frontend/src/views/admin/__tests__/DashboardView.spec.ts frontend/src/types/index.ts
git commit -m "test: lock dashboard recommendation remediation cases"
```

---

### Task 2: 引入“容量池”推导层，替换“分组即推荐对象”的建模

**Files:**
- Create: `backend/internal/service/dashboard_recommendation_pool.go`
- Create: `backend/internal/service/dashboard_recommendation_pool_test.go`
- Modify: `backend/internal/service/dashboard_recommendation_service.go`

**Step 1: 为推荐服务增加容量池中间模型**

建议新增：

```go
type dashboardCapacityPool struct {
    PoolKey                string
    Platform               string
    RecommendedAccountType string
    GroupIDs               []int64
    GroupNames             []string
    PlanNames              []string
    AccountIDs             []int64
    TotalAccounts          int
    SchedulableAccounts    int
}
```

`PoolKey` 建议先按“共享账号连通分量”推导，而不是直接拿套餐名或分组名：

- 如果两个订阅分组通过 `account_groups` 共享任意一个账号，则属于同一容量池
- 如果分组之间没有共享账号，则各自独立成池
- 再用 `platform + dominant_account_type` 给池打展示标签

**Step 2: 实现容量池推导逻辑**

实现建议：

- 查询所有活跃订阅分组
- 查询这些分组关联的活跃账号
- 用“分组 - 账号”二部图做 connected components
- 每个 component 产出一个容量池

伪代码：

```go
for each active subscription group:
    connect group <-> account via account_groups

for each connected component:
    build one capacity pool recommendation target
```

**Step 3: 把当前聚合输入从 `dashboardRecommendationInput` 改成“池输入”**

不要再直接对每个 `group` 生成推荐项，而是先把组级统计汇总到池级：

- 活跃订阅数：池内所有分组求和
- 活跃用户数：池内所有分组求和
- 30 天成本：池内所有分组求和
- 当前账号数 / 可调度账号数：池内去重后账号统计

**Step 4: 运行后端测试**

Run:

```bash
cd backend && go test -tags=unit ./internal/service -run 'Test(ComputePoolBaseline|BuildCapacityPools|RecommendByPool|ComputeDashboardGroupCapacityRecommendation)'
```

Expected: PASS

**Step 5: 提交容量池建模**

```bash
git add backend/internal/service/dashboard_recommendation_pool.go backend/internal/service/dashboard_recommendation_pool_test.go backend/internal/service/dashboard_recommendation_service.go backend/internal/service/dashboard_recommendation_service_test.go
git commit -m "feat: infer dashboard recommendations by capacity pool"
```

---

### Task 3: 重写基线与推荐公式，去掉“闲置样本稀释”和“套餐驱动补号”

**Files:**
- Modify: `backend/internal/service/dashboard_recommendation_service.go`
- Modify: `backend/internal/service/capacity_recommendation_preference.go`
- Modify: `backend/internal/service/dashboard_recommendation_service_test.go`

**Step 1: 让基线只吃“真实活跃样本”**

在 `computeDashboardRecommendationBaselines` 之前先过滤样本：

```go
isActiveSample := input.ActiveSubscriptions > 0 ||
    input.ActiveUsers30d > 0 ||
    input.AvgDailyCost30d > 0
```

规则：

- 活跃样本参与 `subscriptions / active_users / cost` 基线
- 零活跃样本不参与基线，但保留在最终推荐列表中用于展示“当前健康，无需动作”

**Step 2: 推荐值改成“推荐可调度账号数”优先**

当前实现把建议账号总数直接和 `current_schedulable_accounts` 比较，容易把不可调度存量忽略掉。建议改成显式双字段：

```go
RecommendedSchedulableAccounts int
RecommendedAdditionalSchedulable int
CurrentUnschedulableAccounts int
```

推荐理由里要明确：

- 需要“补充新账号”
- 或“先恢复现有不可调度账号”

最小收口规则：

- 如果 `current_total_accounts > current_schedulable_accounts`
- 且缺口不超过不可调度存量
- 默认先提示“恢复可调度能力”，不要直接把全部缺口都措辞成“新增账号”

**Step 3: 推荐维度从“套餐高低”改为“池负载 + 池容量”**

保留这些指标：

- `projected_daily_cost`
- `growth_factor`
- `capacity_utilization`
- `active_subscriptions`

去掉这类暗示套餐是推荐对象的语义：

- “每个套餐单独补号”
- “按套餐逐个出建议账号总数”

**Step 4: 审视 `subscription_capacity_tightness` 的作用边界**

保留该配置，但收紧它的影响范围：

- 它只能调节“保守程度”
- 不能让零活跃样本参与计算
- 不能成为重复补号的根源

必要时把 `BaselineScale` 的下限适度抬高，避免高保守分值把建议放大得过快。

**Step 5: 运行后端测试**

Run:

```bash
cd backend && go test -tags=unit ./internal/service -run 'Test(ComputeDashboard|ComputePoolBaseline|RecommendByPool|BuildCapacityRecommendationPreferenceProfile)'
```

Expected: PASS

**Step 6: 提交算法整改**

```bash
git add backend/internal/service/dashboard_recommendation_service.go backend/internal/service/capacity_recommendation_preference.go backend/internal/service/dashboard_recommendation_service_test.go
git commit -m "fix: remove idle dilution from dashboard recommendations"
```

---

### Task 4: 调整 API 合同，让接口先表达“全站 + 容量池”，再表达套餐贡献

**Files:**
- Modify: `backend/internal/service/dashboard_recommendation_service.go`
- Modify: `backend/internal/handler/admin/dashboard_handler.go`
- Modify: `frontend/src/types/index.ts`
- Modify: `frontend/src/api/admin/dashboard.ts`
- Modify: `backend/internal/server/api_contract_test.go`

**Step 1: 重定义返回结构**

建议收口成：

```go
type DashboardCapacityRecommendationResponse struct {
    GeneratedAt time.Time `json:"generated_at"`
    LookbackDays int `json:"lookback_days"`
    Summary DashboardCapacityRecommendationSummary `json:"summary"`
    Pools []DashboardCapacityPoolRecommendation `json:"pools"`
}
```

其中 summary 建议至少包含：

- `pool_count`
- `group_count`
- `current_schedulable_accounts`
- `recommended_additional_schedulable_accounts`
- `recoverable_unschedulable_accounts`
- `urgent_pool_count`

池项建议包含：

- `pool_key`
- `platform`
- `recommended_account_type`
- `group_names`
- `plan_names`
- `current_total_accounts`
- `current_schedulable_accounts`
- `recommended_schedulable_accounts`
- `recommended_additional_schedulable_accounts`
- `recoverable_unschedulable_accounts`
- `reason`

**Step 2: 保持 endpoint 不变，降低接入面**

继续使用：

```text
GET /api/v1/admin/dashboard/recommendations
```

只改语义，不改路径，减少路由与权限面的额外风险。

**Step 3: 补 API contract test**

增加断言：

- 返回体顶层不再只有 `items`
- 新结构存在 `summary` 与 `pools`
- `pools[*].plan_names` 是解释字段，不是主推荐标识

**Step 4: 运行测试**

Run:

```bash
cd backend && go test -tags=unit ./internal/server ./internal/handler ./internal/service -run 'Test.*Dashboard.*Recommendations'
```

Expected: PASS

**Step 5: 提交 API 合同调整**

```bash
git add backend/internal/service/dashboard_recommendation_service.go backend/internal/handler/admin/dashboard_handler.go frontend/src/types/index.ts frontend/src/api/admin/dashboard.ts backend/internal/server/api_contract_test.go
git commit -m "refactor: reshape dashboard recommendation api around capacity pools"
```

---

### Task 5: 重做前端监测面板，让“全站视角”成为主叙事

**Files:**
- Modify: `frontend/src/views/admin/DashboardView.vue`
- Modify: `frontend/src/views/admin/__tests__/DashboardView.spec.ts`
- Modify: `frontend/src/i18n/locales/zh.ts`
- Modify: `frontend/src/i18n/locales/en.ts`

**Step 1: 改掉“分组 / 套餐”作为主列名**

把表头从：

```ts
group: '分组 / 套餐'
```

改成类似：

```ts
pool: '容量池'
contributors: '涉及套餐'
current: '当前可调度 / 总账号'
recommended: '建议可调度账号'
gap: '缺口'
```

**Step 2: 顶部摘要改成全站口径**

不要再只显示：

- 评估 X 个订阅分组
- 建议新增 X 个号

建议改成：

- 评估 X 个容量池 / Y 个订阅分组
- 全站建议补充 X 个可调度账号
- 其中可优先恢复现有不可调度账号 Z 个

**Step 3: 表格行内明确区分“恢复”与“新增”**

例如：

```vue
当前可调度 / 总账号: 5 / 7
建议可调度账号: 8
缺口: 3
其中现有不可调度可恢复: 2
预计新增账号: 1
```

如果本期不做这么细，也至少要把文案改成：

- `建议补充 X 个可调度账号`

不要继续写：

- `建议新增 X 个`

**Step 4: 套餐名下沉为解释信息**

在容量池名下显示：

- 平台
- 推荐账号类型
- 贡献套餐列表

但不要再让套餐名成为主行标题。

**Step 5: 跑前端测试与类型检查**

Run:

```bash
cd frontend && pnpm test -- DashboardView.spec.ts
cd frontend && pnpm run typecheck
```

Expected: PASS

**Step 6: 提交前端面板整改**

```bash
git add frontend/src/views/admin/DashboardView.vue frontend/src/views/admin/__tests__/DashboardView.spec.ts frontend/src/i18n/locales/zh.ts frontend/src/i18n/locales/en.ts frontend/src/types/index.ts frontend/src/api/admin/dashboard.ts
git commit -m "refactor: present dashboard recommendations by site capacity pools"
```

---

### Task 6: 做上线前回归、灰度观察和文档收口

**Files:**
- Modify: `CHANGELOG.md`
- Inspect: `frontend/src/views/admin/DashboardView.vue`
- Inspect: `backend/internal/service/dashboard_recommendation_service.go`

**Step 1: 做一次测试站点人工回归**

至少复核以下场景：

1. 两个套餐共享同一批账号时，只出现一个容量池推荐项
2. 零活跃套餐不会再把活跃套餐的补号数放大
3. 当前总账号数大于可调度账号数时，面板不会机械显示全部都是“新增”
4. 当池内容量利用率为 0% 且负载极低时，不会再出现明显夸张的建议值

**Step 2: 记录整改后的观察指标**

建议上线后观察 3 天：

- `recommended_additional_schedulable_accounts` 是否大幅回落
- 同一平台共享池是否仍被重复推荐
- 人工判断与面板建议是否更一致

**Step 3: 更新变更记录**

在 `CHANGELOG.md` 的 `Unreleased` 记录：

- 推荐维度从订阅分组改为容量池
- 零活跃样本不再参与推荐基线
- 前端改为展示“可调度账号缺口”

**Step 4: 验证最终命令**

Run:

```bash
cd backend && go test -tags=unit ./...
cd frontend && pnpm test
cd frontend && pnpm run typecheck
```

Expected: PASS

**Step 5: 最终提交**

```bash
git add backend frontend CHANGELOG.md
git commit -m "fix: align dashboard account recommendations with site capacity pools"
```

---

## 推荐实施顺序

1. 先补测试，把当前错误语义锁住。
2. 先在后端完成容量池建模和零活跃样本剔除。
3. 再调整 API 合同和前端类型。
4. 最后改 Dashboard 面板文案和展示。
5. 用测试站点做一次人工对照，确认推荐值回到运营直觉可接受区间。

## 验收标准

- 同一共享账号池不会因多个套餐重复出现多条主推荐。
- 零活跃套餐不会再把活跃套餐的推荐值放大。
- 面板摘要和表格行都以“全站 / 容量池 / 可调度缺口”为主叙事。
- 用户再看这个面板时，不会自然地理解成“每个套餐都要单独补号”。
- 当前测试站点里类似 `GPT-Standard` 这类 0% 容量利用率、小量活跃订阅场景，不再出现明显夸张的“建议新增 8 个号”。
