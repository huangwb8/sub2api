# sub2api-add-users：用户使用指南

本 README 面向**使用者**：如何触发并正确使用 `sub2api-add-users` skill。
执行指令与硬性规范在 `SKILL.md`；默认参数在 `config.yaml`。

`sub2api-add-users` 用来回答一个很具体的问题：一个真实运营中的 sub2api 站点，现在还适合新增多少同类订阅用户？它会只读采集容量推荐、用量统计和分组容量数据，结合源码口径，把账号池冗余或缺口换算成管理员能执行的加用户建议。

## 快速开始

### 推荐用法：让 Agent 自动完成容量分析

```text
请使用 sub2api-add-users skill 评估这个 sub2api 站点当前适合新增多少用户。
输入：
- 站点地址：https://example.com
- 管理员只读 API Key：sk-xxxx
输出：写入 ./tmp/sub2api-add-users/run-{时间戳}/report.md
```

### 进阶用法：指定时间段与关注点

```text
请使用 sub2api-add-users skill 评估这个 sub2api 站点还能新增多少同类订阅用户。
输入：
- 站点地址：https://example.com
- 管理员只读 API Key：sk-xxxx
- 时间段：2026-04-25 00:00 到 2026-05-02 00:00
- 时区：Asia/Shanghai
输出：写入 ./tmp/sub2api-add-users/run-{时间戳}/report.md
另外，还有下列参数约束：
- 只允许只读查询，不执行任何写操作
- 重点说明每个容量池的缺口或冗余
- 不用其它容量池的冗余抵消已有缺口
```

## 适合什么场景

| 你的需求 | 推荐使用方式 | 结果 |
|----------|--------------|------|
| 想知道今天还能不能继续卖用户 | 直接给站点地址和管理员 API Key | 得到建议新增用户数和判断依据 |
| 想评估一周运营后还有多少容量 | 指定最近 7 天或更明确的时间窗 | 得到按容量池拆分的新增建议 |
| 想确认是否已经有算力缺口 | 给真实站点凭据并说明关注缺口 | 报告会优先呈现负值和缺口容量池 |
| 想自己先跑脚本再交给 Agent 解读 | 使用 README 末尾的脚本备选用法 | 得到 `data/`、`analysis/` 和 `report.md` 初稿 |

不适合的情况：

- 没有真实站点地址或管理员凭据。
- 只想靠截图、口头估计或历史印象判断还能卖多少用户。
- 希望 skill 自动新增账号、启停账号、改配置或执行其它线上写操作。

## 结果怎么读

最终报告会给出一个总建议用户数：

| 结果 | 含义 | 管理员动作 |
|------|------|------------|
| 正值，例如 `+12` | 当前容量有冗余，建议最多新增这么多同类订阅用户 | 仍建议按容量池小批量新增，新增后复跑验证 |
| `0` | 当前不建议新增用户 | 继续观察，或补齐数据后再评估 |
| 负值，例如 `-8` | 当前已有容量缺口，绝对值表示折算出的用户缺口 | 先恢复不可调度账号或补充账号，再考虑新增用户 |

关键原则：不同平台、不同容量池不能默认互相支援。只要任一容量池存在缺口，总结论会优先呈现缺口，不会用其它池的冗余把它抵消掉。

## 它会做什么

这个 skill 的工作分为四步：

1. 确认输入：检查站点地址、鉴权方式、时间段和时区。
2. 只读采集：调用管理端 GET 接口，保存容量推荐、用量统计、分组容量和 Dashboard 聚合数据。
3. 确定性分析：把容量池里的可调度账号缺口或冗余换算为用户数。
4. AI 复核：结合 `references/source-map.md` 指向的源码位置，解释推荐口径、可调度账号定义和风险限制。

它不会直接修改线上站点。发现需要补账号、恢复账号或调整配置时，只会写进 `report.md`。

## 输入要求

| 输入 | 是否必需 | 说明 |
|------|----------|------|
| 站点基础地址 | 必需 | 例如 `https://example.com`。脚本会自动补齐 `/api/v1`。 |
| 管理员只读 API Key | 二选一必需 | 推荐方式，对应请求头 `x-api-key`。 |
| 管理员 JWT | 二选一必需 | 备选方式，对应请求头 `Authorization: Bearer`。 |
| 时间段 | 可选 | 未提供时默认最近 7 天。 |
| 时区 | 可选 | 未提供时默认 `Asia/Shanghai`。 |

如果缺少站点地址或鉴权信息，skill 会中止，不会猜测、登录、绕过鉴权或读取历史凭据。只有你明确允许时，才会读取项目根目录 `remote.env` 中的 `REMOTE_BASE_URL` 和 `REMOTE_ADMIN_API_KEY`。

## 输出文件

每次运行会创建一个独立目录：

```text
./tmp/sub2api-add-users/run-{时间戳}/
├── data/       # 原始只读响应、采集元数据和接口错误记录
├── analysis/   # 指标摘要、容量池 CSV 等中间结果
└── report.md   # 最终加用户容量建议报告
```

常见文件：

| 文件 | 用途 |
|------|------|
| `data/_metadata.json` | 本次采集的站点、时间段、时区、鉴权模式和只读标记 |
| `data/_errors.json` | 非核心接口失败原因 |
| `data/recommendations.json` | 核心容量推荐数据，失败时会中止 |
| `data/usage_stats.json` | 核心用量统计数据，失败时会中止 |
| `data/capacity_summary.json` | 分组实时容量摘要，缺失时记录错误但不直接中止 |
| `analysis/summary.json` | 分析脚本输出的总建议、容量池结果和阈值 |
| `analysis/pool_capacity.csv` | 容量池明细表，便于人工复核 |
| `report.md` | 最终给管理员看的加用户容量建议报告 |

## 配置选项

默认配置来自 `config.yaml`：

| 配置 | 默认值 | 说明 |
|------|--------|------|
| `directories.work_root` | `tmp/sub2api-add-users` | 运行产物根目录 |
| `defaults.timezone` | `Asia/Shanghai` | 默认时区 |
| `defaults.lookback_days` | `7` | 未指定时间段时回看天数 |
| `defaults.user_limit` | `80` | Dashboard 用户趋势采集数量 |
| `defaults.request_timeout_seconds` | `30` | 单个接口请求超时 |
| `analysis_thresholds.reserve_account_ratio` | `0.15` | 按可调度账号比例保留的安全余量 |
| `analysis_thresholds.min_account_reserve` | `1` | 每个容量池最少保留账号数 |
| `analysis_thresholds.max_redundant_utilization` | `0.75` | 超过后不再把容量视为可新增冗余 |
| `analysis_thresholds.high_utilization` | `0.85` | 高利用率阈值，会提高安全保留 |
| `analysis_thresholds.low_confidence_score` | `0.45` | 低置信度阈值，会按 50% 折减 |
| `analysis_thresholds.medium_confidence_score` | `0.70` | 中等置信度阈值，会按 75% 折减 |

通常你不需要手动修改配置；直接在 prompt 里说明时间段和业务关注点即可。

## 使用示例

### 示例：判断还能新增多少用户

```text
请使用 sub2api-add-users skill 判断这个站点现在还能新增多少同类订阅用户。
输入：
- 站点地址：https://example.com
- 管理员只读 API Key：sk-xxxx
输出：生成 ./tmp/sub2api-add-users/run-{时间戳}/report.md
```

### 示例：排查容量缺口

```text
请使用 sub2api-add-users skill 判断这个站点是否已经存在用户容量缺口。
输入：
- 站点地址：https://example.com
- 管理员只读 API Key：sk-xxxx
- 时间段：最近 7 天
输出：生成 ./tmp/sub2api-add-users/run-{时间戳}/report.md，重点列出负值容量池、缺口用户数和补账号建议。
```

### 示例：使用管理员 JWT

```text
请使用 sub2api-add-users skill 评估这个站点的加用户容量。
输入：
- 站点地址：https://example.com
- 管理员 JWT：eyJ...
- 时区：Asia/Shanghai
输出：生成 ./tmp/sub2api-add-users/run-{时间戳}/report.md
```

### 示例：允许读取本地 remote.env

```text
请使用 sub2api-add-users skill 分析 remote.env 指向的远程测试站点，只做只读查询。
输入：
- 凭据来源：允许读取项目根目录 remote.env 中的 REMOTE_BASE_URL 和 REMOTE_ADMIN_API_KEY
- 时间段：最近 7 天
输出：生成 ./tmp/sub2api-add-users/run-{时间戳}/report.md
```

## 安全边界

- 只调用 GET 接口。
- 不执行 POST、PUT、PATCH、DELETE。
- 不触发清理、重置、回填、测试代理、更新配置等写操作。
- 不在 `report.md` 里展示完整密钥、用户邮箱、API Key、上游账号凭据或代理凭据。
- 默认只把数据保存到本次 `tmp/sub2api-add-users/run-*` 目录。
- 需要补账号、恢复账号或调整配置的建议只进入报告，不直接执行。

## 常见问题

### Q：为什么默认看最近 7 天？

A：加用户建议更关心稳定容量和近期高峰，7 天比 24 小时更能覆盖日常波动。你也可以在 prompt 里指定其它时间段。

### Q：为什么有的容量池为正，总结论还是负数？

A：因为不同容量池不能默认互相支援。一个池的账号冗余不一定能承接另一个平台或分组的用户缺口，所以只要任一池有缺口，总结论会优先显示缺口。

### Q：`+20` 是不是代表一定可以马上新增 20 个用户？

A：不是保证值，而是同类订阅用户画像下的保守建议。实际执行时建议小批量新增，新增后复跑 skill 观察容量利用率、平均耗时和缺口是否变化。

### Q：核心接口失败会怎样？

A：`recommendations.json` 和 `usage_stats.json` 是核心数据，失败时会中止。其它接口失败会写入 `data/_errors.json`，报告会说明数据完整性风险。

### Q：能不能自动帮我补账号或恢复账号？

A：不能。这个 skill 只做只读分析。补账号、恢复账号、调整分组或改配置都只会作为建议写入报告。

## 备选用法：脚本流程

如果你想自己先采集和分析，再让 Agent 解读，可以直接运行脚本。

### 使用 Admin API Key 采集

```bash
# 建议用环境变量传入凭据，避免密钥出现在命令历史里
export SUB2API_BASE_URL="https://example.com"
export SUB2API_ADMIN_API_KEY="sk-xxxx"

# 采集最近 7 天的只读容量数据
python3 skills/sub2api-add-users/scripts/collect_sub2api_add_users_data.py \
  --out-dir "./tmp/sub2api-add-users/run-20260502231000/data"
```

### 指定时间段采集

```bash
# 设置站点与只读管理员凭据
export SUB2API_BASE_URL="https://example.com"
export SUB2API_ADMIN_API_KEY="sk-xxxx"

# 采集指定 ISO 时间段；接口会按覆盖该时间段的日期桶查询
python3 skills/sub2api-add-users/scripts/collect_sub2api_add_users_data.py \
  --since "2026-04-25T00:00:00+08:00" \
  --until "2026-05-02T00:00:00+08:00" \
  --timezone "Asia/Shanghai" \
  --out-dir "./tmp/sub2api-add-users/run-20260502231000/data"
```

### 使用管理员 JWT 采集

```bash
# JWT 模式会使用 Authorization: Bearer 请求头
export SUB2API_BASE_URL="https://example.com"
export SUB2API_AUTH_MODE="bearer"
export SUB2API_AUTH_TOKEN="eyJ..."

# 采集只读容量数据
python3 skills/sub2api-add-users/scripts/collect_sub2api_add_users_data.py \
  --out-dir "./tmp/sub2api-add-users/run-20260502231000/data"
```

### 生成分析初稿

```bash
# 根据 data/ 目录生成 analysis/summary.json、analysis/pool_capacity.csv 和 report.md 初稿
python3 skills/sub2api-add-users/scripts/analyze_sub2api_add_users.py \
  --data-dir "./tmp/sub2api-add-users/run-20260502231000/data" \
  --analysis-dir "./tmp/sub2api-add-users/run-20260502231000/analysis" \
  --report-path "./tmp/sub2api-add-users/run-20260502231000/report.md"
```

脚本生成的是确定性初稿。最终交付时，仍建议让 Agent 结合 `references/source-map.md` 里的源码核对地图继续解释容量推荐口径、可调度账号定义和执行风险。
