# ChatGPT OAuth Experimental Image Generation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 为 Sub2API 增加“实验性 ChatGPT OAuth 生图”能力，使 OpenAI OAuth 账号在明确受控、可观测、可回滚的前提下尝试承接 `gpt-image-2` 图片生成请求。

**Architecture:** 不直接把 OAuth 生图混入现有 OpenAI API Key 图片链路，而是增加一条独立的 OAuth Images 实验分支，由 feature flag 显式开启。实现上优先复用现有 OpenAI Images handler、调度、计费、usage 记录与错误透传框架，只在“账号筛选、上游端点解析、OAuth 请求头构造、响应/usage 归一化”几个点引入 OAuth 专用逻辑。整个特性按“先探测与证据固化 → 仅支持 `/v1/images/generations` → 再评估 `/v1/images/edits`”三段推进。

**Tech Stack:** Go 1.26+、Gin、现有 `OpenAIGatewayHandler` / `OpenAIGatewayService`、OpenAI OAuth TokenProvider、Zap 日志、现有图片计费与 usage 记录链路。

**Minimal Change Scope:** 仅允许修改 `backend/internal/handler/openai_images.go`、`backend/internal/service/openai_gateway_images.go`、`backend/internal/service/openai_gateway_service.go`、`backend/internal/service/account.go`、`backend/internal/config/`、相关测试文件、必要的运维/用户文档与 `docs/plans/`。避免修改现有 API Key 图片路由语义、Responses OAuth 主链路、前端大范围表单行为、数据库 schema。

**Success Criteria:** 开启实验开关后，满足条件的 OpenAI OAuth 账号可以稳定承接至少 `POST /v1/images/generations` 的非流式请求；失败时能明确区分“上游不支持 / 本地配置关闭 / 账号不满足条件 / 风控拦截”；关闭开关后行为完全回退到当前“仅 API Key 支持图片”的状态；API Key 图片链路无回归。

**Verification Plan:** `cd backend && go test -tags=unit ./internal/service ./internal/handler -run 'OpenAI.*Image|OAuth.*Image|Images'`；本地或远程只读探测脚本对目标 OAuth 账号执行 1 次 capability probe；手工验证 4 条链路：OAuth+flag off、OAuth+flag on+probe fail、OAuth+flag on+probe pass、API Key 既有图片请求。

---

## 背景与当前状态

- 当前图片路由已存在，入口为 `POST /v1/images/generations` 与 `POST /v1/images/edits`。
- 当前图片 handler 在账号选择后会显式跳过非 `API Key` 账号，直接导致 OAuth 永远不会参与图片调度。
- 当前图片 service 也在转发入口处对 `account.Type != AccountTypeAPIKey` 直接报错。
- 当前 OpenAI OAuth 文本链路并不走 `api.openai.com/v1/responses` 标准开发者 API，而是走 `https://chatgpt.com/backend-api/codex/responses` 与一组 OAuth 专属请求头。
- 社区经验表明：ChatGPT OAuth 生图“可能可行”，但更像走 Codex/ChatGPT internal 链路，而非稳定公开的 Images API；该前提决定了此特性必须定义为实验性，而不是默认能力。

## 关键假设

1. 某些 ChatGPT OAuth 账号在当前时间点确实存在可工作的图片生成上游端点，但该端点可能不是标准 `api.openai.com/v1/images/*`。
2. 即便上游可用，不同订阅层级、地区、组织形态、风控状态下，支持度也可能不同，因此必须把“是否可用”从静态账号类型判断升级为“动态 capability probe + 短期缓存”。
3. `images/edits` 的 multipart 语义和上游风控复杂度高于 `images/generations`，不应和 MVP 同期强绑。

## 非目标

- 不在第一期承诺支持 `POST /v1/images/edits` 的 OAuth 生图。
- 不承诺官方稳定支持；文档必须明确这是 experimental capability。
- 不修改数据库 schema，不引入新的持久化表。
- 不把 `/v1/models` 默认模型列表扩展成“正式公开 OAuth 图片模型目录”；仅在实验说明、测试与 capability 结果中体现。

## 风险清单

### 风险 1：OAuth 上游并非标准 Images API

- 表现：`/v1/images/generations` 映射到 `api.openai.com` 返回 403/404/unsupported。
- 应对：在实施前先加只读 probe，支持多个候选上游策略，但默认只启用通过 probe 的策略。

### 风险 2：ChatGPT internal 需要额外头或会话字段

- 表现：同一 token 在 Responses 可用，但 Images 返回 401/403/500。
- 应对：把 OAuth Images 请求构造独立封装，不复用 API Key request builder；记录最小必要头白名单与拒绝原因。

### 风险 3：usage 字段与标准 Images API 不一致

- 表现：成功返回图片，但无 `usage` 或字段结构不同，导致计费/usage 记录异常。
- 应对：先做归一化提取器，允许“有图但无 usage”进入受控降级路径，并打结构化日志；必要时仅允许实验账号走“先成功转发、后弱计费”。

### 风险 4：多账号调度随机命中不可用 OAuth 账号

- 表现：同组里部分 OAuth 账号可生图、部分不可，调度抖动严重。
- 应对：引入短 TTL capability cache；调度只在 probe pass 的 OAuth 账号集合内选择。

### 风险 5：误伤现有 API Key 图片链路

- 表现：改动后 API Key 图片请求行为变化。
- 应对：所有 OAuth 支持放在 feature flag + account type 分支内；现有 API Key 路径保持默认优先且逻辑不重排。

## 配置与发布策略

### 开关分层

- 全局开关：`gateway.openai_oauth_images_experimental_enabled`
- 可选账号级开关：`accounts.extra.openai_oauth_images_experimental`
- 可选候选上游策略：`chatgpt_codex_responses_tool`、`chatgpt_internal_images`、`api_platform_images_with_oauth`

### 发布策略

1. 第一期默认关闭，仅开发/测试环境开启。
2. 第二期只对白名单账号或手动开启账号生效。
3. 第三期在 probe 命中率和成功率稳定后，考虑开放更多账号。

### 回滚策略

- 关闭全局开关后，图片调度立即恢复“仅 API Key”。
- 保留 probe 与诊断日志，但不再选择 OAuth 账号。
- 不依赖数据库迁移，因此回滚不需要数据修复。

## 依赖顺序

1. 先做 capability probe 与配置开关。
2. 再做 OAuth 图片请求构造与最小 generation 转发。
3. 再做 usage / 计费归一化与调度缓存。
4. 最后才评估 edits、多策略选择和 UI 暴露。

## 实施阶段

### Phase 0：证据固化与探测脚手架

目标：先把“哪个 OAuth 上游端点真实可用”从社区经验变成仓库内可重复验证的事实。

### Phase 1：仅支持 OAuth `/v1/images/generations` 非流式 MVP

目标：在开关开启且 probe 成功时，允许 OAuth 账号参与 generation 调度。

### Phase 2：增强观测、失败回退与 usage 归一化

目标：让失败有可解释性，成功可计量，调度不抖动。

### Phase 3：评估 `/v1/images/edits` 与流式 generation

目标：只有在 Phase 1/2 稳定后，才进入高复杂度链路。

## 任务清单

### Task 1: 新增 OAuth Images 实验开关与配置读取

**Files:**
- Modify: `backend/internal/config/`
- Modify: `backend/internal/service/account.go`
- Test: `backend/internal/service/account_openai_passthrough_test.go` 或新增 `backend/internal/service/account_openai_images_experimental_test.go`

**Step 1: 定义全局配置项**

- 在网关配置中新增 `OpenAIOAuthImagesExperimentalEnabled` 布尔字段。
- 保持默认值为 `false`。

**Step 2: 定义账号级开关读取**

- 在 `Account` 上增加 `IsOpenAIOAuthImagesExperimentalEnabled()`。
- 语义：只有 `PlatformOpenAI + AccountTypeOAuth` 且 `extra.openai_oauth_images_experimental=true` 时才返回 true。
- 若未配置账号级开关，可允许“仅由全局开关控制”的模式，但要在实现里统一判断，避免歧义。

**Step 3: 补充单元测试**

Run: `cd backend && go test -tags=unit ./internal/service -run 'OpenAI.*Images.*Experimental|Account.*OpenAI'`
Expected: PASS

### Task 2: 建立 OAuth Images capability probe 与缓存接口

**Files:**
- Create: `backend/internal/service/openai_oauth_images_probe.go`
- Create: `backend/internal/service/openai_oauth_images_probe_test.go`
- Modify: `backend/internal/service/openai_gateway_images.go`
- Optional Modify: `backend/internal/service/openai_gateway_service.go`

**Step 1: 定义 probe 结果结构**

建议结构：

```go
type OpenAIOAuthImagesCapability struct {
    Supported bool
    Strategy  string
    CheckedAt time.Time
    TTL       time.Duration
    Status    int
    Reason    string
}
```

**Step 2: 定义候选策略枚举**

- `chatgpt_codex_responses_tool`
- `chatgpt_internal_images`
- `api_platform_images_with_oauth`

要求：
- 策略是显式枚举，不允许用自由字符串散落在 handler/service。

**Step 3: 实现只读 probe**

- 优先使用最轻量请求。
- 必须避免真实图片编辑写入。
- generation probe 可使用最小 prompt、最低成本参数，或优先走“不产生成本”的 capability/validation 路径；若上游不存在这种路径，则在测试环境用极小成本真实请求，并把成本告警写进文档。

**Step 4: 增加短 TTL 缓存**

- 建议先用内存或现有 cache 抽象，不新增表。
- TTL 建议 5-15 分钟。

**Step 5: 测试矩阵**

- probe success
- probe 403 unsupported
- probe 404 endpoint not found
- probe 5xx temporary unavailable
- cache hit / cache expire

Run: `cd backend && go test -tags=unit ./internal/service -run 'OAuth.*Images.*Probe'`
Expected: PASS

### Task 3: 抽象 OpenAI Images 上游 URL 解析，支持 OAuth 专用分支

**Files:**
- Modify: `backend/internal/service/openai_gateway_images.go`
- Test: `backend/internal/service/openai_gateway_images_test.go`

**Step 1: 拆分当前 URL 构造逻辑**

把当前 `buildOpenAIImagesURL(account, endpoint)` 改成两层：

- `resolveOpenAIImagesUpstream(account, endpoint, strategy)`
- `buildOpenAIImagesRequest(...)`

**Step 2: 保留 API Key 现有行为完全不变**

- `API Key` 仍走 `api.openai.com` 或 `account.base_url`
- 相关既有测试必须保持通过

**Step 3: 为 OAuth 预留专用目标地址逻辑**

- 根据 probe strategy 决定目标地址
- 若 strategy 未命中，返回明确错误，不允许静默 fallback 到 API Key 语义

**Step 4: 补充 URL 单测**

- OAuth strategy A → 目标 URL A
- OAuth strategy missing → error
- API Key 路径无回归

Run: `cd backend && go test -tags=unit ./internal/service -run 'BuildOpenAI.*Image|ImagesURL'`
Expected: PASS

### Task 4: 新增 OAuth 图片请求构造器

**Files:**
- Modify: `backend/internal/service/openai_gateway_images.go`
- Optional Extract: `backend/internal/service/openai_oauth_images_request.go`
- Test: `backend/internal/service/openai_oauth_images_request_test.go`

**Step 1: 区分 API Key 与 OAuth 请求头构造**

- API Key：保持当前 `Authorization: Bearer <api_key>` 语义
- OAuth：使用 token provider 的 access token，并按 strategy 注入必要头

**Step 2: 最小头白名单**

至少明确以下字段是否需要：
- `authorization`
- `chatgpt-account-id`
- `originator`
- `accept`
- `accept-language`
- `user-agent`
- 可能的 beta/feature 头

**Step 3: 避免复用 Responses passthrough 的噪声头**

- 不直接整包复用文本 OAuth passthrough header whitelist
- 仅拷贝已验证对 Images 有意义的低风险头

**Step 4: 单元测试**

- OAuth request 必须包含必须头
- 不应透传入站 `Cookie` / `X-Api-Key`
- Host/URL/Content-Type 符合 strategy 约束

Run: `cd backend && go test -tags=unit ./internal/service -run 'OAuth.*Images.*Request'`
Expected: PASS

### Task 5: 允许符合条件的 OAuth 账号参与 generation 调度

**Files:**
- Modify: `backend/internal/handler/openai_images.go`
- Modify: `backend/internal/service/openai_gateway_images.go`
- Modify: `backend/internal/service/openai_account_scheduler.go`
- Test: `backend/internal/handler/openai_images_test.go`
- Test: `backend/internal/service/openai_gateway_images_test.go`

**Step 1: 放宽 handler 中的“仅 API Key”硬过滤**

当前逻辑：
- 非 `API Key` 一律 `skip_non_apikey_account`

改为：
- `API Key` 直接可用
- `OAuth` 仅在实验开关开启且 probe pass 时可用
- 其他类型继续跳过

**Step 2: 放宽 service 入口硬限制**

当前 `ForwardAsImageGeneration` 与 `ForwardAsImageEdit` 都会拒绝非 API Key。

改为：
- `ForwardAsImageGeneration` 支持 `OAuth` 实验分支
- `ForwardAsImageEdit` 暂时仍拒绝 `OAuth`，并返回明确 experimental unsupported error

**Step 3: 调度选择原则**

- 若组内同时存在 API Key 与 OAuth 且两者都支持图片，第一期默认优先 API Key
- 只有在明确配置“允许 OAuth 图片承载”或 API Key 不可用时，才选择 OAuth

说明：这是为了把实验性能力放在后备位，不抢当前稳定链路。

**Step 4: 覆盖测试**

- flag off：OAuth 仍被跳过
- flag on + probe fail：OAuth 被跳过并记录原因
- flag on + probe pass：OAuth 可被选中
- API Key 与 OAuth 同时存在：默认优先 API Key

Run: `cd backend && go test -tags=unit ./internal/handler ./internal/service -run 'OpenAI.*Images.*OAuth|Images.*Scheduler'`
Expected: PASS

### Task 6: 实现 OAuth generation 最小转发闭环

**Files:**
- Modify: `backend/internal/service/openai_gateway_images.go`
- Test: `backend/internal/service/openai_gateway_images_test.go`

**Step 1: 仅支持 `POST /v1/images/generations` 非流式**

第一期明确约束：
- `stream=true` 对 OAuth 返回 `400 invalid_request_error` 或 `501 not_supported_yet`
- `edits` 对 OAuth 返回 `501 experimental_not_enabled`

**Step 2: 透传成功响应并保留图片数据**

- 若上游返回标准 Images JSON，则沿用现有解析
- 若上游返回 Responses 风格包裹体，则增加归一化转换

**Step 3: 抽取最小 usage**

至少支持：
- `ImageCount`
- `Model`
- `BillingModel`
- 如可得则提取 `input_tokens` / `output_tokens` / `image_tokens`

**Step 4: 单元测试**

- 标准 Images 响应
- 非标准但可归一化响应
- 无 usage 但有图片数据的响应

Run: `cd backend && go test -tags=unit ./internal/service -run 'ForwardAsImageGeneration.*OAuth|ParseOpenAIImagesUsage'`
Expected: PASS

### Task 7: 明确 usage/计费降级策略

**Files:**
- Modify: `backend/internal/handler/openai_images.go`
- Modify: `backend/internal/service/openai_gateway_images.go`
- Modify: `backend/internal/service/billing_service.go`（仅在必要时）
- Test: `backend/internal/service/billing_service_image_test.go`

**Step 1: 定义实验期计费原则**

优先顺序建议：
1. 使用上游返回的真实 image/text token usage
2. 若无 usage，但 probe strategy 明确且返回图片成功，可走“保守默认计费”或“拒绝记账并记风险日志”
3. 禁止静默套用无关文本模型价格

**Step 2: 防止错误 fallback 到文本模型价格**

必须专门保护：
- `gpt-image-2-2026-04-21`
- 非标准 OAuth 图片模型别名

要求：
- 若无合法图片模型定价，返回明确计费错误或进入配置化降级
- 不能回落到 `gpt-5.1-codex` 之类文本价格

**Step 3: 补充测试**

- 已知图片模型定价命中
- 未知 OAuth 图片别名不误落文本模型
- 无 usage 的降级路径可控

Run: `cd backend && go test -tags=unit ./internal/service -run 'Billing.*Image|Pricing.*Image'`
Expected: PASS

### Task 8: 观测、日志与错误分类

**Files:**
- Modify: `backend/internal/handler/openai_images.go`
- Modify: `backend/internal/service/openai_gateway_images.go`
- Test: `backend/internal/service/openai_gateway_images_test.go`
- Optional Doc: `docs/`

**Step 1: 新增结构化日志字段**

建议至少记录：
- `oauth_images_experimental`
- `oauth_images_strategy`
- `oauth_images_probe_supported`
- `oauth_images_probe_reason`
- `oauth_images_usage_mode` (`upstream` / `normalized` / `degraded`)

**Step 2: 新增错误分类**

- `oauth_images_experimental_disabled`
- `oauth_images_probe_failed`
- `oauth_images_upstream_unsupported`
- `oauth_images_stream_not_supported`
- `oauth_images_edits_not_supported`

**Step 3: 为运营排查保留低噪声日志**

- 首次失败记录 warn
- 同类重复失败在短时间内压缩

### Task 9: 文档与操作说明

**Files:**
- Modify: `README.md`（仅在功能实现后需要对外说明时）
- Modify: `README_EN.md`（若中文 README 更新）
- Create or Modify: `docs/chatgpt-oauth-images-experimental.md`
- Keep: `docs/plans/2026-04-30-chatgpt-oauth-image-generation-experimental-plan.md`

**Step 1: 写清实验性边界**

- 仅支持哪些账号
- 默认关闭
- 哪些端点支持
- 哪些场景不保证稳定

**Step 2: 写清启用步骤**

- 全局开关
- 账号级开关
- 如何查看 probe 成功/失败

**Step 3: 写清回滚步骤**

- 关闭开关
- 清理 capability cache（如有）

### Task 10: Phase 3 评估项，不在 MVP 当轮强制实现

**Files:**
- Future Modify: `backend/internal/service/openai_gateway_images.go`
- Future Test: `backend/internal/service/openai_gateway_images_stream_test.go`

**Step 1: 评估 OAuth `stream=true`**

- 只有在上游事件格式被真实捕获并固定后才做

**Step 2: 评估 OAuth `images/edits`**

- 先确认 multipart 端点与必要头
- 未验证前不应实现

## 验证矩阵

| 场景 | 预期 |
|------|------|
| API Key + `/images/generations` | 维持当前成功行为 |
| OAuth + 全局开关关闭 | 不参与调度 |
| OAuth + 全局开关开启 + 账号开关关闭 | 不参与调度 |
| OAuth + 双开关开启 + probe fail | 不参与调度，并返回明确错误/日志 |
| OAuth + 双开关开启 + probe pass + non-stream generation | 可以成功转发 |
| OAuth + stream generation | 明确拒绝或标注未实现 |
| OAuth + edits | 第一阶段明确拒绝 |
| 未知图片别名 | 不误用文本价格 |

## 推荐实施顺序

1. Task 1
2. Task 2
3. Task 3
4. Task 4
5. Task 5
6. Task 6
7. Task 7
8. Task 8
9. Task 9
10. Task 10（仅评估，不承诺本轮实现）

## 建议验收门槛

- 单元测试全部通过
- 至少 1 个真实 OAuth 测试账号 probe pass 且成功生成 1 张图
- 至少 1 个真实 OAuth 测试账号 probe fail 并能给出明确诊断
- API Key 图片回归测试通过
- 关闭开关后的回滚验证通过

## 执行备注

- 这是高风险兼容特性，必须先让“不可用时也可解释”成立，再追求“可用时覆盖更多端点”。
- 若真实探测显示社区可行路径已失效，应在 Phase 0 结束时终止实现，不要硬写一个不可验证的 OAuth 图片分支。
- 若真实探测显示只有 `chatgpt_codex_responses_tool` 可用，则 MVP 应只支持该策略，不应过早抽象成多策略全支持。

Plan complete and saved to `docs/plans/2026-04-30-chatgpt-oauth-image-generation-experimental-plan.md`. Two execution options:

**1. Subagent-Driven (this session)** - I dispatch fresh subagent per task, review between tasks, fast iteration

**2. Parallel Session (separate)** - Open new session with executing-plans, batch execution with checkpoints

Which approach?
