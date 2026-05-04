# Upstream 489120 To Df722c Optimization Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 基于上游 `48912014a16e2dd1cfca8b7cad785d0e8e7bfeec..df722c9a6e97312491232c11bf305d5f93b45e04` 的 24 个 commit，选择性吸收对当前 fork 有价值的稳定性、计费和运维优化。

**Architecture:** 不直接 cherry-pick 上游区间，因为本地 `main` 已与上游 `df722c9a` 分叉，且本地已经有 `chatapi`、插件、风控、代理质量等自有演进。实施时按主题移植：先补测试刻画本地行为，再用最小改动合入兼容逻辑，最后跑后端单测、前端 typecheck/lint 与必要的 UI 截图审查。

**Tech Stack:** Go / Gin / Ent / PostgreSQL / Vue 3 / TypeScript / Pinia / Tailwind CSS / Vite / pnpm。

**Minimal Change Scope:** 允许后续实施修改 `backend/internal/handler`、`backend/internal/service`、`backend/internal/pkg/apicompat`、`backend/internal/pkg/openai_compat`、`backend/internal/repository/affiliate_repo.go`、`backend/migrations`、`frontend/src/components/account`、`frontend/src/views/admin`、`frontend/src/api/admin`、`frontend/src/router`、`frontend/src/i18n` 和相关测试；避免改动插件系统、支付主链路以外语义、README 体系和无关 UI 重构。

**Success Criteria:** 关键 OpenAI 请求不漏计、不误计；`/v1/chat/completions` 能按账号能力选择 Responses 转换或 Chat Completions 直转；OpenAI Images 流式/非流式 billing 元数据稳定；管理员可以审计邀请返利记录；批量编辑支持现有 OpenAI compact 配置；license 状态明确。

**Verification Plan:** 后端执行 `cd backend && go test -tags=unit ./internal/pkg/apicompat ./internal/service ./internal/handler/...`，涉及迁移时补跑相关 migration regression test；前端执行 `cd frontend && pnpm run typecheck && pnpm run lint:check && pnpm test -- BulkEditAccountModal`; UI 页面变更按 AGENTS.md 生成 `tmp/screenshots/run-{timestamp}/before.png`、`after.png`、`compare.png`。

---

## 上游区间变化概览

区间 `(version1, version2]` 包含 24 个 commit，整体 diff 为 62 个文件、5151 行新增、404 行删除。

### OpenAI 兼容网关与计费

- `4e4cc809`、`4d145300`、`adf01ac8`：为 OpenAI APIKey 账号增加 Responses API 能力探测，确认上游不支持 `/v1/responses` 时把入站 `/v1/chat/completions` 路由到上游 `/v1/chat/completions` 原生直转路径。
- `57099a6a`、`e736de1e`、`4cbf518f`：修复原生 Chat Completions 路径的 `reasoning_effort` 提取、上游端点日志、usage billing 保留。
- `72d5ee4c`：让 OpenAI compat 流式转换识别 `response.done` 终止事件，减少流式 usage 未被 drain 导致的漏计。
- `47fb38bc`：即使 usage 为 0，也保留 OpenAI usage log，避免请求审计断层。
- `23555be3`、`ff50b8b6`：修复 OpenAI WS passthrough 使用记录缺失 `reasoning_effort` 和 `User-Agent` 的问题。
- `df722c9a`：移除 OpenAI unknown model fallback，避免未知模型被静默映射为默认模型并导致计费或响应口径偏差。

### OpenAI Images

- `b2bdba78`：增强图片请求处理。只有上游确认为 event-stream 时才按流式处理；流式路径在没有 SSE data 时尝试从 fallback JSON body 中抽取 usage 和可计费图片数；补充 `usage.images`、`tool_usage.image_gen.images` 和 completed event 中 `b64_json`/`url` 的计数兜底。

### 管理端返利记录

- `6a41cf6a`、`0a914e03`、`650ddb2e`、`3ab40269`、`0b84d12d`、`76e2503d`：新增管理员返利邀请、返利、转余额记录页和 API；修复成熟返利额度统计、用户跳转、转入余额历史展示和审计来源；新增上游迁移 `134_affiliate_ledger_audit_snapshots.sql`。

### 前端批量编辑

- `3953dc9c`、`c129825f`：账号批量编辑弹窗新增 OpenAI compact mode 和 compact model mapping 批量配置，测试覆盖提交 payload。

### 版本与 license

- `d9e68f2c`：上游 `backend/cmd/server/VERSION` 同步到 `0.1.122`，对本 fork 的 `v1.3.x` 版本线没有直接吸收价值。
- 本区间未修改 `LICENSE`。本地 `LICENSE` 与上游 `df722c9a` 仅存在文件末尾换行差异，不构成 license 变更；本轮不需要同步 license。

## 本地项目启发与吸收结论

需要吸收，但不能整包吸收。当前 fork 已经有本地专属能力，尤其是 `chatapi` 账号类型、OpenAI 图片网关、账号批量编辑、邀请返利和更完整的运营体系；上游区间更适合作为“问题清单和测试样本库”来按主题移植。

### P0 必须优先吸收

- **OpenAI usage 为 0 也应写审计日志。** 本地 `OpenAIGatewayService.RecordUsage` 当前会跳过全 0 token usage，这会让成功但上游未返回用量的请求在运营审计中消失。应改为保留日志但保持计费金额为 0，并确保余额、订阅和 API Key quota 不产生错误扣减。
- **`response.done` 终止事件兼容。** 本地 `apicompat` 目前主要识别 `response.completed/incomplete/failed`，需要补上 `response.done`，减少 WS/Realtime 或透传路径中流式终止事件别名造成的 usage 漏计。
- **OpenAI Chat Completions 直转计费完整性审计。** 本地已有 `chatapi` 直转路径，但仍应借鉴上游 raw CC 测试，确认 `reasoning_effort`、`service_tier`、`stream_options.include_usage`、上游端点记录和 raw usage billing 都被保留。

### P1 建议吸收

- **OpenAI APIKey Responses 能力探测。** 本地已经用 `chatapi` 账号类型解决“只支持 `/v1/chat/completions` 的上游”问题，但对历史 OpenAI APIKey 账号和第三方 base_url 仍可以增加非阻塞能力探测或迁移提示。推荐先实现只读/后台探测标记，不改变未探测账号的既有行为。
- **OpenAI Images 流式稳健性。** 本地 `openai_gateway_images.go` 对 `reqStream` 直接进入流式处理，且流式解析主要依赖 SSE data。建议吸收上游的 content-type 判断、非 SSE fallback body 解析和图片计数兜底，防止图片生成成功但 billing metadata 缺失。
- **移除或收窄 unknown model fallback。** 本地 `openai_codex_transform.go` 仍存在未知模型默认回落到 `gpt-5.1` 的逻辑。建议先用测试定位哪些 fallback 是用户体验兜底、哪些会污染计费模型，然后只移除计费和上游选择链路中的静默 fallback。
- **批量编辑 OpenAI compact 配置。** 本地已经有 compact 相关后端能力和批量编辑弹窗，缺少上游新增的 compact mode / compact model mapping 批量配置入口。建议吸收，适合多 OpenAI OAuth 账号运维。

### P2 可择机吸收

- **管理员返利记录页。** 本地已有邀请返利基础能力和管理员专属用户设置页，但缺少按邀请、返利、转余额拆分的记录审计页面。若当前个人站点已经启用邀请返利，应吸收；若返利量很低，可排在 OpenAI 计费稳定性之后。
- **上游版本号同步。** 不吸收。当前 fork 的版本线由本项目配置和 tag 管理，上游 `0.1.122` 不适用于本地 `v1.3.x`。

## 实施任务

### Task 1: 保留 OpenAI 零用量审计日志

**Files:**
- Modify: `backend/internal/service/openai_gateway_service.go`
- Test: `backend/internal/service/openai_gateway_record_usage_test.go`

**Steps:**
1. 新增失败测试：`RecordUsage` 输入全 0 token 时仍调用 usage repo，`ActualCost` 为 0，billing repo/API Key quota/用户余额不错误扣减。
2. 移除或重构 `RecordUsage` 中全 0 usage 的早返回。
3. 确认非 0 usage 的余额、订阅、API Key quota 行为不变。
4. Run: `cd backend && go test -tags=unit ./internal/service -run 'Test.*RecordUsage'`

### Task 2: 补齐 `response.done` 流式终止兼容

**Files:**
- Modify: `backend/internal/pkg/apicompat/responses_to_anthropic.go`
- Modify: `backend/internal/pkg/apicompat/responses_to_chatcompletions.go`
- Modify: `backend/internal/pkg/apicompat/types.go`
- Test: `backend/internal/pkg/apicompat/anthropic_responses_test.go`
- Test: `backend/internal/pkg/apicompat/chatcompletions_responses_test.go`

**Steps:**
1. 为 Anthropic 和 Chat Completions 两条转换链新增 `response.done` 测试。
2. 将 `response.done` 纳入 completed 类终止事件处理。
3. Run: `cd backend && go test -tags=unit ./internal/pkg/apicompat`

### Task 3: 审计并强化 OpenAI Chat Completions 直转路径

**Files:**
- Modify: `backend/internal/service/openai_gateway_chat_completions.go`
- Modify: `backend/internal/handler/openai_chat_completions.go`
- Test: `backend/internal/service/openai_gateway_chat_completions_test.go`
- Test: `backend/internal/service/openai_gateway_record_usage_test.go`

**Steps:**
1. 用本地 `chatapi` 直转路径补齐上游 raw CC 场景测试：stream usage、非 stream usage、`reasoning_effort`、`service_tier`、上游 endpoint 和 User-Agent。
2. 对比上游 `openai_gateway_chat_completions_raw.go`，只移植本地缺失的计费和日志字段，不引入与 `chatapi` 账号类型冲突的新路径。
3. 如果要增加 APIKey Responses 能力探测，先只写 `extra.openai_responses_supported` 并保持 unknown 走旧行为。
4. Run: `cd backend && go test -tags=unit ./internal/service ./internal/handler/... -run 'OpenAI|ChatCompletions|RecordUsage'`

### Task 4: 强化 OpenAI Images 流式解析与计数

**Files:**
- Modify: `backend/internal/service/openai_gateway_images.go`
- Test: `backend/internal/service/openai_gateway_images_test.go`

**Steps:**
1. 新增测试：请求 `stream=true` 但上游返回 JSON 时，网关不误按 SSE 处理，并能抽取 usage 和图片数量。
2. 新增测试：SSE completed event 包含 `usage.images`、`tool_usage.image_gen.images`、`b64_json` 或 `url` 时能得到正确 `ImageCount`。
3. 移植上游的 event-stream guard 和 fallback body 解析，但保持本地 OAuth images capability probe 逻辑不变。
4. Run: `cd backend && go test -tags=unit ./internal/service -run 'Images|OpenAIImages'`

### Task 5: 批量编辑支持 OpenAI compact 配置

**Files:**
- Modify: `frontend/src/components/account/BulkEditAccountModal.vue`
- Modify: `frontend/src/components/account/__tests__/BulkEditAccountModal.spec.ts`
- Modify: `frontend/src/i18n/locales/zh.ts`
- Modify: `frontend/src/i18n/locales/en.ts`

**Steps:**
1. 在批量编辑弹窗中为 OpenAI OAuth/APIKey passthrough-capable 账号增加 compact mode 和 compact model mapping 控件。
2. 提交 payload 时把 `extra.openai_compact_mode` 与 `credentials.compact_model_mapping` 分别放到既有结构中。
3. 补齐 i18n 和 Vitest。
4. UI 修改前后按 AGENTS.md 生成截图对比。
5. Run: `cd frontend && pnpm test -- BulkEditAccountModal && pnpm run typecheck && pnpm run lint:check`

### Task 6: 评估并落地管理员返利记录页

**Files:**
- Modify: `backend/internal/handler/admin/affiliate_handler.go`
- Modify: `backend/internal/service/affiliate_service.go`
- Modify: `backend/internal/repository/affiliate_repo.go`
- Create: `backend/migrations/127_affiliate_ledger_audit_snapshots.sql`
- Modify: `backend/internal/server/routes/admin.go`
- Create/Modify: `frontend/src/api/admin/affiliates.ts` 或本地等价 admin API 文件
- Create/Modify: `frontend/src/views/admin/affiliates/*` 或并入现有 `frontend/src/views/admin/AffiliateView.vue`
- Modify: `frontend/src/router/index.ts`
- Modify: `frontend/src/components/layout/AppSidebar.vue`
- Modify: `frontend/src/i18n/locales/zh.ts`
- Modify: `frontend/src/i18n/locales/en.ts`

**Steps:**
1. 先确认本地 `user_affiliate_ledger` 和 `payment_audit_logs` 字段与上游迁移差异，迁移编号使用本地下一号 `127`，不要照搬上游 `134`。
2. 后端新增只读列表接口：邀请记录、返利入账、转余额记录，支持分页、用户跳转所需字段和审计来源。
3. 前端优先复用现有 `AffiliateView.vue` 管理页样式，避免为低频功能做过度页面拆分。
4. 补充 repository/service 测试和迁移 regression test。
5. UI 修改前后按 AGENTS.md 生成截图对比。

## 不吸收项

- 不同步上游 `backend/cmd/server/VERSION` 到 `0.1.122`。
- 不改 `LICENSE`：该上游区间没有 license 文本变更。
- 不直接 cherry-pick 合并 commit，因为本地 fork 在调度、插件、风控、版本线和 OpenAI 账号类型上已有明显分叉。

