# Upstream `23def40b..78f691d2` 选择性吸收优化计划

> **For Codex / Claude:** 本文档只负责规划，不直接修改业务源码。后续实施应按主题分批吸收、分批验证，禁止把当前 fork 直接整段追平上游。

**Goal:** 基于上游 `Wei-Shaw/sub2api` 在 `23def40bc5415c04ca3a05bb6d67a6ff1e4a3566..78f691d2de24d0d13ce68922e120c8119ea32856` 之间的演进，梳理 commit 变化，判断哪些改动对当前个人 fork 真正有启发，并把值得吸收的部分沉淀为低风险、可验证的实施计划。

**Method:** 本轮按 `awesome-code` 的协调思路先运行 `agent_coordinator.py` 做任务拆解；`dispatch_gate.can_proceed = true`。随后结合本地 `git log`、`git show`、`git diff`、当前 fork 源码对照，以及两路并行只读评估，判断每个主题属于“建议吸收 / 有条件吸收 / 不吸收 / 仅记录无需动作”中的哪一类。

## 范围结论

- 该区间共 **7** 个提交，其中 **5** 个实质变更主题、**2** 个 merge/sync 提交。
- 总计涉及 **37** 个文件，约 **566** 行新增、**352** 行删除。
- 变化聚焦在 5 个方向：
  - 微信支付配置校验前置化与结构化错误码
  - 支付错误国际化与字段标签本地化
  - OpenAI/Codex 模型清理与归一化兜底修正
  - CLA 文档与自动签署工作流
  - README 赞助商内容更新
- 对当前 fork 来说，最值得吸收的是支付链路的 correctness / UX 修复；模型清理需要重写吸收，不能照搬；CLA 与 sponsors 更偏仓库治理，不是当前主线。

## 上游变化摘要

### Commit 列表

- `79192cf6` `feat(payment): harden wxpay config validation with structured errors`
- `40d4e167` `feat(payment): i18n payment error codes and label localization`
- `bbc4aed3` `fix(openai): 移除已下线 Codex 模型并修复归一化兜底副作用`
- `a8854947` `Merge pull request #1764 from touwaeriol/feat/wxpay-pubkey-hardening`
- `ffc9c387` `Merge pull request #1766 from touwaeriol/fix/codex-drop-removed-models`
- `960b2bb8` `feat(legal): add CLA with automated GitHub Actions enforcement`
- `78f691d2` `chore: update sponsors`

### 主题 1：微信支付配置前置校验与结构化错误

对应提交：

- `79192cf6`

核心变化：

- `backend/internal/payment/provider/wxpay.go`
  - 把 `fmt.Errorf(...)` 文本错误改成结构化 `ApplicationError`
  - 引入 `WXPAY_CONFIG_MISSING_KEY` / `WXPAY_CONFIG_INVALID_KEY_LENGTH` / `WXPAY_CONFIG_INVALID_KEY`
  - 在 `NewWxpay(...)` 就解析 `privateKey` / `publicKey`，避免坏配置拖到首次下单才暴露
- `backend/internal/service/payment_config_providers.go`
  - 新增 `validateProviderConfig(...)`
  - 对启用中的 provider 在创建/更新时立即校验，而不是延迟到订单创建
- `backend/internal/service/payment_order.go`
  - 尽量透传具体支付错误，而不是统一包成泛化 `PAYMENT_GATEWAY_ERROR`
- `backend/internal/payment/provider/wxpay_test.go`
  - 补了 PEM 与配置错误分支测试

### 主题 2：支付错误国际化

对应提交：

- `40d4e167`

核心变化：

- `frontend/src/utils/apiError.ts`
  - `extractApiErrorCode(...)` 改为优先取 `reason`
  - 新增 `extractApiErrorMetadata(...)`
  - 新增 `extractI18nErrorMessage(...)`
- `frontend/src/i18n/locales/{zh,en}.ts`
  - 为支付错误 reason 补充完整 i18n 文案
- 多个 payment UI
  - 从 `extractApiErrorMessage(...)` 迁移到 `extractI18nErrorMessage(...)`
  - 自动把 `metadata.key` 这类字段名映射成“证书序列号 / Public Key ID”等界面标签

### 主题 3：OpenAI/Codex 模型清理与归一化守卫

对应提交：

- `bbc4aed3`

核心变化：

- 删除多组已下线或不再支持的 Codex/GPT-5 兼容映射
- `normalizeCodexModel(...)` 默认回退从 `gpt-5.1` 切到 `gpt-5.4`
- 修复 `shouldAutoInjectPromptCacheKeyForCompat(...)` 与 `isOpenAIGPT54Model(...)` 在 fallback 变化后可能误判非 GPT 模型的问题
- 同步清理前端白名单、UseKeyModal 预设与计费回退分支

### 主题 4：CLA 与自动签署工作流

对应提交：

- `960b2bb8`

核心变化：

- 新增 `CLA.md`
- 新增 `.github/workflows/cla.yml`
- 目的偏向外部 PR 协作与未来双许可证/闭源版本治理

### 主题 5：赞助商信息更新

对应提交：

- `78f691d2`

核心变化：

- 更新 `README.md` / `README_CN.md` / `README_JA.md`
- 新增赞助商 logo `assets/partners/logos/bestproxy.png`

## 当前 fork 的差距判断

### 已确认未吸收且值得处理

#### 1. 微信支付配置错误仍然是“保存时静默、下单时爆炸”

当前 fork 现状：

- `backend/internal/payment/provider/wxpay.go` 仍使用普通 `fmt.Errorf(...)`，且 `NewWxpay(...)` 只校验必填字段与 `apiV3Key` 长度，不会预解析 PEM。
- `backend/internal/service/payment_config_providers.go` 在启用 provider 时没有立即执行 provider-level 配置校验。
- `backend/internal/service/payment_order.go` 仍会把创建 provider / 下单阶段的具体错误重新包成泛化 `PAYMENT_GATEWAY_ERROR`。

判断：

- 这是当前区间里最有价值的吸收项之一。
- 它直接减少支付配置错误的发现延迟，能把“真实订单失败”前移为“后台保存配置就报错”。
- 对个人站点尤其有价值，因为微信支付配置问题往往不是高频，但一旦出错就是关键路径阻断。

建议优先级：

- `P0`

#### 2. 支付错误国际化链路仍然不完整

当前 fork 现状：

- `frontend/src/utils/apiError.ts` 仍优先读 `code`，没有元数据提取与 `extractI18nErrorMessage(...)`。
- 当前前端虽然已经有部分 `payment.errors.*` 文案和对 `TOO_MANY_PENDING` 的特判，但整体还是“局部补洞”，不是完整的 reason-code 驱动方案。
- `frontend/src/components/payment/providerConfig.ts` 已经把 `wxpay.publicKey` / `publicKeyId` / `certSerial` 设为必填，但这只是上游方案里很小的一部分。

判断：

- 这项收益比主题 1 略低，但和主题 1 强耦合。
- 如果后端先引入结构化错误而前端不跟上，管理员仍可能看到不够友好的原始报错文本。
- 适合和主题 1 同轮做完，形成完整闭环。

建议优先级：

- `P1`

### 有启发，但不能直接照搬

#### 3. OpenAI/Codex 模型清理值得做“模型矩阵治理”，不值得直接套上游补丁

当前 fork 现状：

- 当前 fork 的 OpenAI 模型面远比上游这次 patch 更大，且已经显著前移到新的 GPT-5.x / Claude 4.x / Gemini 3.x 时代。
- `backend/internal/pkg/openai/constants.go`、`backend/internal/service/openai_codex_transform.go`、`frontend/src/composables/useModelWhitelist.ts`、`frontend/src/components/keys/UseKeyModal.vue` 仍保留 `gpt-5`、`gpt-5.1-codex`、`gpt-5.2-codex`、`gpt-5.4-nano` 等多组兼容名。
- `normalizeCodexModel(...)` 目前默认仍回退到 `gpt-5.1`，与上游改成 `gpt-5.4` 的策略不同。
- `frontend/src/components/keys/UseKeyModal.vue` 的部分示例配置已经默认偏向 `gpt-5.4`，说明当前 fork 并非完全背离上游方向，而是处于“新旧兼容并存”的过渡态。

判断：

- 直接吸收上游这笔删除 patch 风险偏高，因为它会影响：
  - 你的模型白名单
  - UseKeyModal 中给用户的配置模板
  - 计费 fallback
  - 测试里依赖的样例模型
- 但它的启发是有价值的：当前 fork 需要一套“模型矩阵治理”流程，定期清理死别名、统一默认 fallback、并给非 GPT 模型加显式守卫，而不是无限叠加兼容映射。

建议优先级：

- `P2`

### 当前不建议吸收

#### 4. CLA 工作流

当前 fork 现状：

- 仓库中没有 `CLA.md`
- `.github/workflows/` 里也没有 CLA 检查工作流
- 上游 CLA/workflow 绑定的是上游仓库主体与签署语境，不能直接复制到当前 fork 使用

判断：

- 如果你准备长期接受外部 PR，并希望保留未来的双许可证或闭源发行空间，CLA 有治理价值。
- 但对当前“个人 fork + 自主迭代”为主的状态，它不是稳定性或营收链路上的优先项。
- 现在加它会增加外部贡献门槛和仓库治理复杂度，不符合这轮“先稳主链路”的目标。
- 即便未来要做，也应该按当前 fork 的仓库地址、维护者主体和签署流程重新起草，而不是直接吸收上游文件。

建议优先级：

- `暂不吸收`

#### 5. Sponsors / README 赞助商更新

判断：

- 这属于品牌/运营层同步，不影响当前 fork 的稳定性、兼容性或支付可用性。
- 你的 README 体系已经与上游不同步，且本地采用 `README.md` / `README_EN.md` / `README_JA.md`，并不沿用上游的 `README_CN.md` 组织方式。

建议优先级：

- `不吸收`

## License 结论

- 上游在本区间 `23def40b..78f691d2` 内 **没有再次修改 `LICENSE`**。
- 当前仓库本地 `LICENSE` 与上游 `78f691d2:LICENSE` **没有实质内容差异**，仅存在文件结尾换行差异。
- 因此本轮 **无需额外修改本地 License**。

## 选择性吸收建议

## P0：优先吸收

### 主题 A：微信支付配置保存即校验，错误码结构化透传

建议目标：

- 管理员在保存启用中的微信支付配置时，就能立即发现缺失字段、错误长度和非法 PEM。
- 订单创建链路保留具体 `reason`，前端后续可据此做准确提示。

建议实施文件：

- `backend/internal/payment/provider/wxpay.go`
- `backend/internal/payment/provider/wxpay_test.go`
- `backend/internal/service/payment_config_providers.go`
- `backend/internal/service/payment_order.go`

实施要点：

- 把 `WXPAY_CONFIG_MISSING_KEY` / `WXPAY_CONFIG_INVALID_KEY_LENGTH` / `WXPAY_CONFIG_INVALID_KEY` 作为稳定错误码。
- `CreateProviderInstance(...)` 与 `UpdateProviderInstance(...)` 仅在实例最终为启用态时执行即时校验；禁用草稿仍允许半成品保存。
- `invokeProvider(...)` 遇到已有结构化 `ApplicationError` 时尽量透传，只补充必要 metadata，不要重新包成单一 `PAYMENT_GATEWAY_ERROR`。

验证方式：

- `cd backend && go test -tags=unit ./internal/payment/provider ./internal/service -run 'Wxpay|Payment'`
- 手工验证：
  - 缺少 `publicKeyId` 或 `certSerial` 时，后台保存立即失败
  - 非法 PEM 在保存时而不是首单时暴露
  - 下单失败提示能携带稳定 reason，而不是只有泛化 message

## P1：建议同轮完成

### 主题 B：支付错误国际化闭环

建议目标：

- 前端优先读取 `reason`
- 读取并渲染 `metadata`
- payment 相关页面统一使用同一套错误翻译入口

建议实施文件：

- `frontend/src/utils/apiError.ts`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`
- `frontend/src/views/admin/SettingsView.vue`
- `frontend/src/views/user/PaymentView.vue`
- `frontend/src/views/user/UserOrdersView.vue`
- 以及当前实际调用 `extractApiErrorMessage(...)` 的 payment 相关组件

实施要点：

- 增加 `extractApiErrorMetadata(...)` 与 `extractI18nErrorMessage(...)`
- 自动把 `metadata.key` / `metadata.keys` 映射到 `admin.settings.payment.field_*`
- 不强行全站替换，只先收口 payment 域，降低回归面

验证方式：

- `cd frontend && pnpm run typecheck`
- `cd frontend && pnpm test -- apiError payment`
- 手工验证：
  - `WXPAY_CONFIG_MISSING_KEY` 会显示为“缺少必填项：证书序列号”而不是原始 key
  - 用户端支付失败不会退回英文原始报错

## P2：可单独成轮

### 主题 C：建立模型矩阵治理，不直接套用上游模型删除补丁

建议目标：

- 梳理当前 fork 中真正仍需支持的 OpenAI/Codex 模型别名
- 统一默认 fallback 策略
- 为 `prompt_cache_key` 与长上下文计费这类依赖“模型族判断”的逻辑加显式守卫

建议实施文件：

- `backend/internal/pkg/openai/constants.go`
- `backend/internal/service/openai_codex_transform.go`
- `backend/internal/service/openai_compat_prompt_cache_key.go`
- `backend/internal/service/billing_service.go`
- `frontend/src/composables/useModelWhitelist.ts`
- `frontend/src/components/keys/UseKeyModal.vue`

实施要点：

- 不直接照搬“删模型” patch，而是先做一次本地运营模型清单盘点。
- 若未来把默认 fallback 从 `gpt-5.1` 调整为 `gpt-5.4`，必须同步补上对非 GPT 模型的前缀守卫，避免 `claude-*` / `gpt-4o` 被误判为 GPT-5.4 家族。
- 任何删除模型别名的动作，都必须同步更新：
  - 后端默认模型列表
  - 计费 fallback
  - 前端白名单
  - UseKeyModal 模板
  - 相关单测

验证方式：

- `cd backend && go test -tags=unit ./internal/service ./internal/pkg/openai`
- `cd frontend && pnpm run typecheck && pnpm test -- useModelWhitelist UseKeyModal`

## 暂不执行的项目

- `CLA.md` 与 CLA Bot 工作流
- README sponsor 同步
- 上游 README 结构直接追平

## 实施顺序

### 阶段 1：支付后端 correctness

- 先落地 `wxpay.go`、`payment_config_providers.go`、`payment_order.go`
- 目标是把错误发现时机从“首单失败”前移到“保存即失败”

### 阶段 2：支付前端国际化

- 基于稳定 reason code 收口 payment 域错误展示
- 先只影响 payment 页面与后台 payment 设置页，避免全站大范围回归

### 阶段 3：模型矩阵治理专项

- 先盘点，再裁剪
- 绝不把上游 `bbc4aed3` 直接 cherry-pick 到当前 fork

## 完成标准

- 管理员保存错误的微信支付配置时，不需要真实下单就能发现问题
- 用户端与管理端看到的是稳定、可本地化的支付错误提示
- 模型治理有单独专项计划与验证清单，而不是继续叠加“兼容名债务”
- `LICENSE` 保持与上游当前区间结论一致，无需额外同步动作
