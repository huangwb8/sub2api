# ChatGPT OAuth 实验性图片生成

本文说明 Sub2API 中 OpenAI OAuth 图片实验分支的当前边界、启用方式与回滚方式。

## 能力边界

- 当前只支持 `POST /v1/images/generations`
- 当前只支持非流式请求：`stream=true` 会被明确拒绝
- `POST /v1/images/edits` 当前不会放开给 OAuth 账号
- 该能力默认关闭；关闭时图片链路保持现有“仅 API Key”语义
- 当前实现的实验策略只有 `api_platform_images_with_oauth`
- 当前 capability probe 采用“人工确认结果 + 短 TTL 缓存”的保守模式，不会在热路径自动发起真实探测请求，避免误产生图片成本或触发上游风控

## 全局配置

在后端配置中增加以下网关项：

```yaml
gateway:
  openai_oauth_images_experimental_enabled: false
  openai_oauth_images_probe_ttl_seconds: 600
```

- `openai_oauth_images_experimental_enabled`
  - `false`：完全关闭 OAuth 图片实验分支
  - `true`：允许继续按账号开关和 probe 结果筛选
- `openai_oauth_images_probe_ttl_seconds`
  - OAuth 图片 capability 缓存 TTL，默认 `600`

## 账号级 extra 字段

为目标 OpenAI OAuth 账号配置以下 `extra` 字段：

```json
{
  "openai_oauth_images_experimental": true,
  "openai_oauth_images_strategy": "api_platform_images_with_oauth",
  "openai_oauth_images_probe_supported": true,
  "openai_oauth_images_probe_reason": "manual_probe_passed",
  "openai_oauth_images_probe_status": 204
}
```

字段说明：

- `openai_oauth_images_experimental`
  - 账号级总开关；未开启时即使全局已开，也不会参与图片调度
- `openai_oauth_images_strategy`
  - 当前仅支持 `api_platform_images_with_oauth`
- `openai_oauth_images_probe_supported`
  - 是否已确认该账号可进入实验分支
- `openai_oauth_images_probe_reason`
  - 最近一次人工 probe 或排查摘要，便于运维回看
- `openai_oauth_images_probe_status`
  - 最近一次 probe 的状态码摘要；仅用于诊断展示

## 调度行为

- 同组同时存在 API Key 与 OAuth 图片可用账号时，当前实现默认优先消耗 API Key
- 只有 API Key 不可用时，才会回退尝试已通过实验开关与 probe 的 OAuth 账号
- 不满足条件的 OAuth 账号会被跳过，并保留可解释的拒绝原因

## 失败分类

当前会显式区分以下几类失败：

- `oauth_images_experimental_disabled`
- `oauth_images_account_disabled`
- `oauth_images_probe_failed`
- `oauth_images_strategy_unsupported`
- `oauth_images_stream_not_supported`
- `oauth_images_edits_not_supported`

## 回滚方式

推荐按以下顺序回滚：

1. 将 `gateway.openai_oauth_images_experimental_enabled` 设为 `false`
2. 重启后端服务，使全局配置立即生效
3. 如需彻底停用，可移除账号 `extra` 中的 `openai_oauth_images_*` 字段

回滚后：

- OAuth 账号不再参与图片调度
- 现有 OpenAI API Key 图片链路保持不变
- 不需要数据库迁移或数据修复

## 验证命令

```bash
cd backend
go test -tags=unit ./internal/service -run 'OpenAI.*Images|Account.*OpenAI|OAuth.*Images'
go test -tags=unit ./internal/handler -run 'OpenAI.*Images|Gateway.*ErrorFallback'
```

## 当前取舍

- 本轮没有引入自动真实 probe，因为那会带来额外图片成本与不稳定上游副作用
- 本轮没有放开 `images/edits`，也没有让 OAuth 支持 `stream=true`
- 本轮没有改前端账号表单；实验开关与 probe 字段默认走运维/管理接口维护
