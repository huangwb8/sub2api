# Upstream `1ce9dc03..c056db74` 选择性吸收优化计划

> **For Codex / Claude:** 本文档只做上游差异分析、取舍判断与实施规划，不直接修改业务源码。后续若进入实现阶段，应按主题拆包、逐项验证，禁止把当前 fork 直接追平上游。

**Goal:** 基于上游 `Wei-Shaw/sub2api` 在 `1ce9dc03f9d15e8a633dafc0e5f1bbf5ac1e179a..c056db740d56ce008292a7b414c804cc6f308208` 之间的演进，梳理哪些变化对当前个人 fork 有启发、哪些值得吸收、哪些已经被本地实现覆盖，并沉淀为低风险、可验证的后续优化路线。

**Method:** 本轮按 `awesome-code` 工作流执行 `agent_coordinator.py`；结果为 `coordination_scope.level = single-pass`、`dispatch_gate.can_proceed = true`、无 required agent 阻塞。随后结合本地 `git log`、`git diff`、当前 fork 源码探针、`CHANGELOG.md` 留痕与 `LICENSE` blob 对比，按“建议吸收 / 条件吸收 / 暂缓吸收 / 已吸收 / 不吸收”分类。

## 范围结论

- 该区间共 **47** 个提交，其中 **36** 个非 merge 提交。
- 变化主要集中在五个主题：
  - Claude Code OAuth mimicry 与 prompt caching 兼容性修复。
  - OpenAI `/responses/compact`、Codex payload 归一化、tool call / stream failover 稳定性。
  - 邀请返利与 affiliate 系统。
  - 支付细节修复，包括 Stripe 展示/放行与易支付退款端点。
  - Anthropic usage 口径修正与工具类型兼容补丁。
- 当前 fork 与上游仍是深度分叉状态，`c056db74` 不是当前 `HEAD` 的祖先，**不建议 merge / rebase / 整段 cherry-pick**。
- 这段上游变化里，真正值得当前 fork 重点吸收的不是“版本同步”，而是：
  - `P0` 流式容错与 Claude Code mimicry/prompt cache 正确性；
  - `P1` 支付退款链路细节审计；
  - `P1/P2` affiliate 能力，前提是你确实要做增长分销。

## License 检查

- 上游 `LICENSE` 在 `version1=1ce9dc03f9d15e8a633dafc0e5f1bbf5ac1e179a` 与 `version2=c056db740d56ce008292a7b414c804cc6f308208` 的 blob hash 相同，均为 `153d416dc8d2d60076698ec3cbfce34d91436a03`。
- 该区间内没有新增、删除或修改 `LICENSE` 的提交。
- 因此本轮**不需要**按“上游 license 变化”同步本地 license。
- 额外说明：当前 fork 工作树里的 `LICENSE` hash 与上游当前 blob 不同；但这不是本次区间新增变化，不属于本轮“随上游变更同步”的范围。若未来要做“完整 upstream license 对齐”，应单独立项处理。

## 上游变化摘要

### 主题 1：Claude Code mimicry 与 prompt caching

代表提交：

- `b5467d61` `fix(gateway): apply full Claude Code mimicry on /chat/completions and /responses`
- `165553cf` `fix(gateway): use full beta list in buildUpstreamRequest mimicry path`
- `66d64545` `feat(claude): add ttl to cache_control with default 5m`
- `5862e2d8` `feat(gateway): add billing attribution block with cc_version fingerprint`
- `a25faeca` `feat(gateway): align body shape with real Claude Code CLI defaults`
- `6e12578b` `feat(gateway): port Parrot tool-name obfuscation + message cache breakpoints`
- `f3233db0` `fix(gateway): apply D/E/F mimicry to native /v1/messages and count_tokens paths`
- `6dc89765` `fix(gateway): always apply full mimicry for OAuth accounts regardless of client identity`
- `bdbd2916` `fix(gateway): skip client header passthrough on OAuth mimicry path`
- `496469ac` `fix(gateway): skip body mimicry for real Claude Code clients to restore prompt caching`

核心变化：

- 把 `/chat/completions`、`/responses`、`/v1/messages`、`/count_tokens` 等路径统一拉到更接近真实 Claude Code CLI 的 body/header 形状。
- 增补 `cc_version` 指纹、`x-client-request-id`、更完整的 beta header、cache_control ttl 与 tool 名混淆逻辑。
- 后续又修正了一个关键回归：对“真实 Claude Code 客户端”不应继续做 body mimicry，否则会破坏长 system prompt 的缓存命中。

对当前 fork 的启发：

- 这是本轮**最值得重视**的主题，因为你的 fork 也已经深度做了 Claude Code / Anthropic 兼容、prompt cache、billing header 与会话隔离。
- 这里的风险不是“功能缺失”，而是“已有复杂实现中是否仍存在边缘回归”。尤其要核对：
  - 真正的 Claude Code 客户端是否会被误套第三方 mimicry，导致缓存命中下降。
  - OpenAI/Claude 兼容路径是否所有入口都复用了同一套 mimicry 规则，而不是出现路径漂移。
  - `metadata.user_id`、`cc_version`、`x-client-request-id`、`cache_control.ttl` 与 tool name rewrite 是否在不同入口表现一致。

建议：**P0 做“对表审计 + 定向补丁”，不要整包照抄。**

### 主题 2：OpenAI `/responses/compact`、Codex payload 与流式 failover

代表提交：

- `095f457c` `feat(openai): port /responses/compact account support flow`
- `e65574de` `fix(openai): normalize codex responses payloads`
- `27ee141c` `fix(openai): preserve mcp tool call ids`
- `5b63a9b0` `fix(openai): fail over before responses stream output`
- `dac6e520` `fix(openai): keep responses stream alive during pre-output failover`
- `8987e0ba` `fix(openai): tighten responses stream account tests`
- `1e57e88e` `fix(openai): bump codex CLI version from 0.104.0 to 0.125.0`

核心变化：

- 引入账号级 `/responses/compact` 能力探测、调度优先级、模型映射与后台测试入口。
- 规范 Codex payload body，保留 MCP/tool continuation 相关 `call_id`。
- 修复 Responses 流在首个 output 之前失败时的 failover 时机与 keepalive 行为。

对当前 fork 的启发：

- 这一组里，`/responses/compact`、payload 归一化、Spark 限制与 tool call id 保留，你当前 fork **大体已经吸收或做了更进一步定制**：
  - 已存在 `/v1/responses/compact` 路径保留与日志。
  - 已存在 `openai_codex_transform.go` 中的 payload 归一化、`call_id` 保留与 Spark 限制提示。
  - 已存在针对 compact / Codex 的模型映射、日志与测试。
- 但“**pre-output failover**”仍值得单独审计。你当前 fork 的 `openai_gateway_service.go` 已有复杂的 `previous_response_id`、WSv2、重放与恢复逻辑，越复杂越应该确认是否覆盖了“还没输出 token 就切账号”的边界。

建议：

- `compact / payload / tool id`：**已吸收或已超越，暂不重复实施。**
- `pre-output failover`：**P0 建议专项审计并补回归测试。**
- `CLI version bump`：**不单独吸收**，除非后续验证到某个真实客户端版本兼容问题。

### 主题 3：Affiliate 邀请返利系统

代表提交：

- `f03de00c` `feat: add affiliate invite rebate flow and admin rebate-rate setting`
- `aa8ee33b` `refactor(affiliate): tighten DI and harden inviter code validation`
- `4e1bb2b4` `feat(affiliate): add feature toggle and per-user custom invite settings`
- `9b6dcc57` `feat(affiliate): 完善邀请返利系统`

核心变化：

- 引入邀请人绑定、返利累计、管理员返利设置、功能开关、用户级自定义返利参数等完整增长链路。
- 补齐 DI、邀请码校验、事务传播与测试。

对当前 fork 的启发：

- 这是一套完整的“增长与分销”能力，不是单点 bugfix。
- 当前 fork 代码里没有现成的 affiliate/rebate 主线；如果要吸收，涉及 repository、service、迁移、用户中心页面、后台设置、支付履约勾连，工程体量不小。
- 它对“个人 sub2api 项目”是否必要，取决于你现在的目标：
  - 如果你重心是稳定性、兼容性、计费正确性，这一主题优先级不高。
  - 如果你准备做邀请裂变、代理合作或推广返佣，这会是一个成体系的增长抓手。

建议：**P2 条件吸收。只有确认要做增长分销时才立项。**

### 主题 4：支付细节修复

代表提交：

- `8f28a834` `fix(payment): 同时启用易支付和 Stripe 时显示 Stripe 按钮`
- `c1b52615` `fix(payment): allow Stripe payment pages to bypass router auth guard`
- `1a0cabbf` `Fix Zpay refund endpoint handling`

核心变化：

- 修复前端可见支付方式过滤漏掉 Stripe 的问题。
- 修复 Stripe 支付页被路由守卫挡住的问题。
- 修复易支付/ZPay 退款端点处理。

对当前 fork 的启发：

- 你的 fork 已经有更完整的支付体系，也已经具备 `unknown order -> 2xx ack` 的 webhook 契约，因此这组不是“必须整包搬运”。
- 但 `ZPay/EasyPay 退款` 仍值得审计，因为它直接关联真实资金链路，且当前 fork 虽然已有退款服务与 EasyPay provider，但未看到与上游同名的退款专项测试文件。
- Stripe 按钮和 auth guard 问题属于**低成本、高可验证**的前端回归检查项，适合作为支付自测 checklist 的一部分，而不一定要专门抄提交。

建议：

- `ZPay/EasyPay 退款端点`：**P1 建议审计并补测试。**
- `Stripe 按钮 / auth guard`：**P2 快速核对即可。**

### 主题 5：Anthropic usage 语义与工具兼容

代表提交：

- `b17704d6` `fix(anthropic): 修正缓存 token 的 Anthropic 用量语义`
- `5f630fbb` `fix(apicompat): recognize web_search_20250305 / google_search in Responses to Anthropic tool conversion`

核心变化：

- 修正 Responses → Anthropic usage 映射时 cache token 的语义。
- 兼容 `web_search_20250305` / `google_search` 等工具类型。

对当前 fork 的启发：

- `web_search_20250305 / google_search` 兼容你当前 fork 已经具备，相关类型与转换逻辑已存在。
- cache token 语义方面，你当前 fork 也已有 `cached_tokens -> cache_read_input_tokens` 的兼容测试与处理，但由于这类问题非常容易在不同路径出现“半同步”，仍值得做一次“非流式 + 流式 + passthrough”全链路核对。

建议：

- `tool type 兼容`：**已吸收，无需重复做。**
- `Anthropic usage 语义`：**P1 建议加一轮对表审计，重点防止不同转换路径口径不一致。**

## 当前 fork 对齐判断

### 已确认已吸收或已有更强本地实现

- OpenAI `/responses/compact` 路径保留、日志与测试已存在。
- `openai_codex_transform.go` 已覆盖 payload 归一化、`call_id` 保留、Spark 图片限制提示等关键点。
- 支付 webhook 的 `ErrOrderNotFound -> 2xx ack` 契约已存在。
- `web_search_20250305 / google_search` 的兼容映射已存在。
- 近期本地 `CHANGELOG.md` 已明确记录：
  - OpenAI 403 连续失败计数与临时冷却。
  - 网关 RPM 限流最小闭环。
  - Spark 限制提示强化。

### 已确认当前 fork 仍值得进一步核对

- Claude Code mimicry 是否在所有入口保持一致，且不会伤害真实客户端 prompt cache。
- Responses 流在首个 output 之前失败时，当前 WSv2/HTTP 逻辑是否都能安全 failover。
- EasyPay/ZPay 退款端点是否与上游修复后的行为一致，且已有充分测试。
- Responses ↔ Anthropic usage 映射在非流式、流式、passthrough 三条路径上是否完全同口径。

### 已确认当前 fork 暂未具备，但是否需要取决于产品方向

- affiliate 邀请返利系统及其前后台页面、迁移、支付返利结算联动。

## 吸收优先级

### P0：建议近期吸收

1. **Claude Code mimicry / prompt cache 对表审计**
   - 目标：确认真实 Claude Code 客户端不会被误做 body mimicry，第三方客户端仍维持必要伪装。
   - 重点：`metadata.user_id`、`cc_version`、`x-client-request-id`、`beta header`、`cache_control.ttl`、tool name rewrite。
   - 实施方式：以当前 fork 既有实现为主，补测试，不追求代码形态与上游一致。

2. **Responses pre-output failover 审计与补测**
   - 目标：在首个 output 产生前发生上游失败时，确保可以安全切换账号或恢复连接，且不会破坏流式响应。
   - 重点：HTTP Responses、WSv2、`previous_response_id`、tool continuation、keepalive。
   - 实施方式：优先补失败注入测试，再按测试结果最小修补。

### P1：建议中期吸收

1. **EasyPay/ZPay 退款端点专项审计**
   - 目标：核对退款请求路径、参数签名、返回解析与错误处理。
   - 重点：真实 provider 差异、退款失败重试边界、单测覆盖。

2. **Anthropic usage 语义一致性审计**
   - 目标：确认 `cached_tokens`、`cache_read_input_tokens`、`cache_creation_input_tokens` 在所有转换路径口径一致。
   - 重点：non-streaming、streaming、passthrough、Responses→Anthropic。

### P2：按业务策略决定

1. **Affiliate 邀请返利系统**
   - 只有在你准备做推广返佣、邀请裂变或代理合作时才值得立项。
   - 如果立项，建议拆成“数据模型与迁移 → 履约返利 → 后台配置 → 用户页与文案 → 反作弊/风控”五步，而不是一次性搬运。

2. **Stripe 前端回归核对**
   - 作为支付链路自测 checklist 的一个子项即可，不建议为此单独大改。

## 建议执行顺序

### 第一阶段：只做正确性审计

- 审计 Claude Code mimicry 与真实客户端 prompt cache 是否冲突。
- 审计 Responses pre-output failover 的边界处理。
- 审计 EasyPay/ZPay 退款端点与 Anthropic usage 语义。

### 第二阶段：只补最小必要修复

- 对第一阶段发现的问题，按“补测试优先、逻辑最小修复其次”的策略处理。
- 禁止顺手重构无关链路，避免把本地 fork 的个性化逻辑冲掉。

### 第三阶段：再决定是否做增长能力

- 若你明确要做邀请返利，再单独为 affiliate 开一份专题实现计划。
- 若当前重心仍是网关稳定性与支付正确性，则 affiliate 继续延期。

## 最终判断

- **有启发，而且有必要吸收一部分。**
- 但最值得吸收的不是“完整跟进上游 47 个 commit”，而是：
  - `P0` Claude Code mimicry/prompt cache 正确性；
  - `P0` Responses pre-output failover；
  - `P1` EasyPay/ZPay 退款端点；
  - `P1` Anthropic usage 语义一致性。
- **affiliate 邀请返利系统暂不建议默认开做**。它是增长功能，不是当前 fork 的稳定性短板；除非你接下来明确要做分销，否则先把兼容性、缓存命中、流式容错和支付正确性打牢，收益更高。
