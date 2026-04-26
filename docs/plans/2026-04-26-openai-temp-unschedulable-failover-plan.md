# OpenAI 账号临时不可调度规则配置计划

> **日期：** 2026-04-26
> **状态：** 已执行完毕，供审查
> **触发背景：** 用户访问 `https://api.benszresearch.com/v1/responses` 时收到 502 Bad Gateway，经排查为 OpenAI 上游间歇性返回 502，sub2api 本身无故障。

---

## 问题排查过程

### 现象

客户端请求 `/v1/responses` 收到 Cloudflare 502 响应：

```
unexpected status 502 Bad Gateway: error code: 502, url: https://api.benszresearch.com/v1/responses, cf-ray: 9f24b6f01cc28553-HKG
```

### 排查步骤与结论

| 检查项 | 结果 | 说明 |
|--------|------|------|
| 后端健康检查 `GET /health` | `{"status":"ok"}` | 后端服务正常运行 |
| `/v1/responses` 路由是否存在 | 401 `INVALID_API_KEY` | 路由注册正确，认证中间件正常拦截 |
| 管理面板统计 `GET /api/v1/admin/dashboard/stats` | 正常返回数据 | uptime=15104s（约 4.2 小时），总请求 25,669 |
| 运维错误日志 `GET /api/v1/admin/ops/request-errors` | 大量 502 错误记录 | 错误源为 `upstream_http`，所有者 `provider` |

**结论：502 来自 OpenAI 上游，不是 sub2api 的问题。**

### 错误分布详情

**时间窗口：** 2026-04-26 17:55:51 ~ 18:02:33（约 7 分钟突发）

**最近 100 条错误按模型分布：**

| 模型 | 错误次数 | 涉及账号 |
|------|---------|---------|
| gpt-5.5 | 53 | kxsw1-team-20260421-02 (30)、kxsw1-team-20260424 (23) |
| gpt-5.4 | 30 | kxsw1-team-20260424 (30) |
| gpt-5.4-mini | 14 | kxsw1-team-20260424 (14) |

**关键观察：**

- `error_source: upstream_http` + `error_owner: provider` — 确认 502 来自 OpenAI
- `retry_count: 0` — 每个 ops 记录的内部重试计数为 0（但 failover 本身在运作）
- Failover 机制正常工作：错误先集中在 account 11，之后自动切换到 account 10
- 出错的账号 10 和 11 共用同一代理 `Decodo-JP-帐号6`

---

## 临时不可调度机制说明

### 机制原理

每个账号的 `credentials` JSON 中可配置 `temp_unschedulable_enabled`（总开关）和 `temp_unschedulable_rules`（规则数组）。当上游返回的 HTTP 响应匹配某条规则时，系统自动将该账号标记为「临时不可调度」并设定冷却时长。调度器在选择账号时会跳过处于冷却期的账号。

### 规则数据结构

```json
{
  "error_code": 502,
  "keywords": ["bad gateway", "server error"],
  "duration_minutes": 3,
  "description": "说明文本"
}
```

- `error_code`：匹配的上游 HTTP 状态码
- `keywords`：响应体关键词列表（**任一**匹配即触发，大小写不敏感）；若响应体为空则不匹配
- `duration_minutes`：冷却时长（分钟）
- `description`：人工可读的说明

### 触发后的数据流

1. 上游请求返回匹配的状态码 + 响应体包含关键词
2. `ratelimit_service.tryTempUnschedulable()` 匹配规则
3. 调用 `repository.SetTempUnschedulable()` 写入 DB + Redis 缓存
4. Redis 使用 Lua 脚本保证「只延长不缩短」
5. 调度器 `IsSchedulable()` 检查 `TempUnschedulableUntil`，若未过期则跳过
6. 冷却期结束后账号自动恢复可调度

### 关键代码位置

| 组件 | 文件 |
|------|------|
| 规则结构体定义 | `backend/internal/service/account.go` |
| 错误匹配与触发 | `backend/internal/service/ratelimit_service.go` |
| 调度器检查 | `backend/internal/service/account.go` → `IsSchedulable()` |
| DB 读写 | `backend/internal/repository/account_repo.go` |
| Redis 缓存 | `backend/internal/repository/temp_unsched_cache.go` |
| 管理员 API | `backend/internal/handler/admin/account_handler.go` |
| 状态结构体 | `backend/internal/service/temp_unsched.go` |

---

## 本次配置的规则

### 规则设计考量

- **502 Bad Gateway：** OpenAI 或 Cloudflare 网关层的瞬时故障，通常短时间内恢复 → 冷却 **3 分钟**
- **503 Service Unavailable：** OpenAI 过载，恢复需要较长时间 → 冷却 **5 分钟**
- **500 Internal Server Error：** OpenAI 内部错误，通常瞬时 → 冷却 **3 分钟**

### 关键词选择理由

每个状态码配置 3 个关键词，确保能匹配大多数上游错误响应格式：

| 状态码 | 关键词 | 匹配场景 |
|--------|--------|---------|
| 502 | `bad gateway` | Cloudflare / OpenAI 502 页面 |
| 502 | `server error` | OpenAI JSON 错误响应 `{"error":{"type":"server_error"}}` |
| 502 | `upstream` | 部分网关错误页面 |
| 503 | `overloaded` | OpenAI 过载消息 `"That model is currently overloaded"` |
| 503 | `unavailable` | Cloudflare 503 页面 `"service unavailable"` |
| 503 | `capacity` | 容量不足类消息 |
| 500 | `internal` | `"internal server error"` |
| 500 | `error` | 通用错误响应 |
| 500 | `server` | `"server error"` |

### 最终规则 JSON

```json
{
  "temp_unschedulable_enabled": true,
  "temp_unschedulable_rules": [
    {
      "error_code": 502,
      "keywords": ["bad gateway", "server error", "upstream"],
      "duration_minutes": 3,
      "description": "上游 502 Bad Gateway，临时冷却 3 分钟"
    },
    {
      "error_code": 503,
      "keywords": ["overloaded", "unavailable", "capacity"],
      "duration_minutes": 5,
      "description": "上游 503 过载，临时冷却 5 分钟"
    },
    {
      "error_code": 500,
      "keywords": ["internal", "error", "server"],
      "duration_minutes": 3,
      "description": "上游 500 内部错误，临时冷却 3 分钟"
    }
  ]
}
```

---

## 执行记录

### 操作方式

通过远程管理 API (`PUT /api/v1/admin/accounts/:id`) 逐个更新每个账号的 `credentials` 字段。

由于 credentials 更新是**整替换**（不是合并），每个账号的操作流程为：

1. `GET /api/v1/admin/accounts/:id` — 获取完整凭证
2. 在现有凭证中追加 `temp_unschedulable_enabled` 和 `temp_unschedulable_rules`
3. `PUT /api/v1/admin/accounts/:id` — 写回完整凭证

### 执行结果

所有 **10 个 OpenAI OAuth 账号**均已配置完毕：

| 账号 ID | 账号名 | 代理 | 规则数 | 结果 |
|---------|--------|------|--------|------|
| 1 | kxsw1-team-20260407 | Decodo-JP-帐号2 | 3 | ✓ |
| 2 | kxsw1-team-20260402 | Decodo-JP-帐号2 | 3 | ✓ |
| 3 | kxsw1-team-20260324 | Decodo-JP-帐号17 | 3 | ✓ |
| 5 | kxsw1-plus-01 | Decodo-JP-帐号16 | 3 | ✓ |
| 6 | kxsw1-team-20260413 | Decodo-JP-帐号3 | 3 | ✓ |
| 10 | kxsw1-team-20260421-02 | Decodo-JP-帐号6 | 3 | ✓ |
| 11 | kxsw1-team-20260424 | Decodo-JP-帐号6 | 3 | ✓ |
| 12 | kxsw2-team-20260425-01 | 住宅JP-帐号3 | 3 | ✓ |
| 13 | kxsw2-team-20260425-02 | 住宅JP-帐号3 | 3 | ✓ |
| 14 | kxsw2-team-20260425-03 | 住宅JP-帐号2 | 3 | ✓ |

---

## 预期效果

当 OpenAI 上游再次返回 502/503/500 时：

1. 首个出错的账号被标记为临时不可调度（3~5 分钟）
2. 调度器自动选择同组内其他健康账号继续服务
3. 用户端无感知，请求正常完成
4. 冷却期结束后账号自动恢复

### 与原有 Failover 机制的协作

sub2api 本身已有基于 `failedAccountIDs` 的 failover 循环（`openai_gateway_handler.go`），两者协作方式：

| 层级 | 机制 | 作用范围 |
|------|------|---------|
| 请求内 | failover 循环 | 单次请求内切换账号 |
| 跨请求 | 临时不可调度 | 后续请求也跳过问题账号 |

临时不可调度是对 failover 的补充——避免后续请求继续命中已知有问题的账号，减少无效上游调用。

---

## 后续可考虑的优化

- **代理多样性：** 当前账号 10+11 共用 Decodo-JP-帐号6，账号 1+2 共用 Decodo-JP-帐号2，账号 12+13 共用住宅JP-帐号3。如果某个代理到 OpenAI 的链路出问题，共用该代理的所有账号会同时触发冷却。可考虑为关键账号分配独立代理。
- **429 Rate Limit 规则：** 当前未配置 429 的临时不可调度规则，因为 429 已有独立的 `rate_limited_until` 机制。如果发现现有 429 处理不够及时，可以追加。
- **冷却时长调优：** 3~5 分钟是初始值，可基于实际运维数据调整。如果 502 通常在 1 分钟内恢复，可缩短冷却时长以减少账号闲置。
