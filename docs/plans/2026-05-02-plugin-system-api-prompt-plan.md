# 插件系统与 api-prompt 实施计划

## 目标

- 在系统设置中新增 `插件` 页面，位于 `数据备份` 与 `邮件设置` 之间。
- 支持管理员创建、配置、测试、启停多个插件实例。
- 所有插件实例统一保存在项目根目录 `./plugins/{插件名}`。
- 先落地首个插件类型 `api-prompt`：
  - 提供内置 prompt 模板。
  - 支持管理员新增自定义模板。
  - 支持用户在创建 API Key 时选择 `通用` 或绑定某个模板。
  - 网关在请求转发时将绑定模板作为额外系统指令注入。

## 架构决策

### 插件实例模型

- 采用“插件实例”而非“单一全局插件”的设计。
- 每个实例拥有：
  - `name`
  - `type`
  - `description`
  - `base_url`
  - `api_key`
  - `enabled`
  - `created_at`
  - `updated_at`
- 目录结构统一为：
  - `./plugins/{插件名}/manifest.json`
  - `./plugins/{插件名}/config.json`

### 驱动式扩展

- 后端提供 `PluginService` + driver registry。
- 当前仅实现 `api-prompt` driver，但保留未来新增插件类型的注册位。
- `api-prompt` 的模板能力通过 plugin-specific config 暴露，不污染通用插件元数据。

### API Key 绑定

- 在 `api_keys` 中新增可扩展的 `plugin_settings` JSON 字段。
- 首期约定 `plugin_settings.api_prompt = { plugin_name, template_id }`。
- 这样以后增加其它插件类型时，无需再次修改 API Key 表结构。

### 请求注入

- 在网关 handler 读取请求体后、解析模型前，根据 API Key 的 `plugin_settings` 调用插件服务做 body 重写。
- 支持注入的入口：
  - Anthropic Messages
  - OpenAI Chat Completions
  - OpenAI Responses
  - Gemini Native `generateContent` / `streamGenerateContent`

## 最小改动范围

- `backend/ent/schema/api_key.go`
- `backend/migrations/`
- `backend/internal/service/` 中与插件、API Key、请求注入直接相关的文件
- `backend/internal/repository/api_key_repo.go`
- `backend/internal/handler/` 与 `backend/internal/server/routes/` 中插件/API Key/网关相关文件
- `frontend/src/views/admin/SettingsView.vue` 与新增的插件设置组件
- `frontend/src/views/user/KeysView.vue`
- `frontend/src/api/`、`frontend/src/types/`、`frontend/src/i18n/`
- `AGENTS.md`、`CHANGELOG.md`、必要的 skill/source-map 文档

## 验收标准

- 管理员可在系统设置中看到 `插件` 页并完成新建、编辑、测试、启停。
- `api-prompt` 插件至少提供一组内置模板，且可新增自定义模板。
- 用户创建或编辑 API Key 时可选择 `通用` 或某个 prompt 模板。
- 绑定模板后的请求会稳定注入额外系统指令，未绑定时行为不变。
- 现有 API Key、网关调度、计费与系统设置其它页面无回归。

## 验证计划

- 后端单元测试：
  - 插件目录读写与实例管理
  - `api-prompt` 模板解析与绑定校验
  - 各协议请求体的 prompt 注入
- 后端构建与定向测试：
  - `cd backend && go generate ./ent`
  - `cd backend && go test -tags=unit ./internal/service ./internal/handler ./internal/repository`
- 前端验证：
  - `cd frontend && pnpm run typecheck`
  - `cd frontend && pnpm run test:run`
  - `cd frontend && pnpm build`
- UI 审查：
  - 在 `./tmp/screenshots/run-{timestamp}/` 保存 `before.png`、`after.png`、`compare.png`
