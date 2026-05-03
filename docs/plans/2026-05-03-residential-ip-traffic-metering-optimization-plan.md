# 住宅代理流量计量优化计划

## 背景

基于 Decodo 代理后台 2026-04-26 ~ 2026-05-03（8天）的实际流量数据与 sub2api 站点估算的对比分析，发现当前流量计量算法存在系统性低估。

## 实测数据

| 指标 | Decodo 实际 | sub2api 估算 (校准值 5.8646) | sub2api 估算 (默认值 7.096) |
|------|------------|---------------------------|---------------------------|
| 总流量 | **15.30 GB** | 12.15 GB (**-20.6%**) | 14.70 GB (**-3.9%**) |

默认值 7.096 与实际偏差仅 -3.9%，已经足够准确。

## 根因分析

### 校准值为何"越校越偏"

校准逻辑计算 `sum(observed_bytes) / sum(observed_tokens)`，其中：

- streaming 请求的 `proxy_traffic_output_bytes = 0`（未捕获）
- `observed_tokens` 包含完整的 input + output + cache tokens
- 分子偏低 → 校准值从 7.096 被拉低到 5.8646

### streaming 响应字节未捕获

源码中 streaming 路径的 `resultWithUsage()` 不设置 `ProxyResponseBytes`，导致：

- 绝大多数 LLM API 调用（streaming）的响应流量"看不见"
- 校准计算基于不完整数据，产生偏差

## 优化方案

### P0：锁定校准值下限为默认值（防止"越校越偏"）

**目标**：校准值不得低于默认值 7.096，避免因 streaming 数据缺失导致校准值被系统性拉低。

**修改位置**：`backend/internal/service/residential_ip_estimator.go` → `buildResidentialIPCalibration()`

**当前逻辑**（第 382-387 行）：

```go
if value < 1 {
    value = 1
}
if value > 64 {
    value = 64
}
```

**改为**：

```go
if value < dashboardOversellDefaultEffectiveBytesPerToken {
    value = dashboardOversellDefaultEffectiveBytesPerToken
}
if value > 64 {
    value = 64
}
```

**效果**：校准结果永远不低于 7.096。实测证明 7.096 与 Decodo 实际流量偏差仅 -3.9%，可信赖。当 streaming 响应字节采集完善后，校准值可能会自然上升超过 7.096，届时此下限不会造成限制。

**风险评估**：极低。如果真实 bytes/token 确实低于 7.096（几乎不可能，因为 JSON body 最少也有结构开销），下限会导致轻微高估，但这在计费场景中偏向保守（多估代理成本 = 多预留预算），方向正确。

### P1：捕获 streaming 响应字节（消除根因）

**目标**：在 streaming 路径累加 SSE chunk 长度，使 `ProxyResponseBytes` 不再为 0。

**修改位置**：

1. `backend/internal/service/gateway_forward_as_chat_completions.go` — streaming 分支的 `resultWithUsage()`
2. `backend/internal/service/gateway_forward_as_responses.go` — 同上
3. `backend/internal/service/openai_gateway_service.go` — OpenAI streaming 分支

**实现思路**：

在 streaming 循环中维护一个 `responseBytes int64` 计数器，每读取一个 SSE chunk 累加 `len(chunk)`，最终赋值给 `result.ProxyResponseBytes`。

**注意点**：

- SSE chunk 包含 `data: {...}\n\n` 帧格式，这部分是实际网络传输的一部分，应当计入
- 需区分"最终聚合响应"和"streaming chunk 原始字节"——这里应记录的是实际网络传输字节数，即 chunk 原始长度

**效果**：校准计算将有完整的分子数据，校准值可自然反映真实 bytes/token 比率。P0 的下限保护仍然保留作为兜底。

### P2：HTTP 协议开销补偿（锦上添花）

**目标**：在 token 估算基础上补偿 HTTP headers、chunked encoding 等协议开销。

**实现思路**：

在 `MeterResidentialIPTraffic()` 中，当 `estimateSource` 为 `token_estimate` 或 `mixed_observed_and_token_estimate` 时，对估算结果乘以一个协议开销系数（建议 1.05）。

**优先级说明**：

P0 + P1 落地后，偏差预计可控制在 5% 以内。P2 仅在有更精确需求时实施。

## 实施顺序

```
P0（立即） → P1（下一迭代） → P2（按需）
```

P0 是一行代码的改动，消除"越校越偏"的问题，立即生效。
P1 是结构性修复，需要仔细测试 streaming 路径的稳定性。
P2 是可选项，当前偏差已经很小。

## 验证方法

每次修改后，对比 Decodo 导出数据与 sub2api oversell-calculator API 返回的流量估算，确认偏差收窄。

验证 API：

```bash
curl -s -H "x-api-key: $ADMIN_KEY" \
  "$BASE_URL/api/v1/admin/dashboard/oversell-calculator" | jq '.data.estimate.residential_ip_estimates[]'
```

关注字段：
- `effective_bytes_per_token` — 应 ≥ 7.096（P0 生效后）
- `observed_traffic_bytes` — 应显著增大（P1 生效后）
- `estimated_total_traffic_gb` — 与 Decodo 导出数据对比
