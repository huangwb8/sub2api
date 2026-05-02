# sub2api-summary：用户使用指南

本 README 面向**使用者**：如何触发并正确使用 `sub2api-summary` skill。
执行指令与硬性规范在 `SKILL.md`；默认参数在 `config.yaml`。

`sub2api-summary` 用来分析真实运营中的 sub2api 站点。它会只读采集管理端运营数据，结合当前项目源码理解数据口径，最后生成一份可执行的 `plan.md` 优化计划。

## 快速开始

### 推荐用法：让 Agent 自动完成采集与分析

```text
请使用 sub2api-summary skill 分析这个 sub2api 站点最近 24 小时的运营数据，并输出优化计划。
输入：
- 站点地址：https://example.com
- 管理员只读 API Key：sk-xxxx
输出：写入 ./tmp/sub2api-summary/run-{时间戳}/plan.md
```

### 进阶用法：指定时间段与时区

```text
请使用 sub2api-summary skill 分析这个 sub2api 站点指定时间段的运营数据，并输出优化计划。
输入：
- 站点地址：https://example.com
- 管理员只读 API Key：sk-xxxx
- 时间段：2026-05-01 00:00 到 2026-05-02 00:00
- 时区：Asia/Shanghai
输出：写入 ./tmp/sub2api-summary/run-{时间戳}/plan.md
另外，还有下列参数约束：
- 只允许只读查询，不执行任何写操作
- 不在报告中展示完整密钥、用户邮箱或 API Key
```

## 适合什么场景

| 你的需求 | 推荐使用方式 | 结果 |
|----------|--------------|------|
| 想看最近 24 小时站点有没有异常 | 直接给站点地址和管理员 API Key | 得到一份包含关键发现、根因分析、P0/P1/P2 建议的 `plan.md` |
| 想复盘某次成本或耗时波动 | 额外给明确起止时间 | 按指定窗口采集用量、模型、分组、用户排行等数据 |
| 想排查账号池、模型、分组或用户集中度问题 | 给真实站点凭据并说明关注点 | 报告会把数据现象和源码口径关联起来 |
| 想先自己跑脚本，再让 Agent 解读 | 使用 README 末尾的脚本备选用法 | 得到 `data/`、`analysis/` 和 `plan.md` 初稿 |

不适合的情况：

- 没有真实站点地址或管理员凭据。
- 希望自动修改线上配置、清理数据、重置账号或执行其它写操作。
- 只想根据截图、口头描述或猜测生成运营结论。

## 它会做什么

这个 skill 的工作分为四步：

1. 确认输入：检查站点地址、鉴权方式、时间段和时区。
2. 只读采集：调用管理端 GET 接口，保存原始 JSON 到本次运行目录。
3. 确定性分析：用脚本生成指标摘要和优化计划初稿。
4. AI 深挖：结合 `references/source-map.md` 指向的源码位置，解释数据背后的产品机制、计费机制、调度机制和运维风险。

它不会替你执行线上写操作。发现需要调整配置、扩容账号、改定价或加监控时，只会写进计划，等待管理员确认后再执行。

## 输入要求

| 输入 | 是否必需 | 说明 |
|------|----------|------|
| 站点基础地址 | 必需 | 例如 `https://example.com`。脚本会自动补齐 `/api/v1`。 |
| 管理员只读 API Key | 二选一必需 | 推荐方式，对应请求头 `x-api-key`。 |
| 管理员 JWT | 二选一必需 | 备选方式，对应请求头 `Authorization: Bearer`。 |
| 时间段 | 可选 | 未提供时默认最近 24 小时。 |
| 时区 | 可选 | 未提供时默认 `Asia/Shanghai`。 |

如果缺少站点地址或鉴权信息，skill 会中止，不会猜测、登录、绕过鉴权或读取历史凭据。

## 输出文件

每次运行会创建一个独立目录：

```text
./tmp/sub2api-summary/run-{时间戳}/
├── data/       # 原始只读响应、采集元数据和接口错误记录
├── analysis/   # 指标摘要等中间分析结果
└── plan.md     # 最终运营优化计划
```

常见文件：

| 文件 | 用途 |
|------|------|
| `data/_metadata.json` | 本次采集的站点、时间段、时区、鉴权模式和只读标记 |
| `data/_errors.json` | 非核心接口失败原因；旧站点缺少部分接口时会记录在这里 |
| `data/usage_stats.json` | 核心用量统计，采集失败时会中止 |
| `data/dashboard_snapshot_v2.json` | 高价值 Dashboard 快照；旧站点可能不存在 |
| `analysis/summary.json` | 确定性脚本输出的指标、阈值和发现 |
| `plan.md` | 最终给管理员看的优化计划 |

## 配置选项

默认配置来自 `config.yaml`：

| 配置 | 默认值 | 说明 |
|------|--------|------|
| `directories.work_root` | `tmp/sub2api-summary` | 运行产物根目录 |
| `defaults.timezone` | `Asia/Shanghai` | 默认时区 |
| `defaults.lookback_hours` | `24` | 未指定时间段时回看小时数 |
| `defaults.page_size` | `200` | 用量明细采样页大小 |
| `defaults.user_limit` | `50` | 用户排行、用户拆解默认数量 |
| `defaults.request_timeout_seconds` | `30` | 单个接口请求超时 |
| `analysis_thresholds.high_average_duration_ms` | `8000` | 平均耗时偏高阈值 |
| `analysis_thresholds.high_top_model_share` | `0.6` | Top 模型成本集中度阈值 |
| `analysis_thresholds.high_top_user_share` | `0.5` | Top 用户成本集中度阈值 |
| `analysis_thresholds.high_unhealthy_account_ratio` | `0.2` | 异常账号比例阈值 |

通常你不需要手动修改配置；直接在 prompt 里说明时间段和关注点即可。

## 使用示例

### 示例：最近 24 小时运营复盘

```text
请使用 sub2api-summary skill 做一次最近 24 小时运营复盘。
输入：
- 站点地址：https://example.com
- 管理员只读 API Key：sk-xxxx
输出：生成 ./tmp/sub2api-summary/run-{时间戳}/plan.md
```

### 示例：排查模型成本集中

```text
请使用 sub2api-summary skill 分析模型成本是否过度集中，并给出优化计划。
输入：
- 站点地址：https://example.com
- 管理员只读 API Key：sk-xxxx
- 时间段：最近 7 天
输出：生成 ./tmp/sub2api-summary/run-{时间戳}/plan.md，重点说明 Top 模型成本占比、影响和验证方案。
```

### 示例：使用管理员 JWT

```text
请使用 sub2api-summary skill 分析这个站点今天的运营数据。
输入：
- 站点地址：https://example.com
- 管理员 JWT：eyJ...
- 时区：Asia/Shanghai
输出：生成 ./tmp/sub2api-summary/run-{时间戳}/plan.md
```

### 示例：允许读取本地 remote.env

```text
请使用 sub2api-summary skill 分析 remote.env 指向的远程测试站点，只做只读查询。
输入：
- 凭据来源：允许读取项目根目录 remote.env 中的 REMOTE_BASE_URL 和 REMOTE_ADMIN_API_KEY
- 时间段：最近 24 小时
输出：生成 ./tmp/sub2api-summary/run-{时间戳}/plan.md
```

## 安全边界

- 只调用 GET 接口。
- 不执行 POST、PUT、PATCH、DELETE。
- 不触发清理、重置、回填、测试代理、更新配置等写操作。
- 不在 `plan.md` 里展示完整密钥、用户邮箱、API Key、上游账号凭据或代理凭据。
- 默认只把数据保存到本次 `tmp/sub2api-summary/run-*` 目录。
- 需要线上写操作的建议只进入计划，不直接执行。

## 常见问题

### Q：为什么必须提供真实站点凭据？

A：这个 skill 的目标是做运营分析，不是凭空推测。没有真实站点数据时，成本、模型、分组、用户和账号池结论都不可靠。

### Q：Admin API Key 和 JWT 用哪个更好？

A：优先使用 Admin API Key。它会通过 `x-api-key` 请求头传入，适合只读排查。JWT 也支持，但通常更适合临时人工排查。

### Q：旧站点缺少 `dashboard/snapshot-v2` 会失败吗？

A：不会直接失败。`dashboard_snapshot_v2.json` 是高价值数据，但旧站点可能没有这个接口。只要核心 `usage_stats.json` 能采集成功，skill 会继续分析，并在 `data/_errors.json` 记录缺失接口。

### Q：报告里的建议会自动修改线上站点吗？

A：不会。报告只写建议和验证方案。任何配置调整、账号处理、价格修改或监控改造都需要管理员另行确认。

### Q：我能把密钥写在命令行参数里吗？

A：脚本支持命令行参数，但推荐通过环境变量传入，避免密钥出现在 shell 历史或进程列表里。

## 备选用法：脚本流程

如果你想自己先采集和分析，再让 Agent 解读，可以直接运行脚本。

### 使用 Admin API Key 采集

```bash
# 建议用环境变量传入凭据，避免密钥出现在命令历史里
export SUB2API_BASE_URL="https://example.com"
export SUB2API_ADMIN_API_KEY="sk-xxxx"

# 采集最近 24 小时的只读运营数据
python3 skills/sub2api-summary/scripts/collect_sub2api_data.py \
  --out-dir "./tmp/sub2api-summary/run-20260502230000/data"
```

### 指定时间段采集

```bash
# 设置站点与只读管理员凭据
export SUB2API_BASE_URL="https://example.com"
export SUB2API_ADMIN_API_KEY="sk-xxxx"

# 采集指定 ISO 时间段；接口会按覆盖该时间段的日期桶查询
python3 skills/sub2api-summary/scripts/collect_sub2api_data.py \
  --since "2026-05-01T00:00:00+08:00" \
  --until "2026-05-02T00:00:00+08:00" \
  --timezone "Asia/Shanghai" \
  --out-dir "./tmp/sub2api-summary/run-20260502230000/data"
```

### 使用管理员 JWT 采集

```bash
# JWT 模式会使用 Authorization: Bearer 请求头
export SUB2API_BASE_URL="https://example.com"
export SUB2API_AUTH_MODE="bearer"
export SUB2API_AUTH_TOKEN="eyJ..."

# 采集只读运营数据
python3 skills/sub2api-summary/scripts/collect_sub2api_data.py \
  --out-dir "./tmp/sub2api-summary/run-20260502230000/data"
```

### 生成分析初稿

```bash
# 根据 data/ 目录生成 analysis/summary.json 和 plan.md 初稿
python3 skills/sub2api-summary/scripts/analyze_sub2api_data.py \
  --data-dir "./tmp/sub2api-summary/run-20260502230000/data" \
  --analysis-dir "./tmp/sub2api-summary/run-20260502230000/analysis" \
  --plan-path "./tmp/sub2api-summary/run-20260502230000/plan.md"
```

脚本生成的是确定性初稿。最终交付时，仍建议让 Agent 结合 `references/source-map.md` 里的源码核对地图继续解释根因和优化路径。
