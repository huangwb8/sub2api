# Upstream c92b88 to b0a225 OpenAI Fast Policy Assessment Plan

**Goal:** 分析 `Wei-Shaw/sub2api` 在 `(c92b88e34abd1b032aa5307d760b8bc0aad49b28, b0a2252ed19c3720e6adafde6083e64fbac2efa9]` 区间的真实变化，判断这些变化对当前个人 `sub2api` 项目的启发与必要吸收项；若上游 license 在该区间发生变化，则同步本仓库 license。

**Method:** 本轮按 `awesome-code` 流程先运行 `agent_coordinator.py` 做任务拆解；结果为 `coordination_scope.level=focused-agent`、`dispatch_gate.can_proceed=true`。随后结合本地 `git log`、`git diff`、当前 fork 代码探针、license 哈希对比与现状扫描，按“已吸收 / 建议补强 / 不必重复吸收”给出结论。

**Key Conclusion:** 该区间只有 2 个 commit，其中只有 1 个实质功能提交：`30f55a1f feat(openai): OpenAI Fast/Flex Policy 完整实现（HTTP + WebSocket + Admin）`，另 1 个是合并 PR 的 merge commit。当前个人仓库已经吸收了这套功能主干，并且在部分细节上继续演化；本轮**不需要重复搬运业务功能**，但**有必要补回上游这次提交所体现的专项回归测试意图**，并对当前本地扩展语义做一次显式验证与文档化。

**License Conclusion:** `version1` 与 `version2` 的 upstream `LICENSE` SHA-256 完全一致，均为 `a5681bf9b05db14d86776930017c647ad9e6e56ff6bbcfdf21e5848288dfaf1b`。本轮不存在“上游 license 变化”。当前 fork 根目录 `LICENSE` 与 upstream 文本内容一致，仅存在结尾换行差异，因此本轮**不修改 license 文件**。

---

## 上游区间变化总览

对比范围：`c92b88e34abd1b032aa5307d760b8bc0aad49b28..b0a2252ed19c3720e6adafde6083e64fbac2efa9`

| Commit | 类型 | 结论 |
|--------|------|------|
| `30f55a1f` | `feat(openai)` | 新增一整套 OpenAI Fast/Flex Policy 能力，覆盖 HTTP、WebSocket、Admin 设置页与专项测试。 |
| `b0a2252e` | merge | 合并 PR #2051，本身不引入新的独立业务逻辑。 |

变更文件共 23 个，集中在以下 4 个层面：

### 主题 1：OpenAI 网关新增 Fast/Flex Policy 执行链

- `backend/internal/service/openai_gateway_service.go`
- `backend/internal/service/openai_gateway_chat_completions.go`
- `backend/internal/service/openai_gateway_messages.go`
- `backend/internal/service/openai_ws_forwarder.go`
- `backend/internal/service/openai_ws_v2_passthrough_adapter.go`

上游做了这些事：

- 新增 `settingService` 注入到 `OpenAIGatewayService`，让网关层可读取管理端策略配置。
- 引入 `service_tier` 规则评估能力，支持 `pass / filter / block` 三态策略。
- 对 HTTP 入口统一处理 `service_tier`，包括 `chat completions`、Anthropic 兼容 `messages`、原生 `responses` 与 passthrough。
- 对 WebSocket `response.create` 入口新增策略执行，并在 block 时返回 Realtime 风格错误事件，再以 `1008 policy_violation` 关闭连接。
- 将客户端别名 `"fast"` 规范化为 `"priority"`，避免把 OpenAI 不接受的别名值直接透传上游。
- 在 passthrough WebSocket 中维护 `capturedSessionModel`，防止首帧与后续帧之间通过切换模型规避 `service_tier` 策略。

### 主题 2：设置模型与默认策略落地

- `backend/internal/service/setting_service.go`
- `backend/internal/service/settings_view.go`
- `backend/internal/handler/dto/settings.go`
- `backend/internal/handler/admin/setting_handler.go`

上游做了这些事：

- 新增 `OpenAIFastPolicyRule` / `OpenAIFastPolicySettings` DTO 与领域设置模型。
- 新增 `GetOpenAIFastPolicySettings` / `SetOpenAIFastPolicySettings`。
- 默认规则不是“只管特定模型”，而是“对所有模型的 `priority` 请求执行 filter”，原因是 `service_tier=fast/priority` 是用户级优先级开关，与模型名正交。
- 当 settings 表中的 JSON 损坏时，不再静默吞掉，而是记录 warning 后回落默认策略，方便运维排查脏数据。

### 主题 3：前端管理页新增 Fast/Flex 配置入口

- `frontend/src/api/admin/settings.ts`
- `frontend/src/views/admin/SettingsView.vue`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`

上游做了这些事：

- 在 admin settings API 类型中增加 `openai_fast_policy_settings`。
- 在设置页新增 OpenAI Fast/Flex Policy 表单卡片。
- 支持按 `service_tier / action / scope / model_whitelist / fallback_action / fallback_error_message` 配置规则。
- 使用 `openaiFastPolicyLoaded` 之类的前端守门逻辑，避免默认值把后端未返回的策略字段错误覆盖为空。

### 主题 4：专项测试密集补齐

- `backend/internal/service/openai_fast_policy_test.go`
- `backend/internal/service/openai_fast_policy_ws_test.go`
- 以及若干 handler / contract / record-usage 相关测试

上游测试重点包括：

- 默认策略、block 自定义错误、scope 区分、模型白名单、fallback 行为。
- HTTP filter 是否真的删掉 `service_tier`。
- WebSocket helper 是否只匹配 `response.create`。
- block 时是否先写 error event，再 close 1008。
- passthrough follow-up frame 无 model 时，是否正确使用 `capturedSessionModel`。
- passthrough 计费是否基于 filter 后的最终 `service_tier` 语义。

---

## 当前个人仓库现状判断

### 已吸收的部分

当前仓库不是“完全未吸收”状态，而是**已经包含这次 upstream 功能主干**。可见证据包括：

- `backend/internal/service/openai_gateway_service.go`
  - 已存在 `evaluateOpenAIFastPolicy`
  - 已存在 `applyOpenAIFastPolicyToBody`
  - 已存在 `applyOpenAIFastPolicyToWSResponseCreate`
  - 已存在 `OpenAIFastBlockedError`
- `backend/internal/service/setting_service.go`
  - 已存在 `GetOpenAIFastPolicySettings`
  - 已存在 `SetOpenAIFastPolicySettings`
- `backend/internal/service/settings_view.go`
  - 已存在 `DefaultOpenAIFastPolicySettings`
- `backend/internal/handler/dto/settings.go`
  - 已存在 `openai_fast_policy_settings`
- `frontend/src/api/admin/settings.ts`
  - 已存在 `OpenAIFastPolicySettings` 类型
- `frontend/src/views/admin/SettingsView.vue`
  - 已存在 OpenAI Fast/Flex Policy 表单与 `openaiFastPolicyLoaded` 守门逻辑

也就是说，上游这次核心能力你这边已经“功能性吸收”了。

### 当前仓库相对 upstream 的继续演化

你的仓库不是原样停留在 `version2`，而是在该能力之上继续演化了，至少包括：

- `normalizeOpenAIServiceTier` 已扩展接受 `auto / default / scale`，不只限于 `priority / flex`。
- 管理端设置页和设置 API 文件在后续版本中已经发生较大重构。
- 与 OpenAI WS fallback、Usage 展示、定价等周边逻辑已有更多本地定制。

这意味着后续若要“吸收上游这次变更”，重点不该是重复移植功能，而应该是**验证当前演化后仍然保留了上游原始安全边界与行为保证**。

### 目前真正值得警惕的缺口

我没有在当前仓库里找到 upstream 本次新增的两份专项测试文件：

- `backend/internal/service/openai_fast_policy_test.go`
- `backend/internal/service/openai_fast_policy_ws_test.go`

同时，也没有在当前测试矩阵里搜到等价强度的直接覆盖项。现有测试里能看到的更多是：

- `policy_violation` 相关的 WS fallback 逻辑测试
- 某些 `service_tier` 计费或使用量展示测试

但这和 upstream 这次提交真正保证的“策略执行正确性”不是一回事。

结论是：

- **功能主干已吸收**
- **专项回归保障没有明显保留下来**
- **这就是本轮最值得补的地方**

---

## 是否有必要吸收

### 结论

有必要吸收，但不是“再搬一次功能代码”，而是“补齐当前分叉版本对这套能力的验证闭环”。

### 必要吸收项

#### P0：补回 OpenAI Fast/Flex Policy 的专项回归测试

原因：

- 当前仓库里的策略代码已经明显不是最初 upstream 版本的原样文件。
- 你还对 `service_tier` 语义做了本地扩展。
- 缺少专门测试时，后续重构极容易悄悄破坏 `filter / block / pass` 的边界行为。

建议：

- 以 upstream `30f55a1f` 的两份测试为语义来源，不要求机械照搬文件结构。
- 按当前代码组织，把关键用例拆入现有 service / WS / admin 测试矩阵。

#### P1：补强 Admin 设置链路的契约测试

原因：

- 这套策略的危险点不只在网关执行，还在于 admin 读写是否会把策略“误清空”或“默认值覆盖真实配置”。
- 当前前端设置页与设置 API 已大幅演化，越需要显式约束 `openai_fast_policy_settings` 的 round-trip 行为。

建议：

- 补 `GET /admin/settings` 与 `PUT /admin/settings` 的字段保真测试。
- 补 `openaiFastPolicyLoaded` 相关前端保存行为测试，确保未加载到该字段时不会错误回写。

#### P1：为当前本地扩展的 `auto / default / scale` 语义补明确验证

原因：

- 这是当前个人仓库相对 upstream 的本地增强。
- 一旦没有测试，未来维护者会分不清这是“有意支持”还是“偶然放过”。

建议：

- 增加“默认规则下允许透传 `auto / default / scale`”的单测。
- 增加“管理员显式配置 `service_tier=all` 时，这些 tier 也应被策略接管”的单测。

### 不必重复吸收项

- 不必重复移植 OpenAI Fast/Flex Policy 主体代码。
- 不必因为本轮分析而改动当前业务逻辑。
- 不必修改 license 文件。

---

## 优化实施计划

## Task A: 先把 upstream 这次提交的行为边界转成当前仓库的测试清单

**Goal:** 用当前仓库语言重新表达 upstream `30f55a1f` 的核心保证，形成可执行验收基线。

**Files:**
- Add/Modify: `backend/internal/service/*test.go`
- Add/Modify: `backend/internal/handler/admin/*test.go`
- Add/Modify: `backend/internal/server/api_contract_test.go`
- Add/Modify: `frontend/src/views/admin/*.spec.ts`

**Scope Checklist:**
- HTTP 请求中 `service_tier=fast` 会被规范化成 `priority`
- 默认策略下 `priority` 被 filter
- `flex` 在默认策略下透传
- `block` 返回稳定错误体
- WebSocket 只对 `response.create` 生效
- block 时客户端先收到 error event，再 close `1008`
- passthrough follow-up frame 缺失 model 时仍能命中策略
- `openai_fast_policy_settings` 能安全 round-trip，不被误清空

## Task B: 以当前代码结构补回最关键的 P0 测试

**Goal:** 防止 Fast/Flex Policy 被未来重构破坏。

**Recommended Cases:**
- `evaluateOpenAIFastPolicy` 默认规则、scope、fallback、whitelist
- `applyOpenAIFastPolicyToBody` 的 `pass / filter / block`
- `applyOpenAIFastPolicyToWSResponseCreate` 的严格匹配与错误事件生成
- passthrough `capturedSessionModel` 回退
- billing / usage 对 filter 后 tier 的最终语义

**Expected Outcome:**
- 不再依赖人工阅读大文件判断策略是否仍正确
- 后续改 OpenAI 网关、WS adapter、settings 页面时有明确回归保护

## Task C: 为本地增强语义补测试与注释对齐

**Goal:** 明确当前 fork 与 upstream 的分叉是“有意设计”，不是“偶然偏差”。

**Recommended Cases:**
- `auto / default / scale` 在默认策略下可透传
- `service_tier=all` 时上述 tier 也被统一接管
- 未识别 tier 的处理行为保持稳定

**Recommended Doc Touchpoints:**
- `backend/internal/service/openai_gateway_service.go`
- 若后续需要，再补一条简短开发说明到相应内部文档或测试注释

## Task D: 最后做一次 admin 配置链路的端到端回归

**Goal:** 避免设置页重构后，把策略配置 silently 覆盖为空。

**Recommended Checks:**
- 后端 bulk settings `GET` 真返回该字段时，前端才允许回写
- 保存普通设置时不会误重置 `openai_fast_policy_settings`
- 自定义错误文案在 `block` 模式下正确 round-trip

---

## 未来实施时的验证命令

后续真正动手补强时，建议最少执行：

```bash
cd backend
go test -tags=unit ./internal/service ./internal/handler ./internal/server
```

```bash
cd frontend
pnpm run test
pnpm run typecheck
```

如果只补单测，也至少应执行定向命令：

```bash
cd backend
go test -tags=unit ./internal/service -run 'OpenAIFast|OpenAIWS|Policy'
```

```bash
cd frontend
pnpm test -- --runInBand SettingsView
```

---

## 最终判断

- 上游 `(c92b88, b0a225]` 区间的真实业务变化，核心就是 OpenAI Fast/Flex Policy 全链路落地。
- 当前个人仓库已经吸收了这套能力主干，甚至做了本地增强，所以**没有必要重复搬代码**。
- 目前最值得吸收、也最应该优先做的，是 upstream 这次提交背后的**专项测试与行为约束**。
- license 在该区间**没有变化**，当前仓库也没有出现需要本轮同步的许可证差异。

因此，本轮最优策略是：**不动业务源码，先把后续补强计划固定在 `docs/plans`；等你准备执行时，优先做测试补强而不是功能重写。**
