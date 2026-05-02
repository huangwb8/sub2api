# api-prompt 本地插件说明

`api-prompt` 是 Sub2API 内置的本地插件类型，用于为 API Key 绑定可复用的 Prompt 模板。插件实例固定保存在 `./plugins/{插件名}`，运行时由 Sub2API 直接读取和执行，不依赖外部 HTTP 插件服务。

## 文件结构

```text
./plugins/{插件名}/
├── manifest.json
└── config.json
```

`manifest.json` 保存通用元数据：

```json
{
  "name": "api-prompt",
  "type": "api-prompt",
  "description": "默认 api-prompt 插件实例",
  "enabled": true,
  "created_at": "2026-05-02T05:20:07Z",
  "updated_at": "2026-05-02T05:20:07Z"
}
```

`config.json` 保存模板列表：

```json
{
  "templates": [
    {
      "id": "engineering-review",
      "name": "工程审查助手",
      "description": "更强调正确性、边界条件与风险识别。",
      "prompt": "你是一位严谨的工程审查助手...",
      "enabled": true,
      "builtin": true,
      "sort_order": 20
    }
  ],
  "source": "local"
}
```

## 管理行为

- 启动时会优先从当前工作目录向上识别 Sub2API 项目根目录，并扫描项目根下的 `./plugins/*/manifest.json`；如果当前运行环境不是完整仓库结构，则回退到当前工作目录下的 `./plugins/*/manifest.json`。
- 当前只加载 `type=api-prompt` 的本地插件实例。
- 管理端插件页可创建、启停、编辑描述、维护模板并检查本地配置。
- 检查配置只校验本地模板是否存在启用项，以及模板字段是否有效。
- 遗留 `manifest.json` 中的 `base_url` 或 `api_key` 字段会被读取时忽略；管理员保存插件后，这些字段不会再写回。

## API Key 绑定

API Key 的 `plugin_settings` 结构保持稳定：

```json
{
  "api_prompt": {
    "plugin_name": "api-prompt",
    "template_id": "engineering-review"
  }
}
```

创建或更新 API Key 时，后端要求绑定的插件已启用，且模板存在、启用并包含非空 `prompt`。用户侧模板列表只返回本地已启用插件中的已启用模板。

## 请求注入

绑定模板后，网关会把模板 `prompt` 注入到请求系统指令中，覆盖入口包括：

- Anthropic Messages
- OpenAI Chat Completions
- OpenAI Responses
- Gemini Generate Content

如果请求期插件被停用、模板被删除或模板不可用，本次请求保持原始请求体不变，避免插件配置漂移影响主链路可用性。
