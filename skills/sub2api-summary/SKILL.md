---
name: sub2api-summary
description: 当用户要求分析某个真实运营中的 sub2api 站点、汇总一段时间的运营数据、诊断用量/成本/账号/模型/分组/用户行为并输出优化建议时使用。必须用于“sub2api 运营复盘”“站点数据分析”“最近 24h 运营建议”“根据线上数据写优化计划”等任务；如果用户没有提供实际站点鉴权信息，应立即中止并要求补充。
metadata:
  author: Bensz Conan
  short-description: 基于真实 sub2api 运营数据生成优化计划
  keywords:
    - sub2api-summary
    - sub2api
    - 运营分析
    - 优化计划
    - 用量分析
---

# sub2api-summary

## 与 bensz-collect-bugs 的协作约定

- 因本 skill 设计缺陷导致的 bug，先用 `bensz-collect-bugs` 规范记录到 `~/.bensz-skills/bugs/`，不要直接修改用户本地已安装的 skill 源码；若有 workaround，先记 bug，再继续完成任务。
- 只有用户明确要求“report bensz skills bugs”等公开上报时，才用本地 `gh` 上传新增 bug 到 `huangwb8/bensz-bugs`；不要 pull / clone 整个仓库。

## 目标

收集真实运营中的 sub2api 站点在指定时间段内的只读运营数据，结合当前 sub2api 源代码理解数据背后的机制，输出可执行的优化计划。

## 输入要求

必须输入：
- 站点基础地址：例如 `https://example.com`
- 管理员只读排查凭据：优先使用 Admin API Key，对应请求头 `x-api-key`；也支持管理员 JWT，对应 `Authorization: Bearer`

可选输入：
- 时间段：用户可给自然语言或明确起止时间；未提供时默认最近 24h
- 时区：未提供时默认 `Asia/Shanghai`

如果缺少站点地址或鉴权信息，立即中止工作，不要尝试猜测、登录、绕过鉴权或读取本地历史凭据。

## 工作目录

每次运行创建：

```text
./tmp/sub2api-summary/run-{时间戳}/
├── data/       # 只读采集到的原始响应与采集元数据
├── analysis/   # 分析脚本输出、中间表和摘要
└── plan.md     # 最终优化计划
```

时间戳使用本地时间 `YYYYMMDDHHMMSS`。
采集脚本默认会拒绝写入 `./tmp/sub2api-summary` 之外的目录；如果调整工作根目录，必须显式传入 `--work-root`。

## 工作流程

### 阶段一：初始化与授权确认

1. 解析用户输入的站点地址、鉴权信息、时间段和时区。
2. 确认鉴权信息来自用户本轮输入或用户明确指定的安全本地来源。
3. 创建工作目录，不把密钥写入 `plan.md`、日志或截图。
4. 若需要使用仓库根目录 `remote.env`，只读取 `REMOTE_BASE_URL` 与 `REMOTE_ADMIN_API_KEY`，且仅在用户明确允许时使用。

### 阶段二：只读采集

优先通过环境变量传入凭据，避免密钥出现在 shell 历史或进程列表：

```bash
export SUB2API_BASE_URL="https://example.com"
export SUB2API_ADMIN_API_KEY="..."
python3 skills/sub2api-summary/scripts/collect_sub2api_data.py \
  --out-dir "./tmp/sub2api-summary/run-{timestamp}/data"
```

使用管理员 JWT 时：

```bash
export SUB2API_BASE_URL="https://example.com"
export SUB2API_AUTH_MODE="bearer"
export SUB2API_AUTH_TOKEN="..."
python3 skills/sub2api-summary/scripts/collect_sub2api_data.py \
  --out-dir "./tmp/sub2api-summary/run-{timestamp}/data"
```

可传入时间段。注意当前 sub2api 管理端汇总接口以日期为主要参数，采集脚本会记录用户请求的 ISO 起止时间，同时用覆盖该时间段的日期桶查询线上接口：

```bash
export SUB2API_BASE_URL="https://example.com"
export SUB2API_ADMIN_API_KEY="..."
python3 skills/sub2api-summary/scripts/collect_sub2api_data.py \
  --since "2026-05-01T00:00:00+08:00" \
  --until "2026-05-02T00:00:00+08:00" \
  --timezone "Asia/Shanghai" \
  --out-dir "./tmp/sub2api-summary/run-{timestamp}/data"
```

只调用 GET 接口。默认采集：
- `/api/v1/admin/dashboard/snapshot-v2`
- `/api/v1/admin/usage/stats`
- `/api/v1/admin/dashboard/models`
- `/api/v1/admin/dashboard/groups`
- `/api/v1/admin/dashboard/users-ranking`
- `/api/v1/admin/dashboard/user-breakdown`
- `/api/v1/admin/dashboard/profitability`
- `/api/v1/admin/dashboard/recommendations`
- `/api/v1/admin/usage`
- `/api/v1/admin/ops/dashboard/snapshot-v2`

部分接口可能因版本或配置不存在；记录失败原因，但不要因为非核心接口失败而停止。鉴权失败、站点不可达、基础用量接口失败时停止并说明。`dashboard/snapshot-v2` 是高价值接口，但旧站点可能不存在；此时继续使用 `usage/stats`、模型、分组和明细样本分析。

### 阶段三：确定性分析

运行：

```bash
python3 skills/sub2api-summary/scripts/analyze_sub2api_data.py \
  --data-dir "./tmp/sub2api-summary/run-{timestamp}/data" \
  --analysis-dir "./tmp/sub2api-summary/run-{timestamp}/analysis" \
  --plan-path "./tmp/sub2api-summary/run-{timestamp}/plan.md"
```

脚本生成初稿后，继续由 AI 结合源码深挖：
- 认证与权限：`backend/internal/server/middleware/admin_auth.go`
- 管理端用量接口：`backend/internal/handler/admin/usage_handler.go`
- Dashboard 统计接口：`backend/internal/handler/admin/dashboard_handler.go`
- UsageLog 数据结构：`backend/ent/schema/usage_log.go`
- 用量查询实现：`backend/internal/repository/usage_log_repo.go`
- 前端接口口径：`frontend/src/api/admin/usage.ts`、`frontend/src/api/admin/dashboard.ts`

不要只复述数据；每个重要发现都要解释它可能对应的产品机制、计费机制、调度机制或运维风险。

### 阶段四：输出优化计划

最终写入 `plan.md`。建议结构：

```markdown
# Sub2API 运营优化计划

## 概览
- 站点：
- 时间段：
- 数据完整性：
- 总体判断：

## 关键发现

## 根因分析

## 优化计划
### P0
### P1
### P2

## 验证方案

## 附录
```

建议必须可执行，包含：
- 证据：引用 `data/` 或 `analysis/` 中的文件名与字段
- 影响：成本、稳定性、用户体验、容量、风控或收入
- 代码关联：指出相关源代码文件或接口口径
- 操作建议：配置调整、产品改造、数据补充、监控告警或后续实验
- 验证方法：后续应看哪些指标改善

## 安全边界

- 只读采集；不调用 POST/PUT/PATCH/DELETE，不触发清理、重置、回填、测试代理、更新配置等写操作。
- 不在输出中展示完整密钥、用户邮箱、API Key、上游账号凭据或代理凭据；必要时只展示脱敏 ID 或聚合统计。
- 下载数据仅保存在本次 `tmp/sub2api-summary/run-*` 目录。
- 发现需要写操作的优化项时，只写入计划，不直接执行。
