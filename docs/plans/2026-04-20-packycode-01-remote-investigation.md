# `packycode-01` 远程排查记录

## 背景

- 排查时间：2026-04-20（Asia/Shanghai）
- 排查方式：仅使用根目录 `remote.env` 中的只读远程测试站点凭据进行查询与只读复现
- 目标账号：`packycode-01`

## 结论

`packycode-01` 不能在站内正常工作的根因，不是上游 API Key 失效，也不是账号未启用/未绑组，而是**当前项目对 OpenAI API Key 账号 `base_url` 的 Responses 路径拼接方式，与 `packyapi` 这类第三方上游的实际可用路径不兼容**。

更具体地说：

- 后台“测试账号”逻辑会把该账号的 `base_url=https://api-slb.packyapi.com` 拼成 `https://api-slb.packyapi.com/responses`
- 真实网关转发逻辑会把同一个 `base_url` 拼成 `https://api-slb.packyapi.com/v1/responses`
- 对 `packyapi` 来说，这两个路径行为不同：
  - `/responses` 可用
  - `/v1/responses` 与 `/v1/responses/compact` 返回 `503 model_not_found`

因此，这个账号会表现成：

- 在后台测试里“看起来是好的”
- 但实际用户流量走网关时却失败

## 关键证据

### 账号本身不是坏的

远程账号详情显示：

- 名称：`packycode-01`
- 平台：`openai`
- 类型：`apikey`
- 状态：`active`
- `schedulable=true`
- 已绑定分组：`GPT_Standard`、`GPT_Premium`、`GPT_Usage-based`、`GPT_Ultra`

说明这不是“账号停用”或“没有绑定分组”的问题。

### 后台测试成功

远程调用：

- `POST /api/v1/admin/accounts/8/test`

返回 SSE 事件：

- `test_start`
- `test_complete`，`success=true`

这说明后台测试链路认为账号可用。

### 实际上游复现显示路径差异

对同一把上游 token 做最小复现：

- `POST https://api-slb.packyapi.com/responses`
  - `model=gpt-4.1-mini`
  - HTTP `200`
- `POST https://api-slb.packyapi.com/responses`
  - `model=gpt-5-codex`
  - HTTP `200`
- `POST https://api-slb.packyapi.com/responses/compact`
  - `model=gpt-5-codex`
  - HTTP `200`

但改成当前网关真实会走的路径后：

- `POST https://api-slb.packyapi.com/v1/responses`
  - `model=gpt-4.1-mini`
  - HTTP `503`
  - 错误：`model_not_found`
- `POST https://api-slb.packyapi.com/v1/responses`
  - `model=gpt-5-codex`
  - HTTP `503`
  - 错误：`model_not_found`
- `POST https://api-slb.packyapi.com/v1/responses/compact`
  - `model=gpt-5-codex`
  - HTTP `503`
  - 错误：`model_not_found`

上游返回的关键信息是：

- “分组 `default` 下模型 `gpt-4.1-mini` 无可用渠道（distributor），请尝试切换其他分组”
- “分组 `default` 下模型 `gpt-5-codex` 无可用渠道（distributor），请尝试切换其他分组”

这说明 `packyapi` 的 `/v1/responses` 路由并不等价于 `/responses`。

## 本地代码对照

### 后台测试链路

`backend/internal/service/account_test_service.go:464`

OpenAI API Key 账号测试时，URL 直接拼接为：

- `normalizedBaseURL + "/responses"`

### 真实网关链路

`backend/internal/service/openai_gateway_service.go:3162`

真实网关会调用 `buildOpenAIResponsesURL(validatedURL)`。

`backend/internal/service/openai_gateway_service.go:4161`

该函数的规则是：

- base 以 `/responses` 结尾：原样返回
- base 以 `/v1` 结尾：追加 `/responses`
- 其他情况：追加 `/v1/responses`

所以当前 `base_url=https://api-slb.packyapi.com` 会被网关拼成：

- `https://api-slb.packyapi.com/v1/responses`

而不是后台测试使用的：

- `https://api-slb.packyapi.com/responses`

## 为什么它会表现成“后台能测通，但站里不能用”

因为这两个路径不一致：

1. 后台测试打的是 `.../responses`
2. 实际用户请求打的是 `.../v1/responses`
3. 对当前第三方上游来说，这两个路径不是同一个语义
4. 于是出现“测试成功、线上失败”的假象

## 非根因但值得记录的附带发现

在本次排查中，远程 `GET /api/v1/admin/accounts/8` 响应中直接返回了完整上游 `api_key`。

这意味着：

- 管理端账号详情接口目前会把敏感上游凭据回传给调用方
- 即使这是管理员接口，也建议评估是否需要改成只写或脱敏返回

这个问题不是 `packycode-01` 不能工作的直接根因，但属于额外的安全风险。

## 当前判断

本次问题的最小根因可以表述为：

> `packycode-01` 的第三方 OpenAI 兼容上游仅对 `/responses` 路径工作，而当前网关对该 `base_url` 自动拼成了 `/v1/responses`，导致真实请求落到错误的上游路由并返回 `503 model_not_found`。

## 后续建议

本次只做调查，不做远程写操作。后续如果要修，可以优先考虑以下两个方向之一：

- 方案 A：让 OpenAI API Key 账号测试与真实网关共用同一套 Responses URL 构造逻辑，避免“测试路径”和“真实路径”不一致
- 方案 B：为第三方 OpenAI 兼容上游增加更明确的 base URL / endpoint 兼容策略，不再默认把所有非 `/v1` 地址强行补成 `/v1/responses`

如果继续落地修复，建议先补一个回归场景：

- `base_url=https://api-slb.packyapi.com`
- 后台测试与真实网关应命中同一条最终上游路径
