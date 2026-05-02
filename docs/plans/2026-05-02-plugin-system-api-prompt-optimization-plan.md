# 插件系统与 api-prompt 优化计划

## 背景

当前仓库已经具备以下能力：

- 管理端已有 `插件` 页面，支持新建、编辑、测试、启停插件实例。
- 插件实例已经统一落在 `./plugins/{插件名}`。
- `api-prompt` 已支持内置模板、管理员自定义模板、用户 API Key 绑定模板，以及在网关请求中注入系统指令。

但与最初目标逐条对照后，仍有几个关键差距：

- `api-prompt` 目前本质上仍是“本地配置驱动”，`base_url` / `api_key` 只用于健康检查，没有真正通过外挂插件 API 提供模板或执行注入。
- 插件系统虽然预留了多插件方向，但当前前后端大量逻辑仍直接写死 `api-prompt`，未来新增第二种插件时改动面会偏大。
- 后端已有核心单测，但前端插件页与 API Key Prompt 绑定链路缺少足够的专项验证，稳定性闭环还不完整。

本计划的目标，是在不破坏现有 `api-prompt` 可用性的前提下，把插件系统从“首个插件可用”升级到“真正可扩展、可外挂、可验证”的状态。

## 优化目标

### P0. 把 `api-prompt` 改造成真正的外挂 API 插件

- 插件实例中的 `base_url` / `api_key` 不再只是测试用途，而是实际参与模板读取与请求增强。
- 后端对 `api-prompt` 的模板列表、模板详情、可选元数据，优先通过外挂插件 API 获取。
- 网关在应用绑定模板时，支持按统一协议调用外挂插件，获取最终应注入的系统指令或增强结果。

### P1. 把插件系统从“首个插件实现”提升为“可持续扩展骨架”

- 后端插件能力从单一 `switch api-prompt` 结构，收敛为更清晰的 driver/adapter 边界。
- 前端插件管理页从“只会显示 `api-prompt`”演进为“可承载多插件类型，但当前先只开放 `api-prompt`”。
- `plugin_settings` 保持扩展式 JSON 结构，不为第二个插件类型重复改 API Key 表结构。

### P1. 补齐验证闭环

- 增加后端针对外挂 API 同步、降级、失败回退、模板绑定校验的单元测试。
- 增加前端针对插件页和 API Key Prompt 选择器的交互测试。
- 保留 UI 截图审查流程，确保插件页视觉风格继续与现有设置页协调。

## 非目标

- 本轮不设计第二个真实插件类型。
- 本轮不引入插件脚本执行、动态代码加载或沙箱运行时。
- 本轮不把插件系统扩展为独立微服务编排平台。
- 本轮不改变现有用户 API Key、计费、调度、账号管理等非插件主链路语义。

## 现状诊断

### 差距 1：外挂 API 语义未真正落地

- 当前 `base_url` / `api_key` 只在测试按钮触发时调用 `GET {base_url}/health`。
- `api-prompt` 模板的真实来源仍是 `./plugins/{插件名}/config.json`。
- 网关注入逻辑也直接读取本地模板内容，而不是向外挂插件 API 请求最终注入结果。

这会导致“插件”更接近“本地配置实例”，而不是你最初想要的“外挂插件 API”。

### 差距 2：扩展骨架不够彻底

- 后端读写配置、测试逻辑、模板导出逻辑都强依赖 `PluginTypeAPIPrompt`。
- 前端 `Plugin.type`、创建表单和模板配置 UI 也都写死为 `api-prompt`。
- 未来新增插件类型时，前后端都需要多处同步改动，扩展成本偏高。

### 差距 3：验证偏后端，前端专项覆盖不足

- 当前已有插件服务与请求注入相关单测。
- 但插件设置页、用户 API Key Prompt 绑定、模板失效后的 UI 表现，缺少足够前端自动化验证。

## 目标架构

### 插件实例层

- 继续保留目录约定：
  - `./plugins/{插件名}/manifest.json`
  - `./plugins/{插件名}/config.json`
- `manifest.json` 负责通用元数据：
  - `name`
  - `type`
  - `description`
  - `base_url`
  - `api_key`
  - `enabled`
  - `created_at`
  - `updated_at`
- `config.json` 负责插件类型私有配置与本地缓存。

### 后端 driver 层

- 引入更明确的 `PluginDriver` 边界，而不是让 `PluginService` 直接承载所有插件细节。
- `PluginService` 负责：
  - 插件实例目录读写
  - 插件启停与实例元数据管理
  - 统一调度 driver
- `api-prompt` driver 负责：
  - 外挂 API 健康检查
  - 模板目录同步
  - 绑定模板合法性校验
  - 基于模板生成注入内容

### `api-prompt` 外挂协议层

建议把外挂 `api-prompt` 协议限制在小而稳定的接口集合，避免一开始设计得过重：

- `GET /health`
  - 用于测试按钮和启动期健康校验
- `GET /v1/templates`
  - 返回当前插件对外暴露的模板目录
- `POST /v1/render`
  - 输入 `plugin_name`、`template_id`、目标协议类型与上下文元数据
  - 返回最终要注入的 prompt 或结构化 system instruction

这样可以把“模板列表”和“实际注入内容”分开，便于以后支持变量化模板、按模型差异化渲染、按用户分组渲染等能力。

### 本地缓存与降级策略

- 插件实例的 `config.json` 不再只是管理员编辑源，而是升级为“本地缓存 + 回退配置”。
- 当外挂 API 正常时：
  - 模板列表以远端为准
  - 可把最近一次同步结果持久化到本地缓存
- 当外挂 API 暂时不可用时：
  - 测试按钮明确报告异常
  - 用户侧模板列表与网关注入可按策略选择“拒绝绑定”或“回退到最近一次有效缓存”

建议默认策略：

- 管理端编辑模板：对纯外挂模式禁用本地直接编辑，避免“本地与远端双写”语义混乱。
- 用户已有绑定继续工作：允许使用最近一次成功同步的缓存模板回退。
- 新建或改绑模板：要求远端模板可用，避免用户绑定到过期模板。

## 分阶段实施

### 阶段 1：后端插件骨架重整

- 抽出 `PluginDriver` 接口与 `api-prompt` driver 实现。
- 把当前 `switch PluginTypeAPIPrompt` 的读写、测试、模板导出逻辑迁移到 driver 内部。
- 保持现有 API 路径不变，先只做内部结构调整。

### 阶段 2：定义并接入 `api-prompt` 外挂 API 协议

- 为 `api-prompt` 定义最小协议：
  - `GET /health`
  - `GET /v1/templates`
  - `POST /v1/render`
- 在插件服务中加入远端客户端、超时、鉴权头、错误包装和响应校验。
- `ListAPIPromptTemplateOptions` 改为优先读远端目录，必要时落本地缓存。
- `ApplyBoundPromptTemplate` 改为通过 driver 获取最终注入内容，而不是只拼本地模板文本。

### 阶段 3：前端插件管理页语义升级

- 插件页继续保留现有风格与信息层级。
- 根据插件模式区分两类配置展示：
  - 外挂驱动型：展示 `base_url`、`api_key`、同步状态、最近同步时间、远端模板数
  - 本地回退型：展示缓存状态和降级说明
- `api-prompt` 模板编辑区需要调整语义：
  - 如果远端接管模板，则改为“只读目录 + 同步状态”
  - 若保留本地 fallback 模板，则明确标识“仅降级回退使用”

### 阶段 4：用户 API Key 绑定与失效处理优化

- Prompt 模板选择器继续保留“通用（不注入额外 Prompt）”选项。
- 绑定模板时增加模板来源和状态信息：
  - 正常
  - 暂时不可用
  - 已失效但当前 Key 仍绑定旧模板
- 对“已绑定模板后来失效”的场景，明确前后端行为：
  - 编辑页提示风险
  - 请求期按缓存/拒绝策略执行

### 阶段 5：测试与文档闭环

- 后端：
  - driver 单测
  - 远端模板同步与缓存回退测试
  - 远端渲染失败与超时测试
  - API Key 绑定校验测试
  - 多协议请求注入测试
- 前端：
  - 插件页新建/测试/启停/同步状态测试
  - 用户 API Key Prompt 选择器测试
  - 模板失效状态展示测试
- 文档：
  - 必要时补插件对接说明
  - 同步 `skills/sub2api-summary/references/source-map.md`

## 改动范围

### 后端

- `backend/internal/service/plugin_service.go`
- `backend/internal/service/` 下新增或重构 driver / client / cache 相关文件
- `backend/internal/handler/admin/plugin_handler.go`
- `backend/internal/handler/plugin_handler.go`
- `backend/internal/handler/plugin_prompt_helper.go`
- 网关相关 handler 中的插件注入入口

### 前端

- `frontend/src/views/admin/SettingsView.vue`
- `frontend/src/views/admin/components/SettingsPluginsTab.vue`
- `frontend/src/views/user/KeysView.vue`
- `frontend/src/api/admin/plugins.ts`
- `frontend/src/api/plugins.ts`
- `frontend/src/types/index.ts`
- `frontend/src/i18n/`

### 文档与辅助文件

- `docs/plans/`
- `AGENTS.md`
- `CLAUDE.md`
- `CHANGELOG.md`
- `skills/sub2api-summary/references/source-map.md`
- `plugins/api-prompt/`

## 风险与应对

### 风险 1：远端插件不可用导致用户请求波动

- 应对：引入本地缓存与明确降级策略，不把远端瞬时故障直接放大为全量用户不可用。

### 风险 2：本地缓存与远端模板状态不一致

- 应对：记录模板同步时间、版本摘要或 ETag，UI 明示“当前数据来源”。

### 风险 3：过早抽象导致实现变重

- 应对：driver 接口只围绕当前已知需求设计，不为未知插件类型预埋过多复杂抽象。

### 风险 4：前端交互语义变复杂

- 应对：保留“创建插件实例 → 测试连接 → 同步模板 → 绑定 API Key”这条主线，不把插件模式解释做成复杂配置迷宫。

## 验收标准

- `api-prompt` 插件实例在配置了 `base_url` / `api_key` 后，模板目录与注入行为都真正由外挂 API 驱动。
- 远端插件不可用时，系统行为符合预设降级策略，并且管理员与用户都能看到明确状态。
- 插件系统新增第二种类型时，不需要再重写当前 `api-prompt` 逻辑结构。
- 插件页视觉风格与现有设置页协调，不破坏现有布局和交互一致性。
- API Key 的“通用 / 模板绑定”工作流保持直观，无回归。
- 现有非插件功能无回归。

## 验证计划

- 后端定向测试：
  - `cd backend && go test -tags=unit ./internal/service ./internal/handler`
- 前端定向测试：
  - `cd frontend && pnpm run typecheck`
  - `cd frontend && pnpm run test:run`
  - `cd frontend && pnpm build`
- UI 审查：
  - 在 `./tmp/screenshots/run-{timestamp}/` 保存 `before.png`、`after.png`、`compare.png`
- 如需远端联调：
  - 优先使用只读测试环境
  - 不在未确认前对真实生产插件执行写操作

## 建议的实施顺序

1. 先做后端 driver 重整，不改对外 API。
2. 再接入 `api-prompt` 外挂协议与缓存/降级策略。
3. 然后调整前端插件页与用户 Key 绑定体验。
4. 最后补齐测试、截图审查和相关文档。

## 评审重点

- 是否接受“远端模板为主，本地缓存为辅”的总体方向。
- 是否接受 `POST /v1/render` 作为外挂插件生成最终注入内容的统一接口。
- 是否接受“已有绑定可降级回退，新绑定必须依赖远端可用”的默认策略。
- 是否接受本轮只把插件系统做成“轻 driver 扩展架构”，而不进一步引入更重的动态插件运行机制。
