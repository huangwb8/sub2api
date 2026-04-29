# OpenAI Images API (`/v1/images/generations`) 支持计划

**目标**：为 Sub2API 网关新增 OpenAI Images API 支持，使用户可以通过 `POST /v1/images/generations` 和 `POST /v1/images/edits` 端点调用 `gpt-image-1`、`gpt-image-1-mini`、`gpt-image-1.5`、`gpt-image-2` 等图片生成模型。

**OpenAI 官方 API 参考**：
- 图片生成：https://developers.openai.com/api/reference/resources/images/methods/generate
- 图片编辑：https://developers.openai.com/api/reference/resources/images/methods/edit
- 模型文档：https://developers.openai.com/api/docs/models/gpt-image-2

## 现状分析

### 已有基础设施

| 能力 | 状态 | 位置 |
|------|------|------|
| 图片模型定价 | 已有 `gpt-image-1`、`gpt-image-1-mini`、`gpt-image-1.5`、`gpt-image-1.5-2025-12-16` | `backend/resources/model-pricing/` |
| `gpt-image-2` 定价 | 缺失，需新增 | `backend/resources/model-pricing/` |
| `BillingModeImage` 计费模式 | 已实现 | `backend/internal/service/channel.go` |
| `CalculateImageCost` 计费函数 | 已实现 | `backend/internal/service/billing_service.go` |
| `ForwardResult` 图片字段 | 已有 `ImageCount`、`ImageSize` | `backend/internal/service/gateway_service.go` |
| UsageLog 图片字段 | 已有 `image_count`、`image_size`、`image_output_cost` | `backend/ent/schema/usage_log.go` |
| Group 图片定价配置 | 已有 `ImagePrice1K/2K/4K` | `backend/internal/service/group.go` |
| Gemini 图片生成支持 | 已通过 Antigravity 网关实现 | `backend/internal/service/antigravity_gateway_service.go` |

### 缺失环节

| 缺失项 | 说明 |
|--------|------|
| `/v1/images/generations` 路由 | 网关路由中完全没有注册 |
| `/v1/images/edits` 路由 | 同上 |
| OpenAI 图片转发 handler | handler 层没有对应的处理函数 |
| OpenAI 图片转发 service | service 层没有 `ForwardAsImageGeneration` 方法 |
| 图片 API 请求/响应类型 | 缺少 Images API 专用的请求体和响应体结构定义 |

## OpenAI Images API 协议规范

### POST /v1/images/generations

**请求体**：

```json
{
  "model": "gpt-image-2",
  "prompt": "A cute baby sea otter",
  "n": 1,
  "size": "1024x1024",
  "quality": "auto",
  "background": "auto",
  "output_format": "png",
  "output_compression": 100,
  "moderation": "auto",
  "stream": false,
  "partial_images": 0
}
```

**关键参数**：

| 参数 | 类型 | 说明 |
|------|------|------|
| `model` | string | `dall-e-2`、`dall-e-3`、`gpt-image-1`、`gpt-image-1-mini`、`gpt-image-1.5`、`gpt-image-2` |
| `prompt` | string | 文字描述，GPT Image 模型最长 32000 字符 |
| `n` | int | 生成图片数（1-10），`dall-e-3` 仅支持 1 |
| `size` | string | GPT Image: `auto`/`1024x1024`/`1536x1024`/`1024x1536`；dall-e-2: `256x256`/`512x512`/`1024x1024`；dall-e-3: `1024x1024`/`1792x1024`/`1024x1792` |
| `quality` | string | GPT Image: `auto`/`low`/`medium`/`high`；dall-e-3: `standard`/`hd` |
| `background` | string | GPT Image 专用: `transparent`/`opaque`/`auto` |
| `output_format` | string | GPT Image 专用: `png`/`jpeg`/`webp` |
| `output_compression` | int | GPT Image 专用: 0-100（仅 webp/jpeg） |
| `stream` | bool | GPT Image 专用，默认 false |
| `partial_images` | int | 流式预览图数量（0-3） |

**非流式响应**：

```json
{
  "created": 1713833628,
  "data": [
    {
      "b64_json": "..."
    }
  ],
  "usage": {
    "total_tokens": 100,
    "input_tokens": 50,
    "output_tokens": 50,
    "input_tokens_details": {
      "text_tokens": 10,
      "image_tokens": 40
    },
    "output_tokens_details": {
      "image_tokens": 50,
      "text_tokens": 0
    }
  }
}
```

**流式响应**（SSE）：

```
event: image_generation.partial_image
data: {"type":"image_generation.partial_image","b64_json":"...","partial_image_index":0}

event: image_generation.completed
data: {"type":"image_generation.completed","b64_json":"...","usage":{...}}
```

### POST /v1/images/edits

**请求体**（multipart/form-data）：

| 参数 | 类型 | 说明 |
|------|------|------|
| `model` | string | 同 generations |
| `prompt` | string | 编辑指令 |
| `image` | file[] | 源图片（可多张） |
| `mask` | file | 蒙版图片 |
| `n` | int | 生成数量 |
| `size` | string | 尺寸 |
| `quality` | string | 质量 |

**响应格式**：与 generations 相同。

## 实施计划

### 阶段 1：后端核心 — 路由与类型定义

#### 1.1 新增 Images API 请求/响应类型

**文件**：`backend/internal/pkg/apicompat/openai_images.go`（新建）

```go
// ImageGenerationsRequest 图片生成请求
type ImageGenerationsRequest struct {
    Model            string  `json:"model"`
    Prompt           string  `json:"prompt"`
    N                int     `json:"n,omitempty"`
    Size             string  `json:"size,omitempty"`
    Quality          string  `json:"quality,omitempty"`
    Background       string  `json:"background,omitempty"`
    OutputFormat     string  `json:"output_format,omitempty"`
    OutputCompression int    `json:"output_compression,omitempty"`
    Moderation       string  `json:"moderation,omitempty"`
    Stream           bool    `json:"stream,omitempty"`
    PartialImages    int     `json:"partial_images,omitempty"`
    ResponseFormat   string  `json:"response_format,omitempty"` // dall-e 专用
    Style            string  `json:"style,omitempty"`            // dall-e-3 专用
    User             string  `json:"user,omitempty"`
}

// ImageGenerationsResponse 图片生成响应
type ImageGenerationsResponse struct {
    Created       int64              `json:"created"`
    Data          []ImageData        `json:"data"`
    Usage         *ImageUsage        `json:"usage,omitempty"`
    OutputFormat  string             `json:"output_format,omitempty"`
    Quality       string             `json:"quality,omitempty"`
    Size          string             `json:"size,omitempty"`
    Background    string             `json:"background,omitempty"`
}
```

#### 1.2 新增路由注册

**文件**：`backend/internal/server/routes/gateway.go`

在 `/v1` 路由组中新增：

```go
// Images API: 仅 OpenAI 平台支持
gateway.POST("/images/generations", func(c *gin.Context) {
    platform := getGroupPlatform(c)
    switch platform {
    case service.PlatformOpenAI:
        h.OpenAIGateway.ImageGenerations(c)
    default:
        c.JSON(http.StatusNotFound, gin.H{
            "error": gin.H{
                "type":    "not_found_error",
                "message": "Image generation is only supported for OpenAI platform groups",
            },
        })
    }
})

gateway.POST("/images/edits", func(c *gin.Context) {
    platform := getGroupPlatform(c)
    switch platform {
    case service.PlatformOpenAI:
        h.OpenAIGateway.ImageEdits(c)
    default:
        c.JSON(http.StatusNotFound, gin.H{
            "error": gin.H{
                "type":    "not_found_error",
                "message": "Image editing is only supported for OpenAI platform groups",
            },
        })
    }
})
```

同时在根路由注册不带 `/v1` 前缀的别名（与 chat/completions 保持一致）：

```go
r.POST("/images/generations", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm,
    gin.HandlerFunc(apiKeyAuth), requireGroupAnthropic, imageGenerationsHandler)
r.POST("/images/edits", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm,
    gin.HandlerFunc(apiKeyAuth), requireGroupAnthropic, imageEditsHandler)
```

### 阶段 2：后端核心 — Handler 层

#### 2.1 新增 ImageGenerations handler

**文件**：`backend/internal/handler/gateway_handler_images.go`（新建）

核心流程（参考 `ChatCompletions` handler）：

```
1. 认证检查 → 提取 apiKey / subject
2. 读取请求体 → JSON 校验
3. 提取 model / n / size / stream
4. 渠道模型映射解析
5. 并发控制（用户槽位 + 账号槽位）
6. 调用 service 层 ForwardAsImageGeneration
7. 异步记录使用量
8. 响应返回
```

与 ChatCompletions handler 的关键差异：

| 差异点 | ChatCompletions | ImageGenerations |
|--------|----------------|------------------|
| 必需字段 | `model` + `messages` | `model` + `prompt` |
| 流式类型 | SSE text/event-stream | SSE（事件类型不同） |
| 并发限制 | 支持流式长连接 | 图片生成通常更快 |
| 请求验证 | messages 数组校验 | prompt 非空 + 模型校验 |
| 响应格式 | Chat Completions JSON | Images Response JSON |

#### 2.2 新增 ImageEdits handler

**文件**：同上 `gateway_handler_images.go`

`/images/edits` 使用 multipart/form-data，需要特殊处理：
- 使用 `c.Request.MultipartForm` 解析 `image`、`mask` 文件
- 构建 multipart 请求转发到上游
- 其余流程与 generations 相同

### 阶段 3：后端核心 — Service 层

#### 3.1 新增 ForwardAsImageGeneration 方法

**文件**：`backend/internal/service/openai_gateway_images.go`（新建）

```go
func (s *OpenAIGatewayService) ForwardAsImageGeneration(
    ctx context.Context,
    c *gin.Context,
    account *Account,
    body []byte,
    reqModel string,
) (*OpenAIForwardResult, error) {
    // 1. 解析请求
    // 2. 模型映射（account 级别）
    // 3. 判断是否为流式请求
    // 4. 构建上游请求 URL：
    //    - API Key 账号: {account.BaseURL}/v1/images/generations
    //    - OAuth 账号: 需确认是否支持（可能仅 API Key 账号支持）
    // 5. 转发请求
    // 6. 处理响应：
    //    - 非流式：直接透传响应体，提取 usage
    //    - 流式：SSE 流式转发，提取最终事件的 usage
    // 7. 构造 OpenAIForwardResult（包含 usage、model、image_count、image_size）
    // 8. 返回结果
}
```

**上游 URL 构建**：

```go
upstreamURL := buildUpstreamURL(account, "/v1/images/generations")
// API Key 账号: account.BaseURL + "/v1/images/generations"
// 默认: "https://api.openai.com/v1/images/generations"
```

**流式转发**：

OpenAI Images API 的流式使用 SSE，事件类型为：
- `image_generation.partial_image` — 中间预览
- `image_generation.completed` — 最终图片

流式处理逻辑参考现有的 `forwardOpenAIPassthrough`，但需要：
- 识别 `image_generation.completed` 事件提取 usage
- 透传所有 SSE 事件给客户端

**使用量提取**：

```go
// 从响应体提取 usage
type imagesUsage struct {
    TotalTokens   int `json:"total_tokens"`
    InputTokens   int `json:"input_tokens"`
    OutputTokens  int `json:"output_tokens"`
}

// 从请求体提取图片数量和尺寸
imageCount := gjson.GetBytes(body, "n").Int()  // 默认 1
imageSize := mapSizeToK(gjson.GetBytes(body, "size").String())  // "1024x1024" → "1K"
```

#### 3.2 新增 ForwardAsImageEdit 方法

**文件**：同上 `openai_gateway_images.go`

与 `ForwardAsImageGeneration` 类似，但需要：
- 构建 multipart/form-data 请求
- 上游 URL 为 `/v1/images/edits`

#### 3.3 图片尺寸映射辅助函数

```go
// mapImageSizeToK 将 OpenAI 尺寸字符串映射为计费尺寸等级
func mapImageSizeToK(size string) string {
    switch size {
    case "1024x1024", "auto", "":
        return "2K"  // 默认 2K
    case "1536x1024", "1024x1536":
        return "2K"
    case "256x256", "512x512":
        return "1K"
    case "1792x1024", "1024x1792":
        return "4K"
    default:
        return "2K"
    }
}
```

### 阶段 4：模型定价与计费

#### 4.1 新增 gpt-image-2 定价

**文件**：`backend/resources/model-pricing/model_prices_and_context_window.json`

新增条目（定价参考 OpenAI 官方）：

```json
"gpt-image-2": {
    "cache_read_input_image_token_cost": 2.0e-06,
    "cache_read_input_token_cost": 1.25e-06,
    "input_cost_per_image_token": 8.0e-06,
    "input_cost_per_token": 5.0e-06,
    "litellm_provider": "openai",
    "max_input_tokens": 16000,
    "max_output_tokens": 8192,
    "mode": "image",
    "output_cost_per_token": 8.0e-06,
    "supported_endpoints": [
        "/v1/images/generations",
        "/v1/images/edits"
    ]
}
```

> 注意：具体定价需以 OpenAI 官方页面为准，实施时需确认。

#### 4.2 计费逻辑复用

现有 `BillingService.CalculateImageCost` 已支持按图片尺寸和数量计费。Images API 的 `usage` 字段提供了 token 级别的用量，可以选择：

- **方案 A**（推荐）：使用 token 用量计费 — 与 OpenAI 计费对齐，使用 `usage.input_tokens + usage.output_tokens`
- **方案 B**：使用现有按张/尺寸计费 — 复用 `ImageCount + ImageSize` 字段

建议使用**方案 A**，因为 GPT Image 模型是 token 计费模型，按张计费不精确。但需要在 `ForwardResult` 中增加 token 级别的用量字段。

### 阶段 5：使用量记录

#### 5.1 RecordUsage 扩展

在 `RecordUsage` 方法中增加对 Images API 请求类型的识别：

```go
// 根据 billing_mode 或 model 识别图片请求
if result.BillingMode == BillingModeImage || isImageModel(result.Model) {
    // 记录 image_count, image_size, image_output_cost
    // 同时记录 token 用量（如果有）
}
```

### 阶段 6：前端适配

#### 6.1 模型列表展示

如果前端有模型管理或模型列表页面，需要确保 `gpt-image-*` 系列模型正确展示其支持的端点。

#### 6.2 管理后台

- 账号编辑：确认账号类型支持图片生成模型的选择
- 使用量统计：图片生成请求的使用量正确显示
- 模型定价：管理后台的定价配置页面正确展示图片模型

### 阶段 7：测试

#### 7.1 单元测试

**文件**：`backend/internal/service/openai_gateway_images_test.go`（新建）

测试用例：

| 测试 | 说明 |
|------|------|
| `TestMapImageSizeToK` | 尺寸映射正确性 |
| `TestForwardAsImageGeneration_NonStream` | 非流式转发 |
| `TestForwardAsImageGeneration_Stream` | 流式转发 |
| `TestForwardAsImageGeneration_Usage` | 使用量提取 |
| `TestForwardAsImageEdit_Multipart` | multipart 请求构建 |
| `TestImageGeneration_AccountSelection` | 图片模型账号选择 |

#### 7.2 集成测试

使用真实 API Key 测试端到端流程：
1. 创建 OpenAI 平台分组
2. 配置包含图片模型能力的账号
3. 调用 `/v1/images/generations` 验证完整流程

### 阶段 8：文档同步

按照项目规范，以下文档需要同步更新：

| 文档 | 更新内容 |
|------|----------|
| `CHANGELOG.md` | 记录新增功能 |
| `README.md` | 支持的 API 端点列表中增加 Images API |
| `docs/` 下相关用户文档 | 图片生成 API 使用说明 |

## 涉及文件清单

### 新增文件

| 文件 | 说明 |
|------|------|
| `backend/internal/pkg/apicompat/openai_images.go` | Images API 请求/响应类型 |
| `backend/internal/handler/gateway_handler_images.go` | Images API handler |
| `backend/internal/service/openai_gateway_images.go` | Images API 转发服务 |
| `backend/internal/service/openai_gateway_images_test.go` | 单元测试 |

### 修改文件

| 文件 | 修改内容 |
|------|----------|
| `backend/internal/server/routes/gateway.go` | 注册 `/images/generations` 和 `/images/edits` 路由 |
| `backend/resources/model-pricing/model_prices_and_context_window.json` | 新增 `gpt-image-2` 定价 |
| `backend/internal/service/billing_service.go` | 可能需要扩展 token 级别图片计费 |
| `CHANGELOG.md` | 变更记录 |

## 风险与注意事项

### OAuth 账号兼容性

OpenAI OAuth（ChatGPT）账号可能不支持 Images API 的 `/v1/images/generations` 端点。在实施时需要：
- 确认 OAuth 账号是否支持图片生成
- 如果不支持，在账号选择时过滤掉 OAuth 账号

### 请求体大小

图片编辑（`/images/edits`）使用 multipart/form-data，需要上传图片文件。现有的 `RequestBodyLimit` 中间件可能需要调大限制（图片文件通常几 MB）。

### gpt-image-2 模型定价

`gpt-image-2` 于 2026 年 4 月 21 日发布，其定价可能与 `gpt-image-1` 不同。实施前需确认 OpenAI 官方定价。

### 流式图片生成

GPT Image 模型支持流式生成（`stream: true`），这与 Chat Completions 的流式不同：
- 事件类型不同（`image_generation.partial_image` / `image_generation.completed`）
- 不需要处理 `content_block` 等复杂结构
- 相对更简单，可以复用 SSE 透传逻辑

## 实施优先级

建议分两期实施：

**第一期（MVP）**：仅支持 `/v1/images/generations`（非流式 + 流式），覆盖 `gpt-image-*` 模型。

**第二期**：支持 `/v1/images/edits`（multipart 请求），扩展到 `dall-e-*` 模型。

这样可以快速上线核心能力，edit 端点的 multipart 处理相对复杂，可以延后实施。
