# WS ctx_pool 模式前端 UI 恢复计划

## 现状分析

### 后端状态：已完成，生产就绪

`ctx_pool` 模式在后端已完整实现，无任何 TODO、FIXME 或桩代码：

| 模块 | 文件 | 状态 |
|------|------|------|
| 常量定义 | `backend/internal/service/account.go` | 三态完整（off/ctx_pool/passthrough）+ 旧值兼容（shared/dedicated → ctx_pool） |
| 协议解析器 | `backend/internal/service/openai_ws_protocol_resolver.go` | 完整路由，无 TODO |
| 连接池 | `backend/internal/service/openai_ws_pool.go`（~1700 行） | 生产级实现：acquire/release、健康检查、预热、动态扩缩、指标采集 |
| 转发器 | `backend/internal/service/openai_ws_forwarder.go` | ctx_pool/passthrough 双模式路由，无未完成分支 |
| 透传适配器 | `backend/internal/service/openai_ws_v2_passthrough_adapter.go` | 独立于 ctx_pool，直接帧中继 |
| 账号配置 | `backend/internal/service/account.go` | 模式解析带完整降级链（新字段 → 旧布尔字段 → 全局默认） |
| 网关配置 | `backend/internal/config/config.go` | 完整的连接池调优参数 |
| DTO/映射 | `backend/internal/handler/dto/` | 正确映射到 API 响应 |
| 测试 | 30+ WebSocket 相关测试文件 | 覆盖真实行为，非桩测试 |

**结论**：后端可正确处理 `ctx_pool` 请求，无需修改。

### 前端状态：基础设施就绪，UI 选项被刻意隐藏

**已完成的部分**：

| 模块 | 文件 | 状态 |
|------|------|------|
| 工具函数 | `frontend/src/utils/openaiWsMode.ts` | 完整支持三态 + 旧值归一化 |
| TypeScript 类型 | 同上 | `OpenAIWSMode` 联合类型已定义 |
| i18n 中文 | `frontend/src/i18n/locales/zh.ts` | `wsModeCtxPool: '上下文池'` 已存在 |
| i18n 英文 | `frontend/src/i18n/locales/en.ts` | `wsModeCtxPool: 'Context Pool (ctx_pool)'` 已存在 |
| 单元测试 | `frontend/src/utils/__tests__/openaiWsMode.spec.ts` | ctx_pool 场景已覆盖 |

**被隐藏的部分**：

#### 1. EditAccountModal（编辑账号弹窗）

文件：`frontend/src/components/account/EditAccountModal.vue`

第 1772 行：`OPENAI_WS_MODE_CTX_POOL` 导入被注释掉
第 1917-1918 行：

```javascript
// TODO: ctx_pool 选项暂时隐藏，待测试完成后恢复
// { value: OPENAI_WS_MODE_CTX_POOL, label: t('admin.accounts.openai.wsModeCtxPool') },
```

#### 2. CreateAccountModal（创建账号弹窗）

文件：`frontend/src/components/account/CreateAccountModal.vue`

第 2845 行：`OPENAI_WS_MODE_CTX_POOL` 导入被注释掉
第 3078-3079 行：同样的 TODO 注释和注释掉的选项

#### 3. BulkEditAccountModal（批量编辑弹窗）

文件：`frontend/src/components/account/BulkEditAccountModal.vue`

第 1103-1106 行：ctx_pool 选项**完全不存在**（无 TODO 注释，无被注释掉的代码）

### 已发现的 UX 问题

如果后端已有账号设置了 `ctx_pool` 模式（通过旧版 UI 或直接改数据库），前端 Select 下拉框会因为找不到匹配选项而**显示为空白**。

## 修改计划

### 步骤 1：恢复 EditAccountModal 的 ctx_pool 选项

文件：`frontend/src/components/account/EditAccountModal.vue`

- 取消注释 `OPENAI_WS_MODE_CTX_POOL` 的导入（~第 1772 行）
- 取消注释 `openAIWSModeOptions` 中的 ctx_pool 选项（~第 1917-1918 行）
- 删除 TODO 注释

### 步骤 2：恢复 CreateAccountModal 的 ctx_pool 选项

文件：`frontend/src/components/account/CreateAccountModal.vue`

- 取消注释 `OPENAI_WS_MODE_CTX_POOL` 的导入（~第 2845 行）
- 取消注释 `openAIWSModeOptions` 中的 ctx_pool 选项（~第 3078-3079 行）
- 删除 TODO 注释

### 步骤 3：为 BulkEditAccountModal 补充 ctx_pool 选项

文件：`frontend/src/components/account/BulkEditAccountModal.vue`

- 导入 `OPENAI_WS_MODE_CTX_POOL`
- 在 `openAIWSModeOptions` 中补充 ctx_pool 选项（放在 off 和 passthrough 之间）

### 步骤 4：验证

- `cd frontend && pnpm run typecheck` — 类型检查通过
- `cd frontend && pnpm test` — 单元测试通过
- `cd frontend && pnpm run lint:check` — ESLint 检查通过
- 手动验证：创建/编辑账号时 Select 下拉框显示三个选项
- 手动验证：已有 ctx_pool 账号编辑时下拉框正确回显

### 步骤 5：同步文档

更新 `docs/关键模型参数设置.md` 中 WS mode 部分，移除对 ctx_pool "暂时不可用"的暗示（如果需要的话）。

## 影响范围

- 仅前端 3 个组件文件，各改动 2-4 行
- 无后端改动
- 无数据库迁移
- 无 API 变更
- i18n 无需改动（翻译已就绪）
- 工具函数和类型无需改动（已就绪）

## 风险评估

| 风险 | 可能性 | 影响 | 缓解措施 |
|------|--------|------|----------|
| ctx_pool 在特定场景下不稳定 | 低 | 中 | 前端选项仍可随时切回 off 或 passthrough |
| 管理员不理解 ctx_pool 含义 | 低 | 低 | 文档已有详细说明，UI 提示文案已就绪 |
| 旧版 shared/dedicated 值兼容问题 | 极低 | 低 | 后端已有归一化逻辑，前端工具函数也已处理 |
