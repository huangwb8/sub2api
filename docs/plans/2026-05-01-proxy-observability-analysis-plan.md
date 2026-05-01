# 代理巡检观测与请求关联分析 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 建立代理巡检结果、真实请求成功记录和上游错误之间的可关联数据闭环，用于评估“代理连接失败”提示的可靠性。

**Architecture:** 先补齐请求日志中的 `proxy_id`，再新增轻量级代理巡检历史表，最后提供只读分析能力。巡检历史按短期保留设计，作为运营排障事实层，而不是永久高频监控数据。

**Tech Stack:** Go / Gin / Ent ORM / PostgreSQL / Redis / Vue 3 / TypeScript / pnpm。

**Minimal Change Scope:** 允许修改 `backend/ent/schema/`、`backend/migrations/`、`backend/internal/service/`、`backend/internal/repository/`、`backend/internal/handler/admin/`、`frontend/src/api/`、`frontend/src/views/admin/`、`frontend/src/types/`、`frontend/src/i18n/`、`skills/sub2api-summary/references/source-map.md`。避免改动代理调度策略、账号选择算法、计费口径和支付逻辑。

**Success Criteria:** 近 7-14 天内可以回答“某代理巡检失败后，后续真实请求成功率是多少”；OpenAI OAuth 成功 usage 能记录当时使用的 `proxy_id`；代理巡检历史有保留策略和基础查询接口；现有代理自动迁移行为不被改变。

**Verification Plan:** 运行 `cd backend && go generate ./ent && go test -tags=unit ./...`；运行 `cd frontend && pnpm run typecheck && pnpm run lint:check`；用远程只读接口验证分析字段不泄露代理密码、账号凭据或真实 API Key。

---

## 背景

当前 IP 管理页面的代理“连接失败”来自 Redis 中的代理延迟/探测缓存。这个设计适合实时展示和迁移判断，但不适合事后分析：

- 无法回放某个代理在过去一周每次巡检的成功/失败状态。
- 无法判断 `ip-api.com` / `httpbin.org` 探测失败是否真实预示 ChatGPT 请求失败。
- 当前远程站点的成功 usage 中 `proxy_id` 返回为 `null`，导致成功请求无法精确关联当时使用的代理。
- 错误日志能看到账号和错误消息，但缺少稳定的代理维度分析入口。

本计划目标不是新增完整监控系统，而是补齐最小的数据事实层，让后续运营判断有据可查。

## 非目标

- 不改变代理自动迁移、半开试探、冷却退避等现有行为。
- 不把巡检日志做成永久全量审计数据。
- 不记录代理密码、完整代理 URL、账号 token、用户请求正文等敏感数据。
- 不在第一阶段做复杂可视化大盘。

## 关键设计

### 数据闭环

需要形成三类数据的时间关联：

| 数据 | 当前状态 | 目标状态 |
|------|----------|----------|
| 代理巡检结果 | 仅 Redis 当前状态 | PostgreSQL 短期历史 + Redis 当前状态 |
| 成功请求 | usage 有账号但 `proxy_id` 不完整 | usage 记录当时账号绑定的 `proxy_id` |
| 失败请求 | ops error 有账号和消息 | 尽量补充 `proxy_id` 或支持通过账号时间线关联 |

### 历史表建议

新增 `proxy_probe_logs`：

| 字段 | 说明 |
|------|------|
| `id` | 主键 |
| `proxy_id` | 被探测代理 |
| `source` | `scheduled_probe` / `manual_test` / `failover_target_check` |
| `target` | `ip-api` / `httpbin` / 未来的 `chatgpt` 等 |
| `success` | 探测是否成功 |
| `latency_ms` | 成功时延迟 |
| `error_message` | 失败原因，截断保存 |
| `ip_address` | 出口 IP，可选 |
| `country_code` / `country` / `region` / `city` | 出口地理信息 |
| `checked_at` | 探测时间 |
| `created_at` | 入库时间 |

索引：

- `(proxy_id, checked_at DESC)`
- `(success, checked_at DESC)`
- `(source, checked_at DESC)`

保留策略：

- 默认保留 `14` 天。
- 后端清理任务每日或定时删除过期记录。
- 可通过配置调整到 `7-30` 天，不建议无限保留。

## 实施任务

### Task 1: 补齐 usage 日志中的 `proxy_id`

**Files:**

- Modify: `backend/internal/service/gateway_record_usage.go`
- Modify: `backend/internal/service/openai_ws_forwarder.go`
- Modify: `backend/internal/service/openai_ws_v2_passthrough_adapter.go`
- Test: `backend/internal/service/gateway_record_usage_test.go`

**Steps:**

1. 写单元测试：当 `Account.ProxyID` 非空时，成功 usage log 必须记录 `proxy_id`。
2. 检查所有高频 OpenAI 成功记录路径是否传入了账号代理信息。
3. 修复遗漏路径，确保不依赖后续账号迁移后的当前代理，而是记录请求发生时账号上的 `ProxyID`。
4. 运行相关测试，确认不会影响无代理账号和非住宅代理估算逻辑。

**Risk:** 中等。该字段影响后续成本分析和代理流量分析，但不应改变计费金额。

### Task 2: 设计并生成代理巡检历史 Schema

**Files:**

- Create: `backend/ent/schema/proxy_probe_log.go`
- Create: `backend/migrations/XXX_add_proxy_probe_logs.sql`
- Modify: `backend/ent/` generated files

**Steps:**

1. 新增 Ent schema，字段按“历史表建议”实现。
2. 新增 PostgreSQL 迁移，包含索引。
3. 执行 `cd backend && go generate ./ent`。
4. 检查生成文件和迁移编号。

**Risk:** 中等。新增表低风险，但需要确保迁移幂等、索引不会过重。

### Task 3: 写入巡检历史

**Files:**

- Modify: `backend/internal/service/proxy_failover_service.go`
- Modify: `backend/internal/service/admin_service.go`
- Create/Modify: `backend/internal/repository/proxy_probe_log_repo.go`
- Modify: `backend/internal/service/wire.go`
- Modify: `backend/cmd/server/wire_gen.go`
- Test: `backend/internal/service/proxy_failover_service_test.go`

**Steps:**

1. 新增 repository/service 接口：`RecordProxyProbeLog(ctx, input)`。
2. 在自动巡检 `runSingleProxyProbe` 成功和失败路径写入日志。
3. 在手动测试 `TestProxy` 路径写入 `manual_test` 日志。
4. 错误消息截断，例如 512 或 1024 字符。
5. 不保存 username/password/full URL。
6. 写测试覆盖成功、失败、日志仓储不可用时不影响主流程。

**Risk:** 中等。写日志必须是旁路，失败不能阻断巡检或请求。

### Task 4: 添加保留期清理

**Files:**

- Modify: `backend/internal/service/usage_cleanup_service.go` 或新增轻量 maintenance service
- Modify: `backend/internal/config/config.go`
- Test: 对应 service 测试

**Steps:**

1. 新增配置：`proxy_probe_logs.retention_days`，默认 `14`。
2. 新增清理逻辑：删除 `checked_at < now - retention_days` 的记录。
3. 复用现有定时维护模式，避免新增复杂 worker。
4. 写测试覆盖默认值、禁用/异常配置、删除边界。

**Risk:** 低到中。重点是不要误删其它表。

### Task 5: 提供只读分析接口

**Files:**

- Modify: `backend/internal/handler/admin/proxy_handler.go`
- Modify: `backend/internal/server/routes/admin.go`
- Modify: `backend/internal/service/admin_service.go`
- Modify: `frontend/src/api/admin/proxies.ts`
- Modify: `frontend/src/types/index.ts`

**Suggested endpoints:**

- `GET /api/v1/admin/proxies/:id/probe-logs`
- `GET /api/v1/admin/proxies/:id/reliability`

**Reliability output:**

- 近 24h / 7d 巡检成功率
- 最近失败时间
- 失败后 15/30/60 分钟内 usage 成功数和错误数
- 代理相关错误数
- 当前是否被账号绑定

**Steps:**

1. 先做后端只读接口。
2. 接口默认限制时间范围和分页大小。
3. 不返回敏感字段。
4. 添加单元测试或 handler 测试。

**Risk:** 中等。分析口径要明确，避免让用户误以为是绝对因果。

### Task 6: 管理后台最小展示

**Files:**

- Modify: `frontend/src/views/admin/ProxiesView.vue`
- Modify: `frontend/src/i18n/locales/zh.ts`
- Modify: `frontend/src/i18n/locales/en.ts`

**Steps:**

1. 在代理质量/详情弹窗中增加“巡检历史”或“可靠性”入口。
2. 展示最近巡检结果、失败原因和 7 天成功率。
3. 加一句短文案说明：巡检失败是风险信号，不等同于真实请求必失败。
4. 保持表格列表页轻量，不在主表渲染大量历史数据。
5. 前端验证：`cd frontend && pnpm run typecheck && pnpm run lint:check`。

**Risk:** 低。注意不要让页面变重。

### Task 7: 同步辅助 skill 地图

**Files:**

- Modify: `skills/sub2api-summary/references/source-map.md`
- Optional: `skills/sub2api-summary/SKILL.md`

**Steps:**

1. 更新 source map，记录 usage `proxy_id`、`proxy_probe_logs`、proxy reliability endpoint 的来源。
2. 如果 skill 分析逻辑依赖运营健康判断，补充“代理巡检历史”的解释。
3. 不改脚本，除非脚本实际读取这些新增接口。

**Risk:** 低。属于文档/skill 对齐。

## 验证矩阵

| 场景 | 期望 |
|------|------|
| 有代理账号成功请求 | usage 记录当时 `proxy_id` |
| 无代理账号成功请求 | usage `proxy_id` 仍为空 |
| 自动巡检成功 | 写入 `proxy_probe_logs.success=true` |
| 自动巡检失败 | 写入失败原因，且不阻断迁移逻辑 |
| 手动测试代理 | 写入 `source=manual_test` |
| 日志仓储失败 | 巡检/请求主流程继续 |
| 清理任务运行 | 只删除超过保留期的巡检日志 |
| 管理端查询 | 不返回代理密码和账号凭据 |

## 回滚方案

- 新表是旁路数据，若出现性能或写入问题，可先关闭写入或跳过 repository 错误。
- 保留 usage `proxy_id` 修复，因为它是事实补全，不改变业务行为。
- 前端可靠性视图可独立回滚，不影响后端调度。
- 数据库回滚时可保留空表，避免破坏已有迁移历史；必要时新增后续迁移禁用相关查询。

## 执行顺序建议

1. 先做 Task 1，补齐成功请求事实。
2. 再做 Task 2-4，建立短期巡检历史。
3. 最后做 Task 5-6，把分析结果暴露给管理员。
4. Task 7 与源码变更同轮完成，避免 `sub2api-summary` 依赖过时源码地图。

## 预计收益

- 可以量化“连接失败”提示的误报率和预测价值。
- 可以按供应商 session、地区、代理 ID 找到短命代理。
- 调整巡检间隔、失败阈值、冷却时间后有数据验证效果。
- 为后续“自动刷新 session id”或“代理质量分层调度”提供依据。
