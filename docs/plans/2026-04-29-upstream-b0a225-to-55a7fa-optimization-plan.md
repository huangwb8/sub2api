# 上游 b0a225 到 55a7fa 变更吸收计划

**目标**：分析 `Wei-Shaw/sub2api` 在 `(b0a2252ed19c3720e6adafde6083e64fbac2efa9, 55a7fa1e07443212681b7ac4b0df56237d7558d5]` 区间的真实变化，判断这些变化对当前个人 `sub2api` 项目的启发与必要吸收项。本计划只沉淀后续优化方案，不修改当前源代码。

**方法**：本轮按 `awesome-code` 流程运行 `agent_coordinator.py`，结果为 `coordination_scope.level=single-pass`、`dispatch_gate.can_proceed=true`，无需阻塞式子代理分派。随后执行 `git fetch upstream main`、`git log`、`git diff --stat`、`git diff --name-status`、license diff、当前仓库代码探针，按“必须吸收 / 建议吸收 / 暂不吸收”给出计划。

**上游比较链接**：`https://github.com/Wei-Shaw/sub2api/compare/b0a2252ed19c3720e6adafde6083e64fbac2efa9...55a7fa1e07443212681b7ac4b0df56237d7558d5`

**关键结论**：该区间包含 41 个 commit，其中 28 个非 merge commit、13 个 merge commit；共影响 62 个文件，约 `3682 insertions / 218 deletions`。上游变化不只是小修补，而是集中补强了 Vertex Service Account、OpenAI/Codex 兼容、Anthropic/网关流式 failover、安全脱敏、账号批量编辑、API Key 限速重置和 Ops 清理策略。当前个人仓库已有部分相邻能力，但仍建议选择性吸收其中的正确性、安全性与管理效率改进。

**License 结论**：上游该区间没有修改 `LICENSE`。`git diff b0a2252e..55a7fa1e -- LICENSE` 为空；本轮不需要同步修改当前仓库 license。

## 上游区间变化总览

### Vertex Service Account 支持

涉及文件：

- `backend/internal/service/vertex_service_account.go`
- `backend/internal/service/claude_token_provider.go`
- `backend/internal/service/gemini_token_provider.go`
- `backend/internal/service/gateway_service.go`
- `backend/internal/domain/constants.go`
- `frontend/src/components/account/CreateAccountModal.vue`
- `frontend/src/components/account/EditAccountModal.vue`
- `frontend/src/constants/account.ts`
- `frontend/src/components/common/PlatformTypeBadge.vue`

上游新增 `service_account` 账号类型，用 Google Service Account JSON 生成 OAuth access token，并支持：

- Gemini / Vertex token cache key。
- Anthropic on Vertex 的 URL、模型 ID、location、请求体转换。
- Vertex 专用 model/location 映射。
- 前端创建、编辑账号时录入 Service Account JSON。
- 单元测试覆盖 service account JSON 校验、token 获取、Claude Vertex 请求构造。

### OpenAI / Codex 兼容性修复

涉及文件：

- `backend/internal/service/openai_codex_transform.go`
- `backend/internal/service/openai_gateway_service.go`
- `backend/internal/service/openai_ws_forwarder.go`
- `backend/internal/pkg/apicompat/*.go`
- `backend/internal/service/openai_images_test.go`
- `backend/internal/service/openai_ws_forwarder_ingress*_test.go`

上游重点修复：

- Codex OAuth / passthrough 路径剥离不支持的 `max_output_tokens`、`max_completion_tokens`、`temperature`、`top_p`、penalty 等字段。
- 保留当前 Codex compact payload 所需字段，避免过度清理。
- Responses API 的 function `tool_choice` 改为兼容格式：`{"type":"function","name":"..."}`，并兼容旧的 nested `function.name`。
- OAuth 路径 drop reasoning items，避免上游不接受历史 reasoning item。
- OpenAI Images honor versioned base URL，避免自定义 base URL 拼接错版本。
- WebSocket ingress 避免把显式 tool replay 误判为隐式 continuation。

### 网关流式 failover 与错误脱敏

涉及文件：

- `backend/internal/service/gateway_service.go`
- `backend/internal/service/gateway_streaming_test.go`
- `backend/internal/pkg/apicompat/*`

上游重点修复：

- Anthropic stream 在客户端输出前遇到 EOF / unexpected EOF 时，包装为 `UpstreamFailoverError`，让 handler 有机会切换账号。
- Anthropic 标准 SSE error event 与 failover body 对齐。
- 对客户端可见的 stream error 做脱敏，避免泄露内部 IP、端口或上游网络拓扑。

### 账号批量编辑增强

涉及文件：

- `backend/internal/handler/admin/account_handler.go`
- `backend/internal/service/admin_service.go`
- `frontend/src/components/account/BulkEditAccountModal.vue`
- `frontend/src/views/admin/AccountsView.vue`
- `frontend/src/api/admin/accounts.ts`

上游新增“按当前筛选结果批量编辑”能力：

- 后端 `bulk-update` 可接收 `filters`，不再只能接收显式 `account_ids`。
- 前端批量编辑弹窗支持 selected / filtered 两种目标模式。
- 对筛选范围、混合平台风险、OpenAI compact 字段做类型与测试补强。

### API Key 限速用量重置

涉及文件：

- `backend/internal/handler/admin/apikey_handler.go`
- `backend/internal/service/admin_service.go`
- `backend/internal/service/billing_cache_service.go`
- `backend/internal/handler/admin/apikey_handler_test.go`

上游让管理员更新 API Key 时可以同时传 `reset_rate_limit_usage=true`，清空 5h / 1d / 7d 限速窗口，并同步失效认证缓存与 Redis 限速缓存。

### Ops 清理策略增强

涉及文件：

- `backend/internal/service/ops_cleanup_service.go`
- `backend/internal/service/ops_settings.go`
- `frontend/src/views/admin/ops/components/OpsSettingsDialog.vue`

上游将保留天数 `0` 定义为“每次清理全部清空”，并使用 `TRUNCATE TABLE` 实现。负数才视为非法或回退默认值。

### 文档与赞助信息

涉及文件：

- `README.md`
- `README_CN.md`
- `README_JA.md`
- `assets/partners/logos/pateway.png`

上游新增 PatewayAI 赞助信息，并移除 superpowers docs。该类变化对个人 fork 的业务能力没有直接收益，除非需要保持上游 README 赞助口径一致，否则不建议吸收。

## 当前个人仓库现状判断

### 已经具备或局部具备

- 当前仓库用户侧 API Key 更新已经支持 `reset_rate_limit_usage`，并会在重置时清空窗口和失效 Redis 限速缓存，位置包括 `backend/internal/service/api_key_service.go` 与 `backend/internal/handler/api_key_handler.go`。
- 当前 OpenAI Codex transform 已经清理一批不支持字段，包括 `max_output_tokens`、`max_completion_tokens`、`temperature`、`top_p`、penalty 等。
- 当前已有 `NeedsToolContinuation` 相关逻辑，说明你这边已经独立补过一部分 tool continuation 识别能力。
- 当前前端 i18n 已有 service account 文案，但后端未见 `AccountTypeServiceAccount`、`vertex_service_account.go` 或完整 token provider 接线，说明这部分目前更像“前端残留/预留”，不是完整闭环。

### 明显缺口

- 管理员侧 API Key 更新目前仍只有 `group_id`，尚不能像上游一样从管理端重置限速用量。
- 账号批量编辑仍要求显式 `account_ids`，没有上游的 `filters` 目标模式；大批量跨页操作仍需用户手动勾选。
- 当前 `tool_choice` 兼容层仍使用 nested `{"function":{"name":"..."}}` 形式；上游已改为 Responses 兼容的 flat `{"name":"..."}`，这是实质兼容性修复。
- 当前 Ops retention 校验仍要求 `1..365`，`0` 会被归一化为默认值，未吸收上游“0=清空全部”的运维语义。
- 当前未见上游的 stream error 脱敏函数与对应测试，需要重点确认客户端可见错误是否可能泄露内部地址。
- 当前未见完整 Vertex Service Account 后端闭环。若继续保留前端文案，后端能力缺口容易造成误导。

## 是否有必要吸收

### P0：必须优先吸收

#### OpenAI / Codex tool_choice 与 tool replay 兼容修复

理由：

- 当前代码仍使用 nested function `tool_choice`，与上游这次修复的目标相冲突。
- 这会影响 Chat Completions / Anthropic / Responses 互转，以及 Codex CLI、工具调用续链场景。
- 兼容层属于网关核心路径，风险高、收益明确。

建议：

- 将 `tool_choice` 互转统一到 Responses flat 格式。
- 保留对旧 nested 格式的反向兼容。
- 补齐 `anthropic_to_responses`、`responses_to_anthropic_request`、`chatcompletions_to_responses` 三条路径测试。
- 同步审查 `NeedsToolContinuation` 与 OpenAI WS ingress，避免显式 tool replay 被误判为隐式续链。

#### 网关流式 failover 与错误脱敏

理由：

- 这是稳定性与安全性问题，不只是体验优化。
- 客户端输出前的 EOF 应触发 failover，而不是直接把单账号断流变成最终失败。
- 网络错误原文可能包含内部 IP、端口、上游地址，属于不必要的信息泄露。

建议：

- 按当前 fork 的 `UpstreamFailoverError` 与已有 failover 结构吸收上游语义，不机械照搬函数位置。
- 为 Anthropic stream、兼容 Responses stream、buffered SSE 缺 terminal event 等路径补回归测试。
- 客户端可见错误统一走脱敏函数，原始错误只进内部日志。

### P1：建议吸收

#### 管理员侧 API Key 限速用量重置

理由：

- 当前用户侧已有重置能力，管理端缺少同等能力，运维处理被限速用户时不够顺手。
- 上游实现很小，且与当前已有 `InvalidateAPIKeyRateLimit` 语义一致，迁移成本低。

建议：

- 在 `AdminAPIKeyHandler` 增加 `reset_rate_limit_usage` 可选字段。
- 在 `AdminService` 增加清空窗口与缓存失效逻辑。
- 保持 `group_id` 与 reset 可组合：只 reset 不改 group、只改 group、不同时存在都应可用。

#### 按筛选结果批量编辑账号

理由：

- 当前个人 fork 管理端账号能力比上游更复杂，跨页批量维护需求更强。
- 上游 `filters` 模式可以显著降低人工操作成本，但必须结合你这边已有的多词搜索、分组筛选、混合渠道风险确认、账号级调度机制规则进行适配。

建议：

- 不直接照搬上游 UI；先在后端补 `filters` 解析和目标 ID 快照。
- 前端弹窗增加 selected / filtered 目标模式，并清晰展示预计影响数量。
- 继续保留混合平台、混合渠道、OpenAI compact 字段风险确认。
- 对 `search` 使用当前 fork 已增强的多词命中语义，避免与现有搜索规则不一致。

#### Ops retention `0=清空全部`

理由：

- 对本地运维、测试站清理、日志爆量恢复有实用价值。
- 上游用 `TRUNCATE` 避免大表批量 delete 带来的 WAL / VACUUM 压力。

建议：

- 只对 ops 表开放 `0` 语义，不扩散到业务 usage / billing 表。
- 前端提示必须明确“0 表示每次清理全部清空”，避免误操作。
- 增加清理审计日志或至少保留 heartbeat 计数，便于追溯。

#### OpenAI Images versioned base URL

理由：

- 当前 fork 支持自定义上游与多种 OpenAI 兼容入口，URL 拼接错误会造成假阳性测试或线上 404。
- 该修复范围较窄，适合与 OpenAI 兼容补丁一起吸收。

建议：

- 先复核当前 `openai_images` / 自定义 base URL 拼接路径。
- 增加带 `/v1`、带版本路径、无尾斜杠三类回归测试。

### P2：按使用场景吸收

#### Vertex Service Account 完整闭环

理由：

- 如果你实际需要 Anthropic on Vertex 或 Gemini Vertex 账号，这是上游区间中最大的新能力。
- 但它引入新的账号类型、密钥 JSON、JWT 签名、token 缓存、Vertex URL / 模型映射和前端表单，改动面大。
- 当前 fork 仅有前端 i18n 文案，后端未闭环；如果不实现，建议至少清理或隐藏相关文案入口，避免误导管理员。

建议：

- 若短期要支持 Vertex：作为独立 feature 分支实施，按“后端能力 -> account test -> 前端入口 -> e2e 测试”推进。
- 若短期不用：不要吸收完整功能，只做一次 UI/文案可见性核查，确认没有无法使用的 service account 入口暴露给管理员。

#### 筛选批量编辑的前端体验增强

理由：

- 上游功能有用，但当前 fork 的账号管理页面已经高度定制。
- 前端需要截图审查，避免把批量操作入口做得过重或误触。

建议：

- 后端 API 先行。
- 前端作为第二阶段，按项目 UI 流程截图对比。

### 暂不建议吸收

- 赞助商 README / logo 变化：对个人 fork 的运行能力没有帮助。
- `superpowers` docs 删除：如果当前 fork 没有对应文档，不需要跟随。
- 上游 merge commit 本身：只吸收实际语义，不按 merge 历史搬运。

## 实施计划

### Task A：OpenAI / Codex 兼容补丁

**目标**：吸收 tool_choice flat 格式、legacy 兼容、显式 tool replay 识别和必要字段清理。

建议改动点：

- `backend/internal/pkg/apicompat/anthropic_to_responses.go`
- `backend/internal/pkg/apicompat/chatcompletions_to_responses.go`
- `backend/internal/pkg/apicompat/responses_to_anthropic_request.go`
- `backend/internal/service/openai_codex_transform.go`
- `backend/internal/service/openai_tool_continuation.go`
- `backend/internal/service/openai_ws_forwarder.go`

验收：

- Anthropic tool choice `{"type":"tool","name":"x"}` 转 Responses 后为 `{"type":"function","name":"x"}`。
- Responses legacy nested 格式仍能转回 Anthropic tool。
- 显式 `tools` / `tool_choice` / `function_call_output` 不触发错误 continuation 推断。
- Codex compact 与 OAuth 路径不误删当前 fork 依赖字段。

### Task B：网关流式 failover 与错误脱敏

**目标**：提高 Anthropic / Responses 流式断流稳定性，并避免客户端错误泄露基础设施细节。

建议改动点：

- `backend/internal/service/gateway_service.go`
- `backend/internal/service/gateway_forward_as_responses.go`
- `backend/internal/service/gateway_forward_as_chat_completions.go`
- `backend/internal/service/openai_gateway_service.go`
- `backend/internal/service/gemini_messages_compat_service.go`

验收：

- 客户端响应尚未提交时，EOF / unexpected EOF 返回 `UpstreamFailoverError`。
- 已提交响应后，只发送标准化 SSE error event。
- 客户端错误不包含 `read tcp 10.x.x.x:port->...`、内网 IP、端口、完整上游域名路径。
- 原始错误仍保留在内部日志或 Ops 诊断中。

### Task C：管理员 API Key 限速重置

**目标**：把当前用户侧已有的限速用量重置能力补到管理员侧。

建议改动点：

- `backend/internal/handler/admin/apikey_handler.go`
- `backend/internal/service/admin_service.go`
- `backend/internal/service/billing_cache_service.go`
- `backend/internal/handler/admin/apikey_handler_test.go`

验收：

- `PUT /api/v1/admin/api-keys/:id` 支持 `{ "reset_rate_limit_usage": true }`。
- 5h / 1d / 7d usage 和窗口开始时间被清空。
- auth cache 与 Redis rate-limit cache 同步失效。
- `group_id` 和 `reset_rate_limit_usage` 同时存在时行为稳定。

### Task D：账号按筛选结果批量编辑

**目标**：增强管理员账号批量维护能力，但保持当前 fork 的搜索、分组、调度和风险确认语义。

建议改动点：

- `backend/internal/handler/admin/account_handler.go`
- `backend/internal/service/admin_service.go`
- `frontend/src/api/admin/accounts.ts`
- `frontend/src/components/account/BulkEditAccountModal.vue`
- `frontend/src/views/admin/AccountsView.vue`

验收：

- 后端接受 `account_ids` 或 `filters`，两者至少一个必填。
- `filters` 使用当前列表查询同一套筛选语义。
- 大结果集按分页解析 ID，避免一次性拉取过大。
- 前端显示 selected / filtered 两种目标模式和预估影响数量。
- 混合平台 / 混合渠道风险确认仍生效。

### Task E：Ops retention 0 语义

**目标**：允许管理员将 ops 数据保留天数设为 `0`，表示每次定时清理全部清空。

建议改动点：

- `backend/internal/service/ops_settings.go`
- `backend/internal/service/ops_cleanup_service.go`
- `backend/internal/service/ops_cleanup_service_test.go`
- `frontend/src/views/admin/ops/components/OpsSettingsDialog.vue`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`

验收：

- `0` 不再被归一化为默认值。
- 负数仍非法或回退默认值。
- `0` 对应 `TRUNCATE TABLE`，表不存在时保持 no-op。
- 前端说明清楚，避免管理员误以为 `0` 是“不清理”。

### Task F：Vertex Service Account 决策分支

**目标**：先决定“实现完整闭环”还是“隐藏未完成入口”。

路径 1：实现完整闭环。

- 增加 `service_account` 后端账号类型。
- 增加 Vertex SA JSON 解析、JWT assertion、token cache。
- 接入 Gemini / Anthropic token provider。
- 接入 Anthropic Vertex URL 与模型 ID 转换。
- 前端补 service account 创建/编辑字段和测试。

路径 2：暂不实现。

- 审查当前前端是否暴露 service account 入口。
- 若有入口但后端不支持，应隐藏或标注不可用，避免保存失败。

## 推荐执行顺序

1. 先做 Task A 和 Task B：它们直接影响网关正确性、安全性与 failover 成功率。
2. 再做 Task C：小改动、高收益，和当前用户侧能力对齐。
3. 再做 Task D：运维效率收益明显，但前后端联动和截图审查成本较高。
4. 再做 Task E：适合运维场景，但要特别防误操作。
5. 最后决定 Task F：Vertex Service Account 是独立 feature，不建议混在兼容性修复里。

## 建议验证命令

后续真正实施时，最少执行：

```bash
cd backend
go test -tags=unit ./internal/pkg/apicompat ./internal/service ./internal/handler/admin
go test -tags=unit ./...
```

```bash
cd frontend
pnpm run typecheck
pnpm run lint:check
pnpm test
```

若实施 Task D 的前端 UI，还应按项目 UI 截图工作流生成：

- `tmp/screenshots/run-{timestamp}/before.png`
- `tmp/screenshots/run-{timestamp}/after.png`
- `tmp/screenshots/run-{timestamp}/compare.png`

## 本轮不做的事

- 不修改业务源代码。
- 不修改 license。
- 不同步上游赞助商 README 内容。
- 不直接 cherry-pick 上游 merge commit。
