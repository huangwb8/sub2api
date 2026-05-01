# Codex 配额状态对齐 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 修复 OpenAI Codex 用量已到 100% 但账号管理页和运维可用性仍显示“正常/可用”的状态口径不一致问题。

**Architecture:** 以服务端 `Account.IsSchedulable()` 作为“实际可调度”单一判断入口，把 Codex 5h/7d 100% 派生限流统一同步到账号列表、ops 可用性统计和前端状态展示。前端只负责展示后端返回的可读状态与补充本地兜底判断，不再各处重复发散实现限流逻辑。

**Tech Stack:** Go / Gin / Ent ORM / Vue 3 / TypeScript / Pinia / pnpm / PostgreSQL / Redis

**Minimal Change Scope:** 允许修改 `backend/internal/service/account.go`、`backend/internal/service/openai_gateway_service.go`、`backend/internal/service/account_usage_service.go`、`backend/internal/service/ops_account_availability.go`、`backend/internal/handler/admin/account_handler.go`、`backend/internal/handler/dto/*`、`frontend/src/components/account/*`、`frontend/src/views/admin/AccountsView.vue`、`frontend/src/types/index.ts`、相关 i18n 和测试文件；避免改动账号调度权重、套餐分组策略、支付/计费逻辑、数据库 schema，除非实现中证明必须。

**Success Criteria:** Codex 5h 或 7d 用量达到 100% 且重置时间在未来时，账号管理页不再显示“正常”，状态筛选不会把它计入“正常”，ops 可用性不会把它计入 available，调度路径不会选择它；窗口重置后状态自动恢复为正常可调度。

**Verification Plan:** `cd backend && go test -tags=unit ./internal/service ./internal/handler/admin ./internal/handler/dto ./internal/repository`；`cd frontend && pnpm run typecheck && pnpm test -- AccountStatusIndicator AccountUsageCell accountUsageRefresh AccountsView`；基于 `remote.env` 对 `/api/v1/admin/accounts` 和 `/api/v1/admin/ops/account-availability` 做只读复核。

---

## 背景

2026-05-01 基于 `remote.env` 对远程站点做只读排查后确认，当前 OpenAI active 账号中存在大量 Codex 窗口已达到 100% 的账号，但账号管理页仍展示为“正常”。

关键样本：

| 账号 | 当前字段 | Codex 用量 | 重置时间 | 现象 |
|------|----------|------------|----------|------|
| `kxsw1-team-20260402` | `status=active`, `schedulable=true` | `5h=100%`, `7d=83%` | `5h` 到 `2026-05-01 14:54:01 +0800` | UI 显示正常 |
| `kxsw1-team-20260407` | `status=active`, `schedulable=true` | `5h=42%`, `7d=100%` | `7d` 到 `2026-05-05 15:06:37 +0800` | UI 显示正常 |
| `noah - team - 0427` | `status=active`, `schedulable=true` | `5h=54%`, `7d=100%` | `7d` 到 `2026-05-05 15:47:45 +0800` | UI 显示正常 |
| `elijah.garcia8069 - team - US - 02` | `status=active`, `schedulable=true` | `5h=100%`, `7d=68%` | `5h` 到 `2026-05-01 16:09:15 +0800` | UI 显示正常 |

远程列表页样本中，`13` 个 active 账号里有 `8` 个 Codex 5h 或 7d 已经 `100%`，但仍返回 `schedulable=true`。同时 `/api/v1/admin/ops/account-availability?platform=openai` 仍统计 OpenAI `available_count=13`，说明 ops 可用性统计也没有纳入 Codex 派生限流。

## 根因判断

### 根因 A：持久字段与派生状态脱节

Codex 用量快照写在 `accounts.extra`：

- `codex_5h_used_percent`
- `codex_5h_reset_at`
- `codex_7d_used_percent`
- `codex_7d_reset_at`
- `codex_usage_updated_at`

代码已有 `codexRateLimitResetAtFromExtra(...)` 和 `Account.IsSchedulable()`，可以从这些字段判断账号实际不可调度。但 `rate_limit_reset_at` 没有被稳定同步，导致依赖持久字段的页面和统计仍认为账号正常。

### 根因 B：列表页与 ops 统计重复实现“可用”判断

账号列表和运维统计主要看：

- `status == active`
- `schedulable == true`
- `rate_limit_reset_at <= now`
- `overload_until <= now`
- `temp_unschedulable_until <= now`

这些判断没有调用 `Account.IsSchedulable()`，因此漏掉了 OpenAI Codex extra 中的 100% 限流状态。

### 根因 C：前端状态组件没有 Codex 派生状态

`AccountStatusIndicator.vue` 当前能识别：

- 账号错误
- 普通 429 限流
- 529 过载
- temp unschedulable
- API key / Bedrock 配额耗尽
- 手动暂停

但它没有把 OpenAI OAuth 的 Codex 5h/7d 100% 识别为“限流/限额中”，所以视觉上仍显示 `active` 对应的“正常”。

## 非目标

- 不调整调度权重与负载均衡算法。
- 不新增跨组溢出或套餐降级逻辑。
- 不改变 Codex 用量快照的上游探测方式。
- 不把 `status` 枚举扩展成数据库级新状态，优先用派生状态表达，避免破坏现有 `active/inactive/error` 语义。
- 不在本次计划里处理 70%/85%/95% 的预测性降权；那属于独立的容量治理优化。

## 实施策略

优先级顺序：

| 优先级 | 工作 | 目的 |
|--------|------|------|
| P0 | 统一后端可调度判断 | 防止服务端统计继续误报可用 |
| P0 | 前端状态与筛选对齐 | 防止管理员看到“正常”误判 |
| P1 | 持久化 `rate_limit_reset_at` 同步 | 让旧路径、ops 和缓存更快收敛 |
| P1 | 测试覆盖 Codex 100% 和重置恢复 | 防止后续回归 |
| P2 | 远程只读复核与截图审查 | 验证真实站点表现 |

## Task 1: 后端派生可用性统一

**Files:**

- Modify: `backend/internal/service/account.go`
- Modify: `backend/internal/service/ops_account_availability.go`
- Test: `backend/internal/service/account_quota_reset_test.go` 或新增 `backend/internal/service/account_codex_schedulable_test.go`
- Test: `backend/internal/service/ops_group_stats_test.go`

**Step 1: 为 Codex 100% 派生状态补单元测试**

新增测试覆盖：

- `codex_5h_used_percent=100` 且 `codex_5h_reset_at` 在未来时，`Account.IsSchedulable()` 返回 `false`
- `codex_7d_used_percent=100` 且 `codex_7d_reset_at` 在未来时，`Account.IsSchedulable()` 返回 `false`
- reset time 已过期时，`Account.IsSchedulable()` 返回 `true`
- 非 OpenAI 账号不受 Codex extra 字段影响

Run:

```bash
cd backend
go test -tags=unit ./internal/service -run 'Test.*Codex.*Schedulable|Test.*AccountAvailability' -count=1
```

Expected: 新测试先失败或暴露 ops 统计未使用派生判断。

**Step 2: ops 可用性使用统一判断**

在 `GetAccountAvailabilityStats(...)` 中用 `acc.IsSchedulable()` 作为 `isAvailable` 的核心判断，而不是只拼接字段判断。

同时保留单独 flag：

- `isRateLimited`: 普通 `rate_limit_reset_at` 未来，或者 Codex extra 计算出的 reset 在未来
- `isOverloaded`
- `isTempUnsched`
- `hasError`

这样 ops 页面既能正确统计 available，也能解释为什么不可用。

**Step 3: 避免时间重复漂移**

如果需要保证同一轮统计使用同一个 `now`，新增小型 helper，例如：

```go
func accountCodexRateLimitResetAt(account *Account, now time.Time) *time.Time
```

不要让多个组件各自 `time.Now()` 导致边界秒级不一致。

**Step 4: 验证**

Run:

```bash
cd backend
go test -tags=unit ./internal/service -count=1
```

Expected: service 层测试通过。

## Task 2: Codex 派生限流同步到持久字段

**Files:**

- Modify: `backend/internal/service/account_usage_service.go`
- Modify: `backend/internal/service/openai_gateway_service.go`
- Test: `backend/internal/service/openai_gateway_service_test.go`
- Test: `backend/internal/service/account_usage_service_test.go`

**Step 1: 明确同步触发点**

建议在两条路径同步：

- 网关请求拿到 Codex rate limit headers 后，更新 extra 的同时调用 `syncOpenAICodexRateLimitFromExtra(...)`
- 管理页 `/accounts/:id/usage` 主动/被动探测后，如果探测结果显示 100%，同步 `rate_limit_reset_at`

目标是让后台持久字段尽快反映真实 Codex 100% 状态，减少依赖前端或 ops 现场重算。

**Step 2: 保持 reset 恢复语义简单**

当 Codex reset 时间在未来：

- `rate_limit_reset_at` 更新为更晚的 Codex reset 时间

当 Codex reset 已过期：

- 不强制清空 `rate_limit_reset_at`，交由现有 `IsSchedulable()` 和查询条件判断 `rate_limit_reset_at <= now` 即可恢复
- 如现有清理路径已有安全清空逻辑，可以复用，但不要新增周期任务

**Step 3: 保护已有上游 429 限流**

如果账号已有未来的 `rate_limit_reset_at`，且比 Codex reset 更晚，不能被较短的 Codex reset 覆盖。

Expected:

- 普通上游 429 和 Codex 100% 同时存在时，保留更保守的恢复时间
- Codex 100% 不会缩短已有限流窗口

**Step 4: 验证**

Run:

```bash
cd backend
go test -tags=unit ./internal/service -run 'Test.*Codex.*RateLimit|Test.*OpenAI.*Usage' -count=1
```

Expected: Codex 快照同步测试通过。

## Task 3: 账号列表接口暴露清晰状态

**Files:**

- Modify: `backend/internal/handler/dto/types.go`
- Modify: `backend/internal/handler/dto/mappers.go`
- Modify: `frontend/src/types/index.ts`
- Test: `backend/internal/handler/admin/account_handler_test.go` 或相关 handler 测试

**Step 1: 增加只读派生字段**

建议在账号 DTO 中新增派生字段，不改变数据库状态枚举：

```json
{
  "effective_status": "active | inactive | error | rate_limited | overloaded | temp_unschedulable | quota_exhausted | paused",
  "effective_status_reason": "codex_5h_exhausted | codex_7d_exhausted | rate_limit | overload | temp_unschedulable | quota_exhausted | manual_paused",
  "effective_rate_limit_reset_at": "2026-05-01T14:54:01+08:00"
}
```

字段命名可在实现前按项目风格微调，但原则是：

- `status` 继续表示管理员配置/基础生命周期
- `effective_status` 表示当前实际可用状态

**Step 2: DTO mapper 复用同一判断**

DTO 中不要重新手写一套复杂逻辑。优先在 service 层提供 helper，让 mapper 调用，例如：

```go
func (a *Account) EffectiveAvailability(now time.Time) AccountAvailabilityState
```

**Step 3: 兼容旧前端**

旧前端即使不读取新增字段，也不应崩溃。新增字段只读、可选，不改变现有响应结构。

**Step 4: 验证**

Run:

```bash
cd backend
go test -tags=unit ./internal/handler ./internal/service -run 'Test.*Account.*Status|Test.*DTO' -count=1
```

Expected: 新字段在 Codex 100% 样本中返回 `rate_limited`，reset 后返回 `active`。

## Task 4: 前端状态展示与筛选对齐

**Files:**

- Modify: `frontend/src/components/account/AccountStatusIndicator.vue`
- Modify: `frontend/src/views/admin/AccountsView.vue`
- Modify: `frontend/src/components/admin/account/AccountTableFilters.vue`
- Modify: `frontend/src/i18n/locales/zh.ts`
- Modify: `frontend/src/i18n/locales/en.ts`
- Modify: `frontend/src/types/index.ts`
- Test: `frontend/src/components/account/__tests__/AccountStatusIndicator.spec.ts`
- Test: `frontend/src/views/admin/__tests__/AccountsView.spec.ts` 或新增 focused test

**Step 1: 状态组件优先使用 `effective_status`**

显示优先级建议：

1. `error`
2. `overloaded`
3. `temp_unschedulable`
4. `rate_limited`
5. `quota_exhausted`
6. `paused`
7. `active/inactive`

Codex 100% 时显示“限流中”或“Codex 限额中”，并展示恢复倒计时。

**Step 2: 本地兜底识别 Codex extra**

在后端新增字段上线前后，为降低版本错配风险，前端可保留轻量兜底：

- OpenAI OAuth
- `codex_5h_used_percent >= 100` 且 reset 在未来
- 或 `codex_7d_used_percent >= 100` 且 reset 在未来

兜底只用于展示与当前页筛选，不承担调度事实来源。

**Step 3: 筛选逻辑对齐**

`AccountsView.vue` 的本地 `accountMatchesCurrentFilters(...)` 需要改成：

- “正常”：实际 active 且无 rate limit / overload / temp unschedulable / paused
- “限流”：普通 `rate_limit_reset_at` 或 Codex 100% reset 在未来
- “不可调度”：手动 `schedulable=false`，不混入限流和 temp unschedulable

**Step 4: i18n 文案**

建议新增：

- `admin.accounts.status.codexRateLimited`
- `admin.accounts.status.codexRateLimitedUntil`
- `admin.accounts.status.effectiveRateLimited`

中文显示可用：

- `Codex 限额中`
- `恢复于 {time}`

**Step 5: 验证**

Run:

```bash
cd frontend
pnpm run typecheck
pnpm test -- AccountStatusIndicator AccountUsageCell accountUsageRefresh AccountsView
```

Expected:

- Codex 100% 样本显示为限流，不显示正常
- 状态筛选“正常”排除 Codex 100% 账号
- 状态筛选“限流”包含 Codex 100% 账号

## Task 5: 远程只读复核与截图审查

**Files:**

- Create: `tmp/screenshots/run-{timestamp}/before.png`
- Create: `tmp/screenshots/run-{timestamp}/after.png`
- Create: `tmp/screenshots/run-{timestamp}/compare.png`

**Step 1: 修复前保存远程证据**

只读命令：

```bash
set -a; source remote.env; set +a
curl -sS -H "x-api-key: ${REMOTE_ADMIN_API_KEY}" \
  "${REMOTE_BASE_URL}/api/v1/admin/accounts?page=1&page_size=100&status=active&sort_by=priority&sort_order=asc"
```

记录：

- active 总数
- Codex 100% 数量
- 仍被标为 normal/available 的数量

**Step 2: 本地截图审查**

按项目 UI 规则，在修复前后同一路由、同一筛选条件截图：

- 路由：`/admin/accounts`
- 筛选：状态 `正常`
- 截图：`before.png` / `after.png` / `compare.png`

**Step 3: 修复后远程复核**

部署或测试站更新后，再执行只读查询：

```bash
set -a; source remote.env; set +a
curl -sS -H "x-api-key: ${REMOTE_ADMIN_API_KEY}" \
  "${REMOTE_BASE_URL}/api/v1/admin/ops/account-availability?platform=openai"
```

Expected:

- Codex 100% 且 reset 未到期账号不再计入 `available_count`
- 账号列表不再把这些账号展示为“正常”
- reset 到期后自动回到正常可用

## 回滚方案

- 新增 DTO 字段是只读扩展，可保留不影响旧客户端。
- 如果前端展示出现异常，可以先回滚前端对 `effective_status` 的使用，服务端仍可继续修正 ops 和调度状态。
- 如果持久化 `rate_limit_reset_at` 同步出现误伤，优先回滚同步调用点，保留 `Account.IsSchedulable()` 的运行时判断。
- 不涉及 schema 迁移，回滚不需要数据库结构变更。

## 审查重点

请重点审查这些决策：

- 是否同意不扩展数据库 `status` 枚举，而是新增 `effective_status` 派生字段。
- 是否同意 Codex 100% 在 UI 中归类为“限流”，而不是新增一个独立筛选项。
- 是否同意 `rate_limit_reset_at` 使用更保守的“取更晚 reset 时间”策略。
- 是否同意本轮只解决 100% 后的状态对齐，不做 70%/85%/95% 的预测性调度降权。

