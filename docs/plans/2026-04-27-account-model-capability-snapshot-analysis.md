# 账号模型能力快照问题分析

> **日期：** 2026-04-27
> **状态：** 根因已定位，待制定修复方案
> **触发背景：** 旧账号创建时模型 A 尚未发布；软件后续更新并支持模型 A 后，该旧账号仍无法自动支持模型 A。

---

## 问题摘要

当前账号模型能力采用了“创建时快照”策略：前端在创建账号时默认把当时平台内置模型列表填入白名单，并在提交时写入账号的 `credentials.model_mapping`。后端又把 `model_mapping` 同时作为“模型映射表”和“账号模型白名单”使用。

因此，只要旧账号已经落库了 `model_mapping`，它支持的模型集合就被固定在创建时刻。后续开发者在代码中新增默认模型，例如新增 OpenAI `gpt-5.5` 或未来模型 A，只会更新代码里的默认模型列表，不会自动修改旧账号已有的 `credentials.model_mapping` JSON。

最终表现就是：新建账号可能支持新模型，旧账号却因为静态白名单缺少该模型而被调度器过滤。

## 现象复盘

典型场景：

1. 管理员在模型 A 发布前添加账号。
2. 创建弹窗默认处于“模型白名单”模式，并把当时平台模型列表写入账号配置。
3. 模型 A 发布后，项目代码新增默认模型 A。
4. 旧账号数据库中的 `credentials.model_mapping` 没有模型 A。
5. 用户请求模型 A 时，调度器检查旧账号的模型支持能力，发现 mapping 中不存在模型 A，于是跳过该账号。

这不是单个模型漏配，而是账号能力建模方式存在时间漂移：代码中的“当前默认模型集”和数据库里的“账号创建时模型快照”会逐渐分叉。

## 证据链

### 前端创建时自动固化模型列表

创建账号弹窗默认使用 `whitelist` 模式：

- `frontend/src/components/account/CreateAccountModal.vue:3020`

弹窗打开时会把当前平台内置模型列表填入 `allowedModels`：

- `frontend/src/components/account/CreateAccountModal.vue:3346`

切到白名单模式时也会重新填充当前平台模型列表：

- `frontend/src/components/account/CreateAccountModal.vue:3496`

提交时，白名单会被转换成 `model_mapping`：

- `frontend/src/composables/useModelWhitelist.ts:418`
- `frontend/src/components/account/CreateAccountModal.vue:4162`

转换规则是 `model -> model`，也就是：

```json
{
  "model_mapping": {
    "old-model-1": "old-model-1",
    "old-model-2": "old-model-2"
  }
}
```

这会把“当时可见的默认模型列表”变成账号持久化配置。

### 后端把 model_mapping 当作白名单

后端 `Account.IsModelSupported()` 的语义是：

- 没有 `model_mapping`：允许所有模型。
- 有 `model_mapping`：只有 key 命中的模型才支持，支持精确匹配和通配符。

关键位置：

- `backend/internal/service/account.go:631`

这意味着 `model_mapping` 一旦存在，就从“可选映射”变成“显式限制”。

### 调度器按账号白名单过滤

OpenAI 调度路径会检查 `account.IsModelSupported(requestedModel)`。模型不支持时，该账号不会进入候选集：

- `backend/internal/service/openai_account_scheduler.go:625`
- `backend/internal/service/openai_gateway_service.go:1503`

所以旧账号缺少新增模型时，不是转发阶段才失败，而是在调度阶段就被排除。

### 模型列表接口也会暴露静态集合

`/v1/models` 会聚合可调度账号的 `model_mapping` key。如果账号池里存在 mapping，就返回这些静态 key；只有没有任何账号配置 mapping 时才回落到默认模型列表：

- `backend/internal/service/gateway_service.go:8718`
- `backend/internal/handler/gateway_handler.go:844`

这会进一步强化用户侧的错觉：平台代码已经支持新模型，但某些分组或账号池看不到新模型。

## 根因判断

根因不是“新模型没有加入默认模型列表”，而是以下设计叠加造成的：

- `model_mapping` 承担了两个职责：模型别名映射，以及账号模型白名单。
- 前端默认白名单模式会把“全量默认模型”写入账号，而不是留空表示“继承平台默认模型集”。
- 数据库中没有字段区分“用户主动限制的白名单”和“创建时 UI 自动填充的默认快照”。
- 后续模型新增只更新代码常量，不会迁移或刷新旧账号的 `credentials.model_mapping`。
- 调度层、模型列表层、测试模型列表层都把已有 mapping 视为账号真实能力边界。

本质上，系统缺少“动态继承默认模型集”的概念，只有“无限制”和“静态白名单/映射”两种状态。

## 影响面

受影响范围：

- 创建时使用默认白名单并保存了 `credentials.model_mapping` 的旧账号。
- OpenAI、Anthropic、Gemini、Bedrock 等使用通用 `model_mapping` 机制的平台。
- 管理端账号测试模型列表。
- 网关 `/v1/models` 返回结果。
- 调度器候选账号选择。

相对不受影响或影响较小的场景：

- 账号没有 `model_mapping`：后端语义是允许所有模型。
- 使用合理通配符映射的账号，例如 `claude-*`、`gpt-5*`。
- OpenAI 自动透传开启且相关路径显式回落默认模型的账号，但如果旧 mapping 被保留，仍需确认各调度路径是否完全绕过限制。
- Antigravity 当前有默认映射兜底和部分自动补 passthrough，但自定义 mapping 仍可能产生类似时间漂移。

## 修复方向

建议不要简单地“每次新增模型就批量追加到所有账号”。那会误伤用户主动限制模型的账号。更稳妥的方向是把账号模型策略显式化：

### 策略显式化

引入明确的模型能力策略，例如：

- `inherit_default`：继承平台当前默认模型集，未来新增默认模型自动可用。
- `whitelist`：用户主动维护静态白名单。
- `mapping`：用户主动维护模型映射。

这样可以把“继承默认”和“锁定白名单”从语义上拆开。

### 创建默认改为继承

新建账号默认不应把全量模型写入 `model_mapping`。默认应保存为空或显式策略为 `inherit_default`，表示账号随平台默认模型集演进。

只有管理员主动选择“限制模型”或“配置映射”时，才写入白名单或映射。

### 旧数据迁移要保守

旧账号已有 `model_mapping` 时，无法完全知道它是用户主动配置，还是历史 UI 自动填充。迁移可以考虑：

- 如果 mapping 是 `model -> model` 且集合等于某个历史平台默认模型集，则可标记为 `inherit_default`。
- 如果 mapping 包含非等值映射、自定义模型或明显删减，则保留为用户配置。
- 对无法判定的账号生成管理端提示或批量审计列表，让管理员确认是否切换为继承默认。

### 模型列表接口同步策略

`/v1/models` 和管理端账号测试模型列表应根据策略返回：

- `inherit_default`：返回当前平台默认模型列表。
- `whitelist`：返回白名单 key。
- `mapping`：返回请求侧可见的 mapping key。

避免只要存在任意 mapping 就把整个分组模型列表固定成静态集合。

## 验收建议

后续修复时应补以下回归测试：

- 创建新账号默认不写入全量 `model_mapping`。
- 新增默认模型后，`inherit_default` 账号自动支持新模型。
- 用户主动白名单账号不会自动获得新模型。
- 用户主动映射账号按 mapping key 判断支持能力。
- `/v1/models` 对继承默认账号返回最新默认模型。
- 调度器在请求新增模型时能选择继承默认的旧账号。
- 旧数据迁移只转换可确定为历史默认快照的账号，不覆盖用户主动限制。

## 当前结论

这个问题暴露的是账号添加策略的固有缺陷：系统把“账号当前能力”存成了创建时的静态结果，而不是存“账号能力策略”。只要模型生态持续变化，静态快照就会反复制造旧账号落后的问题。

后续修复的重点应是把模型能力从“结果快照”改成“策略 + 当前默认模型集解析”，让账号在默认场景下自然继承平台演进，同时保留管理员主动限制和映射的能力。
