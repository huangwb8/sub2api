# OpenAI GPT Image 2 Upstream Absorption Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 充分吸收上游 `Wei-Shaw/sub2api` 对 `gpt-image-2` 的 OpenAI Images API、OAuth Responses 桥接和 Codex 图片工具支持，让本 fork 能稳定通过 Sub2API 调用 `gpt-image-2` 生成图片。

**Architecture:** 以本 fork 已有 `/v1/images/generations` API Key 图片链路为底座，选择性吸收上游 `backend/internal/service/openai_images.go`、`openai_images_responses.go` 和 Codex transform 的核心逻辑。API Key 账号继续直连 `/v1/images/*`，OAuth 账号不再依赖单独实验开关，而是按上游方式把 Images 请求转换为 `/v1/responses` + `image_generation` tool，再把 SSE/Responses 图片结果归一化回 Images API 响应。

**Tech Stack:** Go 1.26+ / Gin / Ent / Redis / OpenAI-compatible HTTP APIs / OpenAI Responses `image_generation` tool / pnpm 前端类型同步。

**Minimal Change Scope:** 允许修改 `backend/internal/handler/openai_images.go`、`backend/internal/service/openai_images*.go`、`backend/internal/service/openai_codex_transform.go`、OpenAI 调度/模型映射/计费相关测试、`backend/resources/model-pricing/model_prices_and_context_window.json`、必要的前端账号测试展示与 `docs/` 用户说明。避免改动支付、插件、非 OpenAI 平台网关、无关 UI 重构和历史计划文档。

**Success Criteria:** `POST /v1/images/generations` 使用 `gpt-image-2` 可在 OpenAI API Key 账号直连成功；OpenAI OAuth 账号可通过 Responses `image_generation` 桥接生成图片；Codex `/v1/responses` 中 image-only 请求会被规范化为文本主模型 + `image_generation` tool；`gpt-5.3-codex-spark` 继续明确拒绝图片能力；usage 记录能保存图片 token、图片数量、尺寸和上游模型；失败时可 failover 且不误封可用账号。

**Verification Plan:** 运行 `cd backend && go test -tags=unit ./internal/service -run 'OpenAI.*Image|Codex.*Image|ModelMapping|Pricing'`、`cd backend && go test -tags=unit ./internal/handler -run 'OpenAI.*Image|Endpoint'`、`cd backend && go test -tags=unit ./internal/server -run 'Images|Gateway'`；用真实只读/低成本环境执行 `python3 tmp/gpt-image-2-probe/test_gpt_image_2.py --model gpt-image-2`，并至少覆盖 API Key 与 OAuth 两类账号。

---

## 背景判断

上游 main `4de28fec8c061ee5f0bad93e885c07fced41c864` 已经把 `gpt-image-2` 作为 OpenAI 默认模型列表和 Images 默认模型的一部分，并注册了 `/v1/images/generations`、`/v1/images/edits` 以及无 `/v1` 别名。

本 fork 当前已经有 OpenAI Images API 支持和一个受 `gateway.openai_oauth_images_experimental_enabled` 控制的 OAuth 实验分支。真实探测显示本地 Codex key 命中了 `/v1/images/generations`，但返回 `OpenAI OAuth experimental image generation is disabled`，说明当前阻塞点是 OAuth 图片分支开关/实现策略，而不是 base URL 后缀。

社区反馈“可以用”更接近上游新版语义：API Key 账号走 Images 直连；OAuth 账号走 Responses `image_generation` tool 桥接；Codex 客户端的图片请求需要 transform 层注入或规范化 `image_generation` tool。

## 上游能力清单

- `backend/internal/server/routes/gateway.go`
  - 注册 `/v1/images/generations`、`/v1/images/edits`。
  - 注册 `/images/generations`、`/images/edits` 别名。
  - 只允许 OpenAI 平台分组进入 Images handler。

- `backend/internal/service/openai_images.go`
  - 定义 `OpenAIImagesRequest` 和 `OpenAIImagesCapability`。
  - 默认模型为 `gpt-image-2`。
  - 允许所有 `gpt-image-*`。
  - API Key 账号通过 `buildOpenAIImagesURL` 直连上游 Images endpoint。
  - 根据响应是否 SSE 处理流式与非流式结果。

- `backend/internal/service/openai_images_responses.go`
  - 将 Images generation/edit 转成 Responses 请求。
  - 使用 `gpt-5.4-mini` 作为 Responses 主模型。
  - 在 `tools` 中添加 `{"type":"image_generation","model":"gpt-image-2"}`。
  - 将 Responses 中的 `image_generation_call.result` 归一化为 Images API 的 `b64_json` / data URL / SSE 事件。

- `backend/internal/service/openai_codex_transform.go`
  - Codex CLI 请求可自动注入 `image_generation` tool。
  - image-only `model: gpt-image-2` 请求会被改写为 Responses-capable 文本模型 + 图片工具模型。
  - `gpt-5.3-codex-spark` 会保留明确的图片不支持提示。

- `backend/resources/model-pricing/model_prices_and_context_window.json`
  - 已包含 `gpt-image-2` 或至少包含 `gpt-image-*` 图片模型定价/endpoint 信息。

## 本 fork 现状差异

- 已有 `backend/internal/handler/openai_images.go`，但入口仍调用 `ForwardAsImageGeneration` / `ForwardAsImageEdit` 分支，而上游 main 已统一为 `Images(c)` + `ForwardImages`。
- 已有 `backend/internal/service/openai_gateway_images.go`，API Key 图片直连能力基本存在。
- 已有 `backend/internal/service/openai_oauth_images_probe.go`，OAuth 图片能力被全局 feature flag、账号 extra 和 probe 字段共同控制。
- 上游 main 已不再依赖这套显式实验开关，而是把 OAuth Images 作为正式桥接路径处理。
- 本 fork 的 Codex transform 已识别 `gpt-image-*`，但缺少上游更完整的 image-only Responses 规范化与 tool 注入逻辑。

## 非目标

- 不在本轮扩展非 OpenAI 平台的图片接口。
- 不新增新的支付、套餐或用户权限策略。
- 不承诺绕过 Cloudflare/Arkose 风控；只做错误分类、failover 和运维可观测。
- 不把 `gpt-5.3-codex-spark` 强行变成可生图模型。
- 不重写整个 OpenAI 网关，只吸收图片链路所需的最小上游能力。

## 迁移策略

优先采用“兼容吸收”而不是整文件覆盖。原因是本 fork 已经有大量本地特性：ChatAPI 账号、Ops 监控、调度规则、错误透传、用量审计、住宅代理计量、插件注入等。直接覆盖上游文件容易抹掉本地增强。

推荐顺序：

1. 先补测试，固定当前 API Key 成功路径和 OAuth 关闭失败路径。
2. 引入上游 Images request 解析和 Responses 桥接 helper。
3. 将 OAuth Images 从实验开关路径升级为正式桥接路径，但保留可回滚配置。
4. 吸收 Codex image-only 请求规范化。
5. 最后统一计费、usage、模型定价和文档。

## Task 1: 上游差异快照与保护性测试

**Files:**
- Modify: `backend/internal/service/openai_gateway_images_test.go`
- Modify: `backend/internal/service/openai_oauth_images_probe_test.go`
- Modify: `backend/internal/handler/openai_images_test.go`
- Modify: `backend/internal/server/routes/gateway_test.go`

**Step 1: 固定当前 API Key Images 行为**

新增或确认测试覆盖：

```go
func TestForwardAsImageGeneration_APIKey_GPTImage2(t *testing.T) {
    // 入站 model=gpt-image-2, prompt=draw a cat
    // 期望上游 URL 为 /v1/images/generations
    // 期望 Authorization 使用 API Key 账号 token
    // 期望响应透传 data[0].b64_json
}
```

Run:

```bash
cd backend
go test -tags=unit ./internal/service -run 'ForwardAsImageGeneration.*GPTImage2|OpenAI.*Images'
```

Expected: PASS，证明吸收前 API Key 路径不能回归。

**Step 2: 固定当前 OAuth 阻塞语义**

新增测试确认当前关闭开关时返回 `oauth_images_experimental_disabled`。后续 Task 4 会把这个测试改为“默认桥接可用 / 配置禁用时关闭”。

Run:

```bash
cd backend
go test -tags=unit ./internal/service -run 'ValidateOpenAIImagesAccount|OAuth.*Images'
```

Expected: PASS。

**Step 3: 记录上游 commit 基线**

在计划执行 PR 或 commit message 中记录：

```text
Upstream baseline: Wei-Shaw/sub2api@4de28fec8c061ee5f0bad93e885c07fced41c864
Feature area: OpenAI Images gpt-image-2 + OAuth Responses image_generation bridge
```

## Task 2: 统一 Images 请求模型与解析能力

**Files:**
- Create or Replace: `backend/internal/service/openai_images.go`
- Modify: `backend/internal/pkg/apicompat/openai_images.go`
- Modify: `backend/internal/service/openai_gateway_images.go`
- Test: `backend/internal/service/openai_images_test.go`

**Step 1: 引入上游 `OpenAIImagesRequest` 结构**

需要包含字段：

```go
type OpenAIImagesRequest struct {
    Endpoint string
    ContentType string
    Multipart bool
    Model string
    ExplicitModel bool
    Prompt string
    Stream bool
    N int
    Size string
    ExplicitSize bool
    SizeTier string
    ResponseFormat string
    Quality string
    Background string
    OutputFormat string
    Moderation string
    InputFidelity string
    Style string
    OutputCompression *int
    PartialImages *int
    HasMask bool
    HasNativeOptions bool
    RequiredCapability OpenAIImagesCapability
    InputImageURLs []string
    MaskImageURL string
    Uploads []OpenAIImagesUpload
    MaskUpload *OpenAIImagesUpload
    Body []byte
}
```

保留本 fork 已有的 body limit、request logger、ops context 接线。

**Step 2: 默认模型和模型校验**

实现：

```go
func applyOpenAIImagesDefaults(req *OpenAIImagesRequest) {
    if req.N <= 0 {
        req.N = 1
    }
    if strings.TrimSpace(req.Model) == "" {
        req.Model = "gpt-image-2"
    }
}

func validateOpenAIImagesModel(model string) error {
    if strings.HasPrefix(strings.ToLower(strings.TrimSpace(model)), "gpt-image-") {
        return nil
    }
    return fmt.Errorf("images endpoint requires an image model, got %q", model)
}
```

**Step 3: 测试 JSON generation 解析**

测试：

```go
func TestParseOpenAIImagesRequest_GenerationGPTImage2(t *testing.T) {
    body := []byte(`{"model":"gpt-image-2","prompt":"draw a cat","size":"1024x1024","quality":"high"}`)
    // Expected: Endpoint=/v1/images/generations, Model=gpt-image-2, Prompt=draw a cat, SizeTier=1K
}
```

Run:

```bash
cd backend
go test -tags=unit ./internal/service -run 'ParseOpenAIImagesRequest'
```

Expected: PASS。

## Task 3: API Key Images 直连路径对齐上游

**Files:**
- Modify: `backend/internal/service/openai_gateway_images.go`
- Modify: `backend/internal/service/openai_images.go`
- Test: `backend/internal/service/openai_gateway_images_test.go`

**Step 1: 吸收 `buildOpenAIImagesURL`**

目标行为：

```go
buildOpenAIImagesURL("https://api.openai.com", "/v1/images/generations")
// https://api.openai.com/v1/images/generations

buildOpenAIImagesURL("https://example.com/v1", "/v1/images/generations")
// https://example.com/v1/images/generations

buildOpenAIImagesURL("https://example.com/v1/images/generations", "/v1/images/generations")
// https://example.com/v1/images/generations
```

**Step 2: 保留账号模型映射**

请求模型选择顺序：

1. 入站 `parsed.Model`
2. channel mapped model
3. account model mapping

最后写入上游请求体中的 `model` 必须是最终 upstream model。

**Step 3: 响应处理**

非流式响应：

- 原样写回 JSON。
- 从 `usage` 抽取 input/output/image token。
- 从 `data` 数组推断图片数。

流式响应：

- 仅当上游 `Content-Type` 是 event-stream 时按 SSE 处理。
- 如果上游返回 JSON fallback，不能误按流式吞掉错误。

Run:

```bash
cd backend
go test -tags=unit ./internal/service -run 'ForwardAsImageGeneration|OpenAIImagesURL|OpenAIImages.*Response'
```

Expected: PASS。

## Task 4: OAuth Images 正式桥接路径

**Files:**
- Create: `backend/internal/service/openai_images_responses.go`
- Modify: `backend/internal/service/openai_gateway_images.go`
- Modify: `backend/internal/service/openai_oauth_images_probe.go`
- Modify: `backend/internal/config/config.go`
- Test: `backend/internal/service/openai_images_test.go`
- Test: `backend/internal/service/openai_oauth_images_probe_test.go`

**Step 1: 引入 `buildOpenAIImagesResponsesRequest`**

生成请求形态：

```json
{
  "model": "gpt-5.4-mini",
  "stream": true,
  "store": false,
  "tool_choice": {"type": "image_generation"},
  "tools": [
    {"type": "image_generation", "action": "generate", "model": "gpt-image-2"}
  ],
  "input": [
    {
      "type": "message",
      "role": "user",
      "content": [{"type": "input_text", "text": "draw a cat"}]
    }
  ]
}
```

其中主模型不要写成 `gpt-image-2`，否则 Responses 会报 image-only model 不适合作为主模型。

**Step 2: 保留回滚开关但改默认策略**

建议新增或复用配置：

```yaml
gateway:
  openai_oauth_images_enabled: true
```

兼容旧配置：

- 如果存在旧 `openai_oauth_images_experimental_enabled=true`，视为开启。
- 如果新配置显式 `false`，OAuth Images 关闭。
- 旧账号 extra `openai_oauth_images_*` 只作为诊断/灰度信息，不再作为硬性必要条件。

**Step 3: OAuth account eligibility**

选择 OAuth 账号时要求：

- `PlatformOpenAI`
- `AccountTypeOAuth`
- 可拿到 access token
- 没有处于 Codex/普通 rate limit
- 满足账号模型能力策略或 mapping 可映射到 `gpt-image-2`

不要再因为缺少 `openai_oauth_images_probe_supported=true` 直接拒绝。

**Step 4: 非流式响应归一化**

从 Responses SSE 中等待 `response.completed`，提取：

```json
{
  "type": "image_generation_call",
  "result": "<base64>",
  "revised_prompt": "draw a cat",
  "output_format": "png"
}
```

输出 Images API：

```json
{
  "created": 1710000000,
  "model": "gpt-image-2",
  "data": [
    {"b64_json": "<base64>", "revised_prompt": "draw a cat"}
  ],
  "usage": {}
}
```

**Step 5: 流式响应归一化**

把 Responses events：

- `response.image_generation_call.partial_image`
- `response.output_item.done`
- `response.completed`

转成 Images SSE：

- `image_generation.partial_image`
- `image_generation.completed`
- `[DONE]`

Run:

```bash
cd backend
go test -tags=unit ./internal/service -run 'OpenAIImages.*OAuth|ImagesResponses|ValidateOpenAIImagesAccount'
```

Expected: OAuth Images 测试从“开关关闭拒绝”转为“默认桥接成功；显式关闭才拒绝”。

## Task 5: Codex Responses 图片工具桥接

**Files:**
- Modify: `backend/internal/service/openai_codex_transform.go`
- Modify: `backend/internal/service/openai_gateway_service.go`
- Test: `backend/internal/service/openai_codex_transform_test.go`
- Test: `backend/internal/service/openai_gateway_service_test.go`

**Step 1: 自动注入 Codex 图片工具**

当请求来自 Codex CLI 且用户意图触发图片生成时，确保 `tools` 中包含：

```json
{"type": "image_generation"}
```

并注入简短 instructions：本地没有 `image_gen` namespace 不代表服务端不能生图，应使用 Responses native `image_generation` tool。

**Step 2: image-only model 规范化**

入站：

```json
{
  "model": "gpt-image-2",
  "prompt": "draw a cat"
}
```

规范化为：

```json
{
  "model": "gpt-5.4-mini",
  "input": "draw a cat",
  "tools": [{"type": "image_generation", "model": "gpt-image-2"}],
  "tool_choice": {"type": "image_generation"}
}
```

**Step 3: 保留 Spark 拒绝**

`gpt-5.3-codex-spark` 遇到图片输入、图片生成工具或 image-only 请求时，继续返回清晰错误，不能被 normalize 成可用路径。

Run:

```bash
cd backend
go test -tags=unit ./internal/service -run 'Codex.*Image|ResponsesImageModel|SparkImage'
```

Expected: PASS。

## Task 6: 调度、模型能力和分组限制

**Files:**
- Modify: `backend/internal/service/openai_account_scheduler.go`
- Modify: `backend/internal/service/openai_model_mapping.go`
- Modify: `backend/internal/pkg/openai/constants.go`
- Modify: `frontend/src/types/index.ts`
- Test: `backend/internal/service/openai_model_mapping_test.go`
- Test: `backend/internal/service/gateway_account_selection_test.go`

**Step 1: 默认模型列表包含 `gpt-image-2`**

确认：

```go
{ID: "gpt-image-2", Type: "model", DisplayName: "GPT Image 2"}
```

**Step 2: 历史账号模型白名单兼容**

对于采用“继承默认模型集”的账号，新增 `gpt-image-2` 后自动可调度。

对于显式 whitelist 账号，不自动越权；管理员需要手动加模型或配置 mapping。

**Step 3: chatapi 账号排除**

Images 和 Responses image_generation 不能调度到 `chatapi` 账号，避免只支持 Chat Completions 的上游被误用。

Run:

```bash
cd backend
go test -tags=unit ./internal/service -run 'ModelMapping|SelectAccount.*Images|ChatAPI.*Images'
```

Expected: PASS。

## Task 7: 计费与 usage 归一化

**Files:**
- Modify: `backend/internal/service/openai_gateway_record_usage.go`
- Modify: `backend/internal/service/billing_service.go`
- Modify: `backend/resources/model-pricing/model_prices_and_context_window.json`
- Modify: `skills/sub2api-summary/references/source-map.md`
- Test: `backend/internal/service/openai_gateway_record_usage_test.go`
- Test: `backend/internal/service/billing_service_image_test.go`

**Step 1: 定价资源确认**

确保 `gpt-image-2` 具备：

```json
{
  "mode": "image_generation",
  "litellm_provider": "openai",
  "supported_endpoints": ["/v1/images/generations", "/v1/images/edits"],
  "input_cost_per_token": 0,
  "input_cost_per_image_token": 0,
  "output_cost_per_image_token": 0
}
```

具体价格必须以实施当天 OpenAI 官方定价为准，不能沿用猜测值。

**Step 2: usage 字段归一化**

从以下来源抽取：

- Images API JSON `usage`
- Responses `response.usage`
- Responses `response.tool_usage.image_gen`
- SSE completed event 中的 `usage`

写入：

- `input_tokens`
- `output_tokens`
- `cache_read_input_tokens`
- `image_output_tokens`
- `image_count`
- `image_size`
- `model`
- `upstream_model`

**Step 3: 运营 skill 同步**

如果 usage schema 或统计口径变化，更新：

```text
skills/sub2api-summary/references/source-map.md
```

说明图片 usage 从 OpenAI Images 和 Responses image_generation 两条路径进入同一记录口径。

Run:

```bash
cd backend
go test -tags=unit ./internal/service -run 'RecordUsage.*Image|Billing.*Image|Pricing.*Image'
```

Expected: PASS。

## Task 8: 错误分类、failover 与风控可观测

**Files:**
- Modify: `backend/internal/service/openai_images.go`
- Modify: `backend/internal/service/openai_images_responses.go`
- Modify: `backend/internal/service/ops_account_availability.go`
- Modify: `docs/chatgpt-oauth-images-experimental.md`
- Test: `backend/internal/service/openai_images_test.go`

**Step 1: 错误分类**

至少区分：

- `invalid_request_error`: 请求参数、模型不是 `gpt-image-*`
- `authentication_error`: token/API key 不可用
- `rate_limit_error`: 上游限额
- `api_error`: Cloudflare/Arkose/上游 5xx
- `unsupported_image_generation`: Responses 不支持 image_generation tool

**Step 2: failover 策略**

可 failover：

- 429
- 500/502/503/504/52x
- 上游连接错误
- 临时 Cloudflare HTML/挑战页

不可盲目 failover：

- 400 参数错误
- 401 鉴权错误
- 明确模型不支持且同组账号同质化时

**Step 3: 运维可观测**

Ops 日志增加：

- inbound endpoint
- upstream endpoint
- request model
- upstream model
- account type
- whether OAuth bridge
- image response format
- image count
- upstream request ID

Run:

```bash
cd backend
go test -tags=unit ./internal/service -run 'OpenAIImages.*Failover|OpenAIImages.*Error'
```

Expected: PASS。

## Task 9: 文档与测试脚本

**Files:**
- Modify: `docs/chatgpt-oauth-images-experimental.md`
- Modify: `README.md`
- Modify: `README_EN.md`
- Modify: `README_JA.md`
- Modify: `tmp/gpt-image-2-probe/test_gpt_image_2.py`

**Step 1: 文档改名或重写**

`docs/chatgpt-oauth-images-experimental.md` 不应继续暗示 OAuth Images 只能实验性手工 probe。建议改成：

```text
docs/openai-images-gpt-image-2.md
```

或保留原文件但标题改为“OpenAI Images 与 ChatGPT OAuth 图片桥接”。

**Step 2: README 能力描述**

中文主 README 先更新，再同步英文/日文骨架：

- 支持 `/v1/images/generations`
- 支持 `gpt-image-2`
- API Key 账号直连
- OAuth 账号走 Responses image_generation 桥接
- 可能受上游风控影响

**Step 3: 测试脚本增强**

`tmp/gpt-image-2-probe/test_gpt_image_2.py` 增加两个模式：

```bash
python3 tmp/gpt-image-2-probe/test_gpt_image_2.py --mode images
python3 tmp/gpt-image-2-probe/test_gpt_image_2.py --mode responses-tool
```

`responses-tool` 请求体：

```json
{
  "model": "gpt-5.4-mini",
  "input": "draw a cat",
  "tools": [{"type": "image_generation", "model": "gpt-image-2"}],
  "tool_choice": {"type": "image_generation"}
}
```

## Task 10: 真实链路验收

**Files:**
- No source changes.
- Output: `tmp/gpt-image-2-probe/*.png`

**Step 1: API Key 账号验收**

Run:

```bash
OPENAI_BASE_URL="https://your-sub2api.example/v1" \
OPENAI_API_KEY="sk-..." \
python3 tmp/gpt-image-2-probe/test_gpt_image_2.py --mode images --model gpt-image-2 --prompt "draw a cat"
```

Expected:

- HTTP 200
- 生成 `gpt-image-2-cat-*.png`
- usage 记录写入图片模型
- 账号没有被误标记错误

**Step 2: OAuth 账号验收**

Run:

```bash
OPENAI_BASE_URL="https://your-sub2api.example/v1" \
OPENAI_API_KEY="sk-..." \
python3 tmp/gpt-image-2-probe/test_gpt_image_2.py --mode images --model gpt-image-2 --prompt "draw a cat"
```

分组内只保留或优先选择 OpenAI OAuth 账号。

Expected:

- 入站仍是 `/v1/images/generations`
- 上游实际为 `/v1/responses`
- 响应返回 Images API 兼容 JSON
- 图片文件可打开

**Step 3: Codex Responses 验收**

Run:

```bash
OPENAI_BASE_URL="https://your-sub2api.example/v1" \
OPENAI_API_KEY="sk-..." \
python3 tmp/gpt-image-2-probe/test_gpt_image_2.py --mode responses-tool --model gpt-5.4-mini --prompt "draw a cat"
```

Expected:

- Responses 输出包含 `image_generation_call`
- 脚本能提取 base64 并保存 PNG

## 回滚策略

- 保留 `gateway.openai_oauth_images_enabled=false` 可一键关闭 OAuth 桥接。
- API Key Images 直连不依赖 OAuth 开关，不能被回滚误伤。
- 如 OAuth 桥接导致风控升高，先关闭 OAuth Images，仅保留 API Key 图片。
- 若计费异常，先停止 `gpt-image-2` 模型调度或从分组模型列表移除，再修复 usage 归一化。
- 所有迁移不涉及数据库破坏性变更，优先通过配置和模型能力策略回滚。

## 最终验证矩阵

| 场景 | 账号类型 | Endpoint | 期望 |
|------|----------|----------|------|
| 猫图非流式 | API Key | `/v1/images/generations` | 200 + `data[0].b64_json` |
| 猫图流式 | API Key | `/v1/images/generations` + `stream=true` | SSE completed |
| 猫图非流式 | OAuth | `/v1/images/generations` | 200 + Images 兼容 JSON |
| 猫图流式 | OAuth | `/v1/images/generations` + `stream=true` | Images SSE 事件 |
| Responses tool | OAuth | `/v1/responses` | `image_generation_call.result` |
| image-only model | OAuth | `/v1/responses` + `model=gpt-image-2` | 自动改写为文本主模型 + tool |
| Spark 图片请求 | OAuth | `/v1/responses` + Spark | 400 明确不支持 |
| 非 OpenAI 分组 | 任意 | `/v1/images/generations` | 404 Images API not supported |
| 上游 429/5xx | 任意 | Images | failover 或限流标记 |
| 参数错误 | 任意 | Images | 400 不 failover |

## 推荐提交拆分

1. `test: cover current openai images gpt-image-2 behavior`
2. `feat: normalize openai images request parsing`
3. `feat: support openai oauth images via responses bridge`
4. `feat: bridge codex image generation requests`
5. `fix: align openai image usage billing`
6. `docs: document gpt-image-2 images support`

每个提交都应能独立通过对应单元测试，避免最后一次性合并导致排错困难。
