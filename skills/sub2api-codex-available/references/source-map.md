# sub2api-codex-available 源码核对地图

本文件用于把“本地 Codex 是否能调用某个账号”的诊断步骤映射到 sub2api 源码。

## 管理端账号与分组接口

- `backend/internal/server/routes/admin.go`
  - `/api/v1/admin/accounts`
  - `/api/v1/admin/accounts/:id`
  - `/api/v1/admin/accounts/:id/test`
  - `/api/v1/admin/accounts/:id/temp-unschedulable`
  - `/api/v1/admin/accounts/bulk-update`
  - `/api/v1/admin/usage`
  - `/api/v1/admin/groups/:id/api-keys`
- `backend/internal/server/middleware/admin_auth.go`
  - Admin API Key 使用 `x-api-key`
  - 管理员 JWT 使用 `Authorization: Bearer`
- `backend/internal/handler/admin/account_handler.go`
  - 账号列表、详情、测试、批量更新入口
- `backend/internal/service/account_test_service.go`
  - 管理端账号测试只验证账号到上游，不等于 Codex E2E

## OpenAI/Codex 网关入口

- `backend/internal/server/routes/gateway.go`
  - OpenAI `/v1/responses`、`/v1/chat/completions` 等入口注册
- `backend/internal/handler/openai_gateway_handler.go`
  - Responses/Codex 风格入口处理
- `backend/internal/handler/openai_chat_completions.go`
  - Chat Completions 入站处理
- `backend/internal/service/openai_gateway_service.go`
  - OpenAI 账号选择、上游请求构造、Responses 转发
- `backend/internal/service/openai_account_scheduler.go`
  - OpenAI 统一调度器
  - `RequiredAPIFormat` 会影响 `chatapi` 是否可被 `/v1/responses` 选择
- `backend/internal/service/openai_gateway_chat_completions.go`
  - Chat Completions 直连和转换路径

## 账号模型能力与 API 格式

- `backend/internal/service/account.go`
  - `IsModelSupported`
  - `ModelCapabilityStrategy`
  - `OpenAIAPIFormat`
  - `IsChatAPIResponsesEnabled`
  - `ShouldForwardChatCompletionsDirect`
- `backend/internal/pkg/openai/constants.go`
  - 默认 OpenAI 模型清单；`inherit_default` 会受该清单限制

## 用量日志核验

- `backend/internal/handler/admin/usage_handler.go`
  - `/api/v1/admin/usage?account_id=...`
- `backend/internal/repository/usage_log_repo.go`
  - usage log 字段包括 `account_id`、`model`、`requested_model`、`inbound_endpoint`、`upstream_endpoint`

## 常见根因

- 目标账号不在测试 API Key 所属分组中。
- 账号 `status` 不是 `active` 或 `schedulable=false`。
- 账号处于 rate limit、overload、temp unschedulable 或配额耗尽窗口。
- `model_capability_strategy=inherit_default`，但 Codex 请求模型不在当前默认 OpenAI 模型清单中。
- `chatapi` 账号未开启 Responses 兼容，导致 `/v1/responses` 调度阶段被 API format 过滤。
- `chatapi` 账号开启 Responses 兼容，但上游只支持 `/v1/chat/completions`，导致调度可选中但转发失败。
- 本地 Codex 使用的用户 API Key 被绑定到其它分组，或认证缓存尚未失效。

## dudu 的 Codex CLI 实测经验

可参考历史项目 `/Volumes/2T01/winE/Starup/dudu`：

- `services/worker/src/pipelines/llm/providers/codex-cli.ts`
  - dudu 把 `codex_cli` 作为单独 provider，输入包括 `model`、`reasoningEffort`、`openAiBaseUrl`、`openAiApiKey` 和可选 `configToml`。
- `services/worker/src/pipelines/llm/providers/cli-sandbox.ts`
  - 运行时向 sandbox 注入 `OPENAI_API_KEY`、`OPENAI_BASE_URL`、`OPENAI_API_BASE`。
- `docker/images/cli-sandbox/dudu-cli-run.sh`
  - 关键做法：Codex CLI 不总是可靠直接读取 `OPENAI_API_KEY`，因此先在临时 HOME 中执行 `codex login --with-api-key`，再执行 `codex exec`。
  - `codex exec` 参数会根据 `--help` 结果兼容不同版本：优先 `--dangerously-bypass-approvals-and-sandbox`，否则使用 `--sandbox danger-full-access` 和 `--ask-for-approval never` / `--approval-policy never`。
- `scripts/codex-selftest.mjs`
  - 轻量 HTTP 自测会先 GET `/models`（404 可忽略），再 POST `/responses`，但这只能证明 Responses 网关可用，不等于 Codex CLI 本体可用。

本 skill 因此分两级验证：
- 轻量 E2E：直接 POST `/v1/responses`。
- 强验证：可选运行真实 `codex exec`，并用 usage log 证明目标账号被命中。
