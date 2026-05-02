# api-prompt 插件协议

`api-prompt` 插件实例仍保存在 `./plugins/{插件名}`，其中 `manifest.json` 负责通用元数据，`config.json` 保存最近一次可用的模板缓存与本地回退模板。

当插件 `manifest.json` 配置了 `base_url` 后，后端会优先按外挂 API 模式工作；未配置 `base_url` 时继续使用本地 `config.json` 模板。

## 认证

如果插件实例配置了 `api_key`，平台调用外挂服务时会同时发送：

- `Authorization: Bearer <api_key>`
- `x-api-key: <api_key>`

外挂服务可任选其一校验。

## 健康检查

```http
GET /health
```

用于管理端“测试连接”。返回任意 `2xx` 状态码即视为健康。

## 模板目录

```http
GET /v1/templates
```

返回格式支持对象包裹或数组：

```json
{
  "templates": [
    {
      "id": "engineering-review",
      "name": "工程审查助手",
      "description": "强调正确性、边界条件与风险识别",
      "prompt": "可选；如提供，会作为远端失败时的缓存回退内容",
      "enabled": true,
      "builtin": false,
      "sort_order": 10
    }
  ]
}
```

`prompt` 在远端目录中是可选字段。新建或改绑 API Key 模板时，平台要求远端目录当前可访问且目标模板为 `enabled=true`。

## 渲染注入

```http
POST /v1/render
Content-Type: application/json
```

请求体：

```json
{
  "plugin_name": "api-prompt",
  "template_id": "engineering-review",
  "target": "openai_chat_completions",
  "context": {
    "plugin_type": "api-prompt"
  }
}
```

`target` 取值包括：

- `anthropic_messages`
- `openai_chat_completions`
- `openai_responses`
- `gemini_generate_content`

响应体：

```json
{
  "prompt": "最终要注入的系统指令"
}
```

也兼容 `system_instruction` 字段。远端渲染失败时，平台会尝试使用最近一次缓存中带 `prompt` 内容的模板继续注入；如果没有可用缓存，则本次请求保持原始请求体不变。
