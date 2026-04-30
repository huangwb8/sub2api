# 住宅 IP 流量对账与校准说明

本文档说明 sub2api 当前如何估算住宅 IP 流量、如何和供应商账单对账，以及管理后台里两套住宅 IP 口径分别代表什么。

## 为什么要拆成双口径

住宅 IP 成本同时服务两个目标：

- **套餐定价**：回答“给用户卖什么价才合理”。
- **站点成本观察**：回答“整站真实住宅 IP 基础设施压力有多大”。

这两个目标的统计边界并不完全一样。为了避免把不同问题压扁成一个数字，Dashboard 现在同时返回两套口径：

- `pricing`
  - 用途：套餐定价测算
  - 默认是否包含管理员：否
  - 当前是否包含失败请求：否
  - 当前是否包含探活流量：否
- `site`
  - 用途：站点真实住宅 IP 成本观察
  - 默认是否包含管理员：是
  - 当前是否包含失败请求：否
  - 当前是否包含探活流量：否

当前实现里，套餐测算结果仍以 `pricing` 口径参与建议售价计算；`site` 口径主要用于解释整站成本与运维压力。

## 当前默认对账样本

系统内置了一份用于校准的供应商样本，对应窗口为 `2026-04-26` 到 `2026-04-30`：

- 供应商账单双向流量：`9.08 GB`
- 同窗口内历史总 token：`1,373,947,575`
- 旧 `4 Bytes/token` 估算：约 `5.118354 GB`
- 相对误差：约 `-43.63%`
- 反推建议校准：约 `7.096031857 Bytes/token`

这意味着旧的固定 `4 Bytes/token` 会明显低估住宅 IP 成本，因此 Dashboard 默认已经切换到更接近供应商账单的校准值。

## 流量估算优先级

系统对住宅 IP 流量的推算遵循下面的优先级：

1. **真实字节优先**：如果 usage log 中已经落了住宅代理请求/响应字节，就优先使用这些 observed bytes。
2. **单条 usage 的 token estimate 补足**：如果一条 usage 只有部分真实字节，则只把缺失部分记为估算字节，而不是把整条记录伪装成 observed。
3. **历史 legacy fallback**：如果是旧 usage、当时尚未落库代理流量快照，则用 `effective_bytes_per_token` 对 token 总量做折算。

当前 `usage_logs` 已增加以下住宅代理归因字段：

- `proxy_id`
- `used_residential_proxy`
- `proxy_traffic_input_bytes`
- `proxy_traffic_output_bytes`
- `proxy_traffic_overhead_bytes`
- `proxy_traffic_estimate_source`

这样可以把“是否走住宅代理”“走了哪个代理”“这条流量到底是实测还是估算”都留在历史 usage 上，避免后续账号换绑代理后把历史归因冲掉。

## Dashboard 字段怎么理解

`residential_ip_estimates[]` 中每个 scope 都会给出：

- `scope`
- `includes_admin`
- `includes_failed_requests`
- `includes_probe_traffic`
- `actual_days`
- `involved_users`
- `estimated_total_traffic_gb`
- `estimated_monthly_traffic_gb`
- `estimated_monthly_cost_usd`
- `estimated_monthly_cost_cny`
- `effective_bytes_per_token`
- `calibration_source`
- `traffic_basis`
- `observed_traffic_bytes`
- `estimated_traffic_bytes`

其中：

- `observed_traffic_bytes` 表示已直接观测到的请求/响应字节。
- `estimated_traffic_bytes` 表示 token 折算补足出来的部分，既可能来自新 usage 的逐条 fallback，也可能来自历史 legacy usage。
- `traffic_basis` 用于解释这条估算主要依赖什么依据，例如 `usage_log_observed_proxy_bytes`、`usage_log_token_estimate`、`legacy_token_estimate` 等。

`residential_ip_reconciliation` 则用于展示最近一次固定样本的供应商对账结果。

## 当前已知边界

当前实现已经比旧版稳定很多，但仍有几个边界需要明确：

- 目前还不是传输层 hook 级别的“绝对真实网络字节”，而是应用层可观测字节优先、缺失时用 token fallback 补足。
- `site` 口径当前已经包含管理员，但还没有默认纳入失败请求、探活流量和所有重试链路。
- 默认校准值暂时是内置样本常量，不是后台可编辑配置；后续如果需要长期运营化，可以再接配置中心或持久化对账结果。

## 运维建议

日常观察住宅 IP 成本时，建议同时看三件事：

- `pricing` 口径是否仍然适合作为套餐定价基线
- `site` 口径是否开始显著高于 `pricing`
- 最近对账样本的误差是否重新扩大

如果供应商账单与系统估算再次出现持续偏离，优先更新对账样本并重新校准 `effective_bytes_per_token`，而不是直接回退到拍脑袋常量。
