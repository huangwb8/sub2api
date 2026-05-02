# 插件系统本地内置化改良计划

## 背景

当前插件系统已经支持 `api-prompt` 插件实例、管理端插件页、用户 API Key 绑定模板，以及请求期系统指令注入。上一版优化方向计划把插件系统继续推进为“远端外挂 API 插件”，引入了 `base_url`、`api_key`、`/health`、`/v1/templates`、`/v1/render`、远端缓存与降级状态。

经过重新审视，当前阶段更适合把插件系统收敛为 Sub2API 的本地内置扩展能力，而不是独立外部插件平台。插件应该服务于“轻量、稳定、容易理解、容易部署”的主目标。

本计划替代 `docs/plans/2026-05-02-plugin-system-api-prompt-optimization-plan.md` 中的远端外挂化方向。

## 核心判断

插件系统保留，但定位调整为：

> 插件系统是 Sub2API 内部的轻量扩展模块。插件实例由 Sub2API 本地读取、管理和执行，不通过远端 HTTP 插件服务参与请求链路。

这意味着：

- 保留 `./plugins/{插件名}` 目录约定。
- 保留插件启停、模板管理、API Key 绑定和请求注入能力。
- 移除远端外挂模式，不再暴露或依赖 `base_url`、`api_key`。
- 移除远端插件协议和缓存降级语义。
- 继续保持 `plugin_settings` 的扩展式 JSON 结构，为未来本地插件类型预留空间。

## 优化目标

### P0. 简化用户心智

- 管理员只需要理解“插件实例保存在本地目录中”。
- 用户只需要理解“API Key 可以绑定一个 Prompt 模板”。
- 不再解释远端服务、鉴权、健康检查、模板同步、缓存回退等概念。

### P0. 简化运行时链路

- 请求期不再调用外部插件 HTTP 服务。
- Prompt 注入只读取本地插件配置。
- 远端不可用、超时、鉴权失败、缓存过期等分支全部删除。

### P1. 保留可扩展骨架

- `PluginService` 仍负责插件实例发现、读写、启停和校验。
- `api-prompt` 仍作为第一个本地插件类型。
- API Key 的 `plugin_settings` 保持现有结构：

```json
{
  "api_prompt": {
    "plugin_name": "api-prompt",
    "template_id": "engineering-review"
  }
}
```

### P1. 降低测试和维护成本

- 单测聚焦本地模板校验、插件启停、绑定校验、请求体注入。
- 前端测试聚焦插件页本地模板编辑和 API Key Prompt 选择器。
- 删除远端 mock server、远端缓存、远端失败回退等测试负担。

## 非目标

- 本轮不新增第二种真实插件类型。
- 本轮不引入动态代码加载、脚本执行或沙箱运行时。
- 本轮不支持远端 Prompt Provider。
- 本轮不改变 API Key、计费、调度、账号管理等非插件主链路语义。
- 本轮不设计插件市场或插件包分发机制。

## 目标形态

### 插件目录

插件实例继续固定存放在：

```text
./plugins/{插件名}/
├── manifest.json
└── config.json
```

`manifest.json` 只保留通用本地元数据：

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

`config.json` 保存插件私有配置。对 `api-prompt` 来说，就是模板列表：

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

### 后端行为

- 启动时扫描 `./plugins/*/manifest.json`。
- 只加载已支持的本地插件类型，目前为 `api-prompt`。
- 管理端插件接口只读写本地文件。
- 用户侧模板列表只返回已启用插件中的已启用模板。
- 创建或更新 API Key 时，绑定模板必须存在且启用。
- 请求期如果插件或模板不可用，保持原请求体不变，避免因为插件配置漂移影响主链路可用性。

### 前端行为

- 插件管理页移除 `Base URL` 和 `API Key` 字段。
- 插件测试按钮改成“检查配置”，只校验本地模板有效性。
- 模板编辑区始终可编辑，不再区分远端只读目录。
- 删除“远端模式”“缓存回退”“最近同步时间”“远端模板数”等展示。
- 用户 API Key 页面保留“通用模式”和“绑定模板”选择。

## 分阶段实施

### 阶段 1：后端模型收敛

- 从公开 `Plugin` DTO、创建请求、更新请求中移除 `base_url`、`api_key`、`api_key_configured`。
- 从 `pluginManifest` 中移除 `BaseURL` 和 `APIKey` 字段。
- `readRecord` 读取旧 manifest 时兼容忽略遗留字段，避免已有本地文件导致启动失败。
- `writeRecord` 写回时不再输出遗留字段。

### 阶段 2：删除远端运行时逻辑

- 删除 `hasRemotePluginEndpoint`、远端 HTTP client、远端请求构造、远端模板拉取、远端渲染。
- `TestPlugin` 改为只检查本地已启用模板数量和模板字段合法性。
- `ListAPIPromptTemplateOptions` 固定从本地 `config.json` 构造选项，`source` 固定为 `local`。
- `RenderAPIPrompt` 固定返回本地模板 prompt。
- `ApplyBoundPromptTemplate` 保持各协议注入逻辑不变。

### 阶段 3：前端插件页简化

- 移除新建与编辑表单中的 `Base URL`、`API Key`。
- 移除远端/缓存状态标签和相关 i18n 文案。
- 插件实例卡片只展示：名称、类型、描述、启停状态、模板数量、更新时间。
- 模板编辑继续支持新增、删除、启停、排序、名称、描述和 prompt。
- “测试连接”文案调整为“检查配置”。

### 阶段 4：用户 API Key 绑定保持兼容

- `GET /api/v1/plugins/api-prompt/templates` 返回本地可用模板。
- 创建/更新 API Key 的 `plugin_settings` 请求结构不变。
- 已有绑定继续按 `plugin_name + template_id` 解析。
- 如果旧绑定指向不存在模板，编辑页仍展示“模板不可用”，用户可切回通用模式或选择新模板。

### 阶段 5：文档与辅助资源同步

- 将 `docs/api-prompt-插件协议.md` 改为本地 `api-prompt` 插件说明，或删除远端协议内容后重命名。
- 更新 `skills/sub2api-summary/references/source-map.md`，把插件理解从远端协议改回本地插件配置。
- 更新 README 中如果存在的插件说明。
- 更新 `CHANGELOG.md`，说明插件系统从远端外挂方向收敛为本地内置扩展。
- 如涉及项目指令约束，检查 `AGENTS.md` 与 `CLAUDE.md` 是否需要同步。

## 兼容策略

### 旧 manifest 兼容

如果已有 `manifest.json` 含有：

```json
{
  "base_url": "https://example.com",
  "api_key": "secret"
}
```

新版本读取时应忽略这些字段。管理员保存插件后，文件会被重写为本地模式结构。

### 旧前端缓存兼容

前端类型删除远端字段后，不再展示相关信息。后端响应也不再返回这些字段，避免产生“远端模式仍可用”的误导。

### 旧 API Key 兼容

`plugin_settings` 不变，因此无需数据库迁移。已有 API Key 绑定只要模板仍存在，就继续生效。

## 测试计划

### 后端

- `PluginService` 创建本地 `api-prompt` 插件。
- 插件启停后，用户侧模板列表正确变化。
- 本地模板字段校验：空 ID、空名称、空 prompt、重复 ID、无启用模板。
- API Key 绑定校验：有效模板通过，无效插件或模板拒绝。
- 请求注入覆盖：
  - Anthropic Messages
  - OpenAI Chat Completions
  - OpenAI Responses
  - Gemini Generate Content
- 遗留 `base_url` / `api_key` manifest 可读取，保存后不再写回。

### 前端

- `pnpm run typecheck`
- `pnpm run lint:check`
- 插件页本地模板增删改启停测试。
- API Key 创建/编辑页 Prompt 模板选择测试。
- 模板失效时的提示展示测试。

### 截图审查

由于本轮会修改管理端插件页 UI，需要按项目 UI 审查规则执行：

- 修改前截取插件页 `before.png`。
- 修改后同路由同视口截取 `after.png`。
- 生成左右对比 `compare.png`。
- 截图保存到 `tmp/screenshots/run-{时间戳}/`。

## 风险与应对

### 风险 1：已配置远端字段的实例行为变化

- 应对：这是预期简化方向。读取兼容但运行时忽略，保存后清理字段。

### 风险 2：已有文档仍描述远端外挂模式

- 应对：本轮必须同步更新 `docs/api-prompt-插件协议.md`、`skills/sub2api-summary/references/source-map.md` 和相关 README 描述。

### 风险 3：前端类型与后端 DTO 不一致

- 应对：后端 DTO 与 `frontend/src/types/index.ts` 同步修改，并执行 typecheck。

### 风险 4：请求期插件配置漂移影响用户请求

- 应对：请求期解析不到模板时保持原请求体不变，同时 API Key 编辑页提示绑定不可用。

## 验收标准

- 管理端插件页不再出现 `Base URL`、`API Key`、远端模式、缓存回退、同步时间等概念。
- `plugins/api-prompt/manifest.json` 不再包含远端字段。
- `api-prompt` 模板完全由本地 `config.json` 管理。
- 用户仍可在 API Key 上绑定模板，并在网关请求中得到系统指令注入。
- 后端单测、前端类型检查、前端 lint 通过。
- 插件页截图对比完成并记录目录。

## 建议执行顺序

1. 先改后端 DTO 与服务逻辑，保留 API 路径不变。
2. 再改前端类型和插件页 UI。
3. 然后同步文档、skill source map 与默认插件实例文件。
4. 最后跑测试和截图审查。

这个顺序能把行为核心先收敛，再处理界面和说明，减少中途出现“前端已经简化但后端仍返回远端字段”的短暂不一致。
