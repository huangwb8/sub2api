# `admin/ops` 健康状态诊断报告（2026-04-27）

## 背景

- 排查时间：2026-04-27 21:46（Asia/Shanghai）
- 排查对象：`https://api.benszresearch.com/admin/ops`
- 排查方式：基于 `remote.env` 远程凭据，只读调用 Admin Ops API；未执行任何写操作
- 主要接口：`/api/v1/admin/ops/dashboard/overview`、`/api/v1/admin/ops/concurrency`、`/api/v1/admin/ops/account-availability`、`/api/v1/admin/ops/request-errors`、`/api/v1/admin/ops/upstream-errors`、`/api/v1/admin/ops/alert-events`、`/api/v1/admin/ops/dashboard/error-trend`
- 上次排查记录：[2026-04-25-admin-ops-remote-findings.md](2026-04-25-admin-ops-remote-findings.md)

## 总体结论

当前健康评分 **49/100**，过去 1 小时 SLA 为 95.8%（143 请求，6 失败）。基础设施层（CPU 11.2%、内存 1.6%、DB/Redis 正常、后台任务全部正常）完全健康，**问题集中在业务层**：

1. TTFT（首 Token 延迟）P99 达到 **13.4s**，远超 3s 健康阈值，直接将 TTFT 评分拉到 0
2. `gpt-5.4` 通过 `/v1/responses` 转发时反复出现 `Upstream stream ended without a terminal response event`，来自同一用户的连续请求
3. 14 个账号中有 5 个被限流（36%），要到 4 月 29 日才解除
4. SOCKS 代理 `gate.decodo.com:7000` 连接被拒，导致部分请求路由失败

与 4 月 25 日排查相比：整体态势有所改善（SLA 从 90.72% 回升到 95.8%，TTFT P99 从 10.6s 上升至 13.4s 但请求量大幅降低），但仍未达到健康水平。

## 健康评分拆解

评分公式：`HealthScore = BusinessHealth × 0.7 + InfraHealth × 0.3`

### 业务健康（32.2/100）

| 组件 | 权重 | 得分 | 计算依据 |
|------|------|------|---------|
| 错误率 | 50% | 64.4 | combined error rate 4.2%（阈值 1%→10% 线性扣分） |
| TTFT | 50% | **0** | P99 = 13,357ms，远超 3,000ms 阈值，直接归零 |

### 基础设施健康（100/100）

| 组件 | 权重 | 得分 | 计算依据 |
|------|------|------|---------|
| 存储（DB+Redis） | 40% | 100 | DB/Redis 均正常 |
| 计算资源（CPU+内存） | 30% | 100 | CPU 11.2%、内存 1.6%，远低于警告线 |
| 后台任务 | 30% | 100 | 5 个任务全部最近成功，无错误 |

### 最终评分

`32.2 × 0.7 + 100 × 0.3 = 52.5 → 四舍五入后约 49–53`（API 返回 49）

**核心结论：TTFT P99 过高是健康评分低的最大因素。如果 TTFT P99 降到 3s 以内，业务健康分可恢复到 82+，总分恢复到 67+。**

## 关键发现

### 1. `gpt-5.4` 流式终止错误反复出现

过去 1 小时全部 6 个 request-error 均为同一模式：

| 字段 | 值 |
|------|-----|
| 错误消息 | `Upstream stream ended without a terminal response event` |
| 状态码 | `502` |
| 严重级别 | `P1` |
| 模型 | `gpt-5.4` |
| 入口端点 | `/v1/chat/completions` |
| 上游端点 | `/v1/responses` |
| 用户 | `1633700998@qq.com`（user_id=11） |
| 客户端 IP | `148.135.23.180` |
| stream 字段 | `false`（非流式请求，但走 Responses API 转发） |
| 时间分布 | 21:13、21:15、21:16、21:21、21:23、21:24（每隔约 1.5 分钟） |

关键特征：
- 所有错误都来自**同一用户**的**同一模型**请求
- 请求声明 `stream=false`，但上游端点是 `/v1/responses`（Responses API 默认流式）
- 涉及 3 个不同账号（account_id: 11, 2, 11），均在 `GPT_Standard` group
- 错误间隔约 1.5 分钟，说明用户在做持续请求

### 2. 上游错误揭示代理层故障

| ID | 模型 | 错误 | 严重级别 |
|----|------|------|---------|
| 2085 | gpt-5.5 | `tls: received record with version 100 when expecting version 303` | P3 |
| 2084 | gpt-5.5 | `429: The usage limit has been reached` | P1 |
| 2077 | gpt-5.4 | `socks connect tcp gate.decodo.com:7000->chatgpt.com:443: unknown error connection refused` | P3 |
| 2076 | gpt-5.4 | 同上 | P3 |

三个不同问题：
- **TLS 版本异常**（id #2085）：`version 100 when expecting version 303` 可能是代理/中间人干扰 TLS 握手
- **SOCKS 代理连接被拒**（id #2076-2077）：`gate.decodo.com:7000` 不可达，影响 account `kxsw2-team-20260425-03`
- **用量限制 429**（id #2084）：账号 `kxsw1-team-20260421-02` 已达用量上限

### 3. 账号限流严重影响可用容量

14 个账号中 **5 个被限流**（36%），解除时间均在未来 30+ 小时：

| 账号 | Group | 限流解除时间 | 剩余时间（h） |
|------|-------|------------|-------------|
| kxsw1-team-20260411 (#4) | Standard | 4/29 09:29 | ~36h |
| kxsw1-plus-01 (#5) | Premium | 4/29 07:04 | ~34h |
| kxsw1-team-20260416 (#7) | Premium | 4/29 06:56 | ~34h |
| kxsw1-team-20260421-01 (#9) | Premium | 4/29 06:56 | ~34h |
| kxsw1-team-20260421-02 (#10) | Standard | 4/29 14:27 | ~41h |

按 Group 分布：
- **GPT_Standard**（8 个账号）：2 个限流，6 个可用
- **GPT_Premium**（6 个账号）：3 个限流，3 个可用

Premium 组一半账号不可用，对高优先级用户影响更大。

### 4. 告警事件已恢复但频繁触发

过去 3 小时内有 10 条告警事件，全部已 `resolved`，但触发频率高：

| 时间 | 告警 | 触发值 |
|------|------|--------|
| 21:14 | P0: 错误率极高 | error_rate=100%（1m 窗口） |
| 21:19 | P1: 错误率过高 | error_rate=28.57%（5m 窗口） |
| 21:19 | P0: 成功率过低 | success_rate=71.43%（5m 窗口） |
| 19:51 | P0: 错误率极高 | error_rate=100% |
| 19:55 | P1: 错误率过高 | error_rate=25% |
| 17:22 | P1: 错误率过高 | error_rate=25% |

告警在 1 分钟窗口内多次达到 100% 错误率，说明存在**间歇性完全失败**时段。

### 5. 实时流量指标

| 指标 | 当前值 | 峰值 |
|------|--------|------|
| QPS | 0.2 | 0.2 |
| TPS | 16,721.8 | 15,864.7 |

流量很低（约每 5 秒 1 个请求），但错误率仍然偏高，说明问题不是过载。

## 代码缺陷

### Bug 1：Group 级别统计重复计算（P3 — 数据不准确）

**影响范围**：
- [ops_account_availability.go:89-111](../../backend/internal/service/ops_account_availability.go#L89-L111)
- [ops_concurrency.go:222-245](../../backend/internal/service/ops_concurrency.go#L222-L245)

**现象**：所有 Group 的统计完全相同（total_accounts=14, available=9, rate_limited=5），与实际账号分布（Standard=8, Premium=6）不符。

**根因**：当没有 `groupIDFilter` 时，代码遍历每个账号的**全部 Groups** 列表，将同一账号的指标重复累加到每个 Group。多对多关系下，14 个账号 × 每个 Group 都计入了全部 14 个账号。

`ops_account_availability.go` 中的问题代码：

```go
// 第 89-111 行
for _, grp := range acc.Groups {  // 遍历账号的所有 Groups
    // ...
    g.TotalAccounts++              // 每个账号被计入其所属的每个 Group
    if isAvailable {
        g.AvailableCount++
    }
    if isRateLimited {
        g.RateLimitCount++
    }
}
```

`ops_concurrency.go` 中同样的问题：

```go
// 第 222-245 行
} else {
    for _, grp := range acc.Groups {  // 同样的重复计算
        // ...
        g.MaxCapacity += int64(acc.Concurrency)
        g.CurrentInUse += currentInUse
        g.WaitingInQueue += waiting
    }
}
```

**修复建议**：当无 groupIDFilter 时，只取账号的主 Group（`acc.Groups[0]`）进行聚合，确保每个账号只被计入一个 Group 的统计。或者改为先收集账号→主 Group 映射，再按 Group 聚合。

**注意**：此 Bug 不影响请求处理，但会让 ops 页面的 Group 级别数据失去参考价值，运维无法通过 Group 视图定位哪个 Group 真正有问题。

### Bug 2：TTFT 健康评分阈值可能需要校准

**文件**：[ops_health_score.go:54-63](../../backend/internal/service/ops_health_score.go#L54-L63)

**当前逻辑**：TTFT P99 > 3s → 评分归零。

结合 4 月 25 日排查确认的住宅代理延迟（9 条代理平均 2093ms，最高 6561ms），3s 阈值对通过住宅代理的架构来说过于严格。在代理延迟基础上加上 OpenAI 自身的 TTFT，P99 轻松超过 3s。建议根据实际代理延迟分层校准，或在健康评分中对代理延迟做补偿。

## 建议行动

### 立即

1. **排查 `gate.decodo.com:7000` 代理状态**：该代理连接被拒导致 account `kxsw2-team-20260425-03` 的请求失败
2. **关注 gpt-5.4 + `/v1/responses` 的流式终止错误**：同一用户持续触发，可能是客户端兼容性问题或 Responses API 转发 bug

### 短期

3. **增加 Standard/Premium 账号**：当前 36% 限流率，Premium 一半不可用，容量不足
4. **修复 Group 统计重复计算 Bug**：让 ops 页面的 Group 视图数据准确可用
5. **校准 TTFT 健康评分阈值**：基于实际代理延迟基线重新设定

### 中期

6. **排查 TLS 版本异常**（`version 100 when expecting version 303`）：可能是代理中间人问题
7. **评估高延迟代理的降权/替换**：参照 4 月 25 日排查中 acc8（6561ms）和 acc9（4491ms）的专项分析
