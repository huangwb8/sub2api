# OpenAI Images 与 ChatGPT OAuth 图片桥接

本文说明 Sub2API 对 OpenAI Images API、`gpt-image-2` 和 ChatGPT OAuth 图片桥接的当前支持方式。文件名保留 `experimental` 是为了兼容旧链接，正文口径以正式桥接能力为准。

## 能力边界

- OpenAI API Key 账号直连上游 `/v1/images/generations` 与 `/v1/images/edits`
- OpenAI OAuth 账号支持 `POST /v1/images/generations`，网关会转换为 ChatGPT Codex Responses `/v1/responses` + `image_generation` tool
- Images generation 未显式传入 `model` 时默认使用 `gpt-image-2`
- OAuth bridge 使用 `gpt-5.4-mini` 作为 Responses 主模型，图片工具模型保持为 `gpt-image-2`
- OAuth generation 支持非流式与 `stream=true`，返回会归一化为 Images API 兼容 JSON 或 Images SSE 事件
- OAuth bridge 当前仅支持 `n=1`；多图请求请拆成多次调用或使用 API Key Images 直连
- OAuth `POST /v1/images/edits` 暂不放开，API Key 账号仍可按 Images edits 直连
- `gpt-5.3-codex-spark` 明确不支持图片输入、图片生成和 `image_generation` tool

## 全局配置

默认开启 OAuth Images 桥接；如遇上游风控或临时故障，可显式关闭：

```yaml
gateway:
  openai_oauth_images_enabled: true
  openai_oauth_images_probe_ttl_seconds: 600
```

- `openai_oauth_images_enabled`
  - `true` 或未配置：允许 OAuth Images 通过 Responses `image_generation` tool 桥接
  - `false`：关闭 OAuth Images 桥接，仅保留 API Key Images 直连
- `openai_oauth_images_experimental_enabled`
  - 旧配置兼容字段；旧版本中设为 `true` 的部署不需要立即清理
  - 新配置 `openai_oauth_images_enabled=false` 优先生效
- `openai_oauth_images_probe_ttl_seconds`
  - OAuth 图片 capability 缓存 TTL，默认 `600`

## 账号 extra 字段

旧版账号 `extra.openai_oauth_images_*` 字段仍会被读取用于诊断和灰度记录，但不再作为硬性准入条件：

```json
{
  "openai_oauth_images_strategy": "chatgpt_codex_responses_tool",
  "openai_oauth_images_probe_supported": true,
  "openai_oauth_images_probe_reason": "manual_probe_passed",
  "openai_oauth_images_probe_status": 204
}
```

- `openai_oauth_images_strategy`
  - 推荐值为 `chatgpt_codex_responses_tool`
  - 旧值 `api_platform_images_with_oauth` 会兼容映射到 Responses bridge
- `openai_oauth_images_probe_supported`
  - 仅保留为诊断信息，不再要求必须为 `true`
- `openai_oauth_images_probe_reason` / `openai_oauth_images_probe_status`
  - 用于记录历史人工 probe 或排查摘要

## 调度行为

- 同组同时存在 API Key 与 OAuth 图片账号时，Images handler 默认优先尝试 API Key 账号
- API Key 不可用或被排除后，会回退到符合 OpenAI OAuth 基本条件的账号
- `chatapi` 账号不会作为 Images 或 Responses image generation 的目标账号
- OAuth bridge 请求的上游端点会记录为 `/v1/responses`，用量模型仍按图片工具模型 `gpt-image-2` 归一化

## 账号池配置教程

### 推荐账号池结构

建议把图片能力放在一个明确的 OpenAI 分组中，例如 `openai-images`：

- 至少放入一个 OpenAI API Key 账号，用于直连 Images API
- 可选放入一个或多个 OpenAI OAuth 账号，用于 API Key 不可用时通过 Responses bridge 回退
- 不要把 `chatapi` 账号放进图片分组；它只适合 Chat Completions 或显式开启的 Responses 兼容链路，不参与 Images 图片生成
- 如果分组启用了模型白名单，必须把 `gpt-image-2` 加入可用模型；继承默认模型集的 OpenAI 账号会自动包含 `gpt-image-2`

### 配置 API Key 账号

在管理后台进入“账号管理”，新增或编辑账号：

| 字段 | 建议值 |
|------|--------|
| 平台 | `openai` |
| 类型 | `apikey` |
| API Key | 上游 OpenAI API Key |
| Base URL | 官方账号可留空或填 `https://api.openai.com`；第三方兼容上游填自己的 OpenAI-compatible 根地址 |
| 分组 | 选择图片分组，例如 `openai-images` |
| 状态 | active / 可调度 |

对应 `credentials` 结构示例：

```json
{
  "api_key": "sk-...",
  "base_url": "https://api.openai.com"
}
```

API Key 账号会把入站 `/v1/images/generations` 直接转发到上游 Images endpoint。`/v1/images/edits` 也仅支持这类账号。

### 配置 OpenAI OAuth 账号

OpenAI OAuth 账号主要用于 ChatGPT/Codex 账号池。图片生成时，Sub2API 会把 Images 请求桥接成 ChatGPT Codex Responses 请求：

| 字段 | 建议值 |
|------|--------|
| 平台 | `openai` |
| 类型 | `oauth` |
| Access Token / Refresh Token | 按现有 OpenAI OAuth 接入流程保存 |
| ChatGPT Account ID | 如账号有多 workspace，建议保存 `chatgpt_account_id` |
| 分组 | 选择图片分组；新增 OAuth 账号未显式选择分组时，后端会默认加入所有活跃 OpenAI 分组 |
| 状态 | active / 可调度 |

常见 `credentials` 结构示例：

```json
{
  "access_token": "eyJ...",
  "refresh_token": "...",
  "chatgpt_account_id": "acct_..."
}
```

OAuth 图片桥接默认开启。需要临时关闭时，在后端配置中设置：

```yaml
gateway:
  openai_oauth_images_enabled: false
```

旧版 `extra.openai_oauth_images_*` 字段只作为诊断信息保留，不再要求手动设置 `openai_oauth_images_probe_supported=true`。

### 分组与模型映射检查

调用前建议确认三件事：

- 用户 API Key 绑定的分组包含可调度的 OpenAI API Key 或 OAuth 账号
- 分组或账号的模型白名单包含 `gpt-image-2`，或者使用默认模型集
- 渠道模型映射不要把 `gpt-image-2` 映射到非 `gpt-image-*` 模型；Images endpoint 会拒绝非图片模型

如果想把用户侧别名映射到图片模型，可以在渠道模型映射里配置：

```json
{
  "openai": {
    "image-latest": "gpt-image-2"
  }
}
```

之后用户可用 `model: "image-latest"` 调用，实际上游模型会按映射进入 `gpt-image-2`。

## API 调用教程

以下示例中的 `SUB2API_BASE_URL` 是你的 Sub2API 站点地址，`SUB2API_API_KEY` 是用户在 Sub2API 生成的 API Key，不是上游 OpenAI API Key。

### 图片生成

推荐使用 OpenAI Images API：

```bash
curl "$SUB2API_BASE_URL/v1/images/generations" \
  -H "Authorization: Bearer $SUB2API_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2",
    "prompt": "A clean product photo of a red ceramic mug on a white desk",
    "size": "1024x1024",
    "quality": "auto",
    "response_format": "b64_json"
  }'
```

成功响应示例：

```json
{
  "created": 1777900000,
  "model": "gpt-image-2",
  "data": [
    {
      "b64_json": "iVBORw0KGgo..."
    }
  ],
  "usage": {
    "input_tokens": 12,
    "output_tokens": 4160,
    "output_tokens_details": {
      "image_tokens": 4160
    }
  }
}
```

如果省略 `model`，Sub2API 会默认写入 `gpt-image-2`：

```bash
curl "$SUB2API_BASE_URL/v1/images/generations" \
  -H "Authorization: Bearer $SUB2API_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "A small watercolor painting of a mountain cabin",
    "response_format": "b64_json"
  }'
```

### 保存 b64 图片

可以用下面的 Python 片段调用并保存图片：

```python
import base64
import os
import requests

base_url = os.environ["SUB2API_BASE_URL"].rstrip("/")
api_key = os.environ["SUB2API_API_KEY"]

resp = requests.post(
    f"{base_url}/v1/images/generations",
    headers={"Authorization": f"Bearer {api_key}"},
    json={
        "model": "gpt-image-2",
        "prompt": "A cozy reading corner with warm light",
        "size": "1024x1024",
        "response_format": "b64_json",
    },
    timeout=180,
)
resp.raise_for_status()
data = resp.json()["data"][0]["b64_json"]
with open("gpt-image-2-output.png", "wb") as f:
    f.write(base64.b64decode(data))
```

### 流式图片生成

流式调用适合希望接收 partial image 或完成事件的客户端：

```bash
curl -N "$SUB2API_BASE_URL/v1/images/generations" \
  -H "Authorization: Bearer $SUB2API_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2",
    "prompt": "A minimal black and white icon of a rocket",
    "stream": true,
    "partial_images": 1
  }'
```

API Key 账号会透传上游 Images SSE；OAuth 账号会把 Responses 事件归一化为 Images 风格事件。

### Responses image_generation tool

如果客户端本身走 `/v1/responses`，可以直接声明 `image_generation` tool：

```bash
curl "$SUB2API_BASE_URL/v1/responses" \
  -H "Authorization: Bearer $SUB2API_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-5.4-mini",
    "input": "Draw a flat vector logo of a sunrise over a bridge",
    "tools": [
      {
        "type": "image_generation",
        "model": "gpt-image-2"
      }
    ],
    "tool_choice": {
      "type": "image_generation"
    }
  }'
```

Codex / Responses 客户端如果误把主模型写成 `gpt-image-2`，Sub2API 会在 OAuth Codex 路径中自动规范化为文本主模型 + `image_generation` tool。`gpt-5.3-codex-spark` 不会被自动改成可图片生成模型，而是返回明确的不支持提示。

### 图片编辑

`/v1/images/edits` 当前只支持 API Key 账号直连：

```bash
curl "$SUB2API_BASE_URL/v1/images/edits" \
  -H "Authorization: Bearer $SUB2API_API_KEY" \
  -F model="gpt-image-2" \
  -F prompt="Replace the background with a bright studio backdrop" \
  -F image="@input.png"
```

OAuth 账号暂不支持 Images edits；如果分组里只有 OAuth 账号，这个接口会返回不支持错误。

## 失败分类

网关会尽量区分以下错误：

- `invalid_request_error`：请求参数错误、模型不是 `gpt-image-*`
- `authentication_error`：API Key 或 OAuth access token 不可用
- `rate_limit_error`：上游限额、临时限流
- `api_error`：上游 5xx、连接错误、Cloudflare/Arkose 等临时挑战
- `unsupported_image_generation`：上游 Responses 明确不支持 `image_generation` tool

可 failover 的情况包括 429、5xx/52x、连接错误和临时 HTML 挑战页。400 参数错误、401 鉴权错误和明确同质化不支持通常不应盲目切换账号。

## 验证命令

```bash
cd backend
go test -tags=unit ./internal/service -run 'OpenAI.*Image|Codex.*Image|ModelMapping|Pricing'
go test -tags=unit ./internal/handler -run 'OpenAI.*Image|Endpoint'
go test -tags=unit ./internal/server -run 'Images|Gateway'
```

真实链路可用低成本提示词验证：

```bash
python3 tmp/gpt-image-2-probe/test_gpt_image_2.py --mode images --model gpt-image-2
python3 tmp/gpt-image-2-probe/test_gpt_image_2.py --mode responses-tool --model gpt-5.4-mini
```

环境变量：

```bash
OPENAI_BASE_URL="https://your-sub2api.example/v1"
OPENAI_API_KEY="sk-..."
```

## 回滚方式

1. 设置 `gateway.openai_oauth_images_enabled=false`
2. 重启后端服务
3. 如需限制成本，可暂时从分组模型列表或账号映射中移除 `gpt-image-2`

回滚后 OAuth 账号不再参与 Images 图片生成，API Key Images 直连链路保持不变，不需要数据库迁移或数据修复。
