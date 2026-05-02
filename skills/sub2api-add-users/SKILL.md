---
name: sub2api-add-users
description: 当用户要求评估真实运营中的 sub2api 站点当前算力是否有冗余、适合新增多少用户、用户增长容量、加用户建议、容量缺口或“还能卖多少用户”时必须使用。必须基于用户提供的真实站点鉴权信息只读采集运营数据；如果用户没有提供站点地址和管理员鉴权信息，应立即中止并要求补充。
metadata:
  author: Bensz Conan
  short-description: 基于真实 sub2api 运营数据评估可新增用户数
  keywords:
    - sub2api-add-users
    - sub2api
    - 加用户建议
    - 容量分析
    - 算力冗余
    - 用户增长
---

# sub2api-add-users

## 与 bensz-collect-bugs 的协作约定

- 因本 skill 设计缺陷导致的 bug，先用 `bensz-collect-bugs` 规范记录到 `~/.bensz-skills/bugs/`，不要直接修改用户本地已安装的 skill 源码；若有 workaround，先记 bug，再继续完成任务。
- 只有用户明确要求“report bensz skills bugs”等公开上报时，才用本地 `gh` 上传新增 bug 到 `huangwb8/bensz-bugs`；不要 pull / clone 整个仓库。

## 目标

收集真实运营中的 sub2api 站点在指定时间段内的只读运营数据，结合当前 sub2api 源代码理解容量推荐口径，输出管理员可执行的“适合新增多少用户”报告。

输出的用户数可以为：
- 正值：当前容量有冗余，建议最多新增这么多同类订阅用户。
- 零：当前不建议新增用户，继续观察或先补数据。
- 负值：当前已经有算力缺口，绝对值表示按既有用户画像折算出的用户容量缺口。

## 输入要求

必须输入：
- 站点基础地址：例如 `https://example.com`
- 管理员只读排查凭据：优先使用 Admin API Key，对应请求头 `x-api-key`；也支持管理员 JWT，对应 `Authorization: Bearer`

可选输入：
- 时间段：用户可给自然语言或明确起止时间；未提供时默认最近 7d
- 时区：未提供时默认 `Asia/Shanghai`

如果缺少站点地址或鉴权信息，立即中止工作，不要尝试猜测、登录、绕过鉴权或读取本地历史凭据。只有用户明确允许时，才可读取仓库根目录 `remote.env` 中的 `REMOTE_BASE_URL` 与 `REMOTE_ADMIN_API_KEY`。

## 工作目录

每次运行创建：

```text
./tmp/sub2api-add-users/run-{时间戳}/
├── data/       # 只读采集到的原始响应与采集元数据
├── analysis/   # 分析脚本输出、中间表和摘要
└── report.md   # 最终加用户容量建议报告
```

时间戳使用本地时间 `YYYYMMDDHHMMSS`。
采集脚本默认会拒绝写入 `./tmp/sub2api-add-users` 之外的目录；如果调整工作根目录，必须显式传入 `--work-root`。

## 工作流程

### 阶段一：初始化与授权确认

1. 解析用户输入的站点地址、鉴权信息、时间段和时区。
2. 确认鉴权信息来自用户本轮输入或用户明确指定的安全本地来源。
3. 创建工作目录，不把密钥写入 `report.md`、日志或截图。
4. 若缺少必要输入，立即中止并告诉用户需要补充什么。

### 阶段二：只读采集

优先通过环境变量传入凭据，避免密钥出现在 shell 历史或进程列表：

```bash
export SUB2API_BASE_URL="https://example.com"
export SUB2API_ADMIN_API_KEY="..."
python3 skills/sub2api-add-users/scripts/collect_sub2api_add_users_data.py \
  --out-dir "./tmp/sub2api-add-users/run-{timestamp}/data"
```

使用管理员 JWT 时：

```bash
export SUB2API_BASE_URL="https://example.com"
export SUB2API_AUTH_MODE="bearer"
export SUB2API_AUTH_TOKEN="..."
python3 skills/sub2api-add-users/scripts/collect_sub2api_add_users_data.py \
  --out-dir "./tmp/sub2api-add-users/run-{timestamp}/data"
```

传入明确时间段：

```bash
python3 skills/sub2api-add-users/scripts/collect_sub2api_add_users_data.py \
  --since "2026-04-25T00:00:00+08:00" \
  --until "2026-05-02T00:00:00+08:00" \
  --timezone "Asia/Shanghai" \
  --out-dir "./tmp/sub2api-add-users/run-{timestamp}/data"
```

只调用 GET 接口。默认采集必要的容量与聚合用量数据：
- `/api/v1/admin/dashboard/recommendations`
- `/api/v1/admin/usage/stats`
- `/api/v1/admin/groups/capacity-summary`
- `/api/v1/admin/dashboard/snapshot-v2`
- `/api/v1/admin/dashboard/groups`
- `/api/v1/admin/dashboard/models`

`recommendations.json` 与 `usage_stats.json` 是核心数据；失败时停止并说明。其它只读接口失败时记录到 `data/_errors.json`，不要因为非核心接口缺失而停止。

### 阶段三：确定性分析

运行：

```bash
python3 skills/sub2api-add-users/scripts/analyze_sub2api_add_users.py \
  --data-dir "./tmp/sub2api-add-users/run-{timestamp}/data" \
  --analysis-dir "./tmp/sub2api-add-users/run-{timestamp}/analysis" \
  --report-path "./tmp/sub2api-add-users/run-{timestamp}/report.md"
```

脚本生成初稿后，继续由 AI 结合源码深挖：
- 容量推荐服务：`backend/internal/service/dashboard_recommendation_service.go`
- 容量池聚合：`backend/internal/service/dashboard_recommendation_pool.go`
- 管理端 Dashboard 接口：`backend/internal/handler/admin/dashboard_handler.go`
- 分组容量接口：`backend/internal/handler/admin/group_handler.go`
- 用量接口：`backend/internal/handler/admin/usage_handler.go`
- 用量日志结构：`backend/ent/schema/usage_log.go`
- 前端类型：`frontend/src/api/admin/dashboard.ts`、`frontend/src/api/admin/groups.ts`

不要只复述脚本结果；最终报告必须解释为什么这些数据能支持“新增用户/存在缺口”的判断。

### 阶段四：输出报告

最终写入 `report.md`。建议结构：

```markdown
# Sub2API 加用户容量建议报告

## 结论
- 建议新增用户数：
- 判断：
- 时间段：
- 数据完整性：

## 计算口径

## 分容量池建议

## 原因

## 风险与限制

## 后续验证

## 附录
```

报告必须包含：
- 总建议用户数：正值、零或负值。
- 分容量池建议：平台、分组、当前可调度账号、估算所需账号、安全保留、可新增用户或用户缺口。
- 证据：引用 `data/` 或 `analysis/` 中的文件名与字段。
- 源码关联：说明 `dashboard/recommendations` 的推荐口径与可调度账号定义。
- 容量池安全口径：只要任一容量池存在缺口，总结论优先呈现缺口，不用其它容量池的冗余抵消。
- 保守性说明：低置信度、高利用率、核心接口缺失或数据时间窗不足时，不要给出激进扩张建议。

## 安全边界

- 只读采集；不调用 POST/PUT/PATCH/DELETE，不触发清理、重置、回填、测试代理、更新配置等写操作。
- 不在输出中展示完整密钥、用户邮箱、API Key、上游账号凭据或代理凭据；必要时只展示脱敏 ID 或聚合统计。
- 下载数据仅保存在本次 `tmp/sub2api-add-users/run-*` 目录。
- 发现需要补账号、恢复账号或调整配置时，只写入报告建议，不直接执行。
