# 代理故障转移机制优化计划

## 背景

当前代理故障转移存在三个不足：
1. 冷却结束后代理立即恢复全量流量，无渐进恢复机制
2. 仅默认保护 OpenAI OAuth 账号，其他平台（Anthropic、Gemini、Antigravity）不触发代理级迁移
3. 源代理地理信息缺失时完全不迁移，账号只做临时封禁

## 涉及文件

| 文件 | 变更内容 |
|------|---------|
| [proxy_failover_service.go](backend/internal/service/proxy_failover_service.go) | 核心：状态机扩展、`RecordUpstreamSuccess`、`listHealthyTargetProxies` 降级逻辑 |
| [scheduling_mechanism_settings.go](backend/internal/service/scheduling_mechanism_settings.go) | 配置：新增字段、改默认值 |
| [ratelimit_service.go](backend/internal/service/ratelimit_service.go) | 新增 `recordProxyUpstreamSuccess`，在 `HandleUpstreamError` 对面调用 |
| [gateway_service.go](backend/internal/service/gateway_service.go) | 成功路径调用 `recordProxyUpstreamSuccess`（约 10 处） |
| [ProxiesView.vue](frontend/src/views/admin/ProxiesView.vue) | 新增 half-open 配置 UI，更新 onlyOpenAIOAuth 文案 |
| [settings.ts](frontend/src/api/admin/settings.ts) | TypeScript 类型新增字段 |
| [zh.ts](frontend/src/i18n/locales/zh.ts) / [en.ts](frontend/src/i18n/locales/en.ts) | i18n 新增文案 |

---

## 实现步骤

### 第一步：地理信息缺失降级迁移（优化 3）

最简单、无配置变更、纯后端逻辑修改。

**修改 `listHealthyTargetProxies`（第 488-555 行）：**

1. 删除第 498-500 行的早返回（`!sourceGeo.hasCountry()` → `return nil, nil`）
2. 重构候选收集为两个列表：
   - `geoMatched`：与源代理同国家/地区的健康代理
   - `fallback`：不同地区或源无地理信息时的所有健康代理
3. 合并排序：geoMatched 排前面（score -1000），fallback 排后面，各组内按 AccountCount 升序 → LatencyMs 升序
4. 当源代理无地理信息时，所有健康候选都进入 fallback 列表

**关键逻辑伪代码：**

```go
sourceHasGeo := sourceGeo.hasCountry()
var geoMatched, fallback []proxyFailoverCandidate

for _, proxy := range proxies {
    // ... 现有的排除/冷却/满载/健康检查 ...
    if sourceHasGeo && sameProxyGeoLocation(sourceGeo, targetGeo) {
        candidate.score -= 1000  // 同区优先
        geoMatched = append(geoMatched, candidate)
    } else {
        fallback = append(fallback, candidate)
    }
}

// 合并：geoMatched 在前，fallback 在后
result = append(sorted(geoMatched), sorted(fallback)...)
```

### 第二步：保护所有平台（优化 2）

**修改 `scheduling_mechanism_settings.go`：**
- 第 66 行：`OnlyOpenAIOAuth: true` → `OnlyOpenAIOAuth: false`

**前端 i18n 更新：**
- `zh.ts`：`onlyOpenAIOAuth` 描述改为"仅保护 OpenAI OAuth（关闭后保护所有平台）"
- `en.ts`：`onlyOpenAIOAuth` 描述改为"Only OpenAI OAuth (disable to protect all platforms)"

**前端默认值：**
- `ProxiesView.vue`：`only_openai_oauth` 默认值改为 `false`

**向后兼容**：已保存过 `only_openai_oauth: true` 的用户不受影响，DB 中存储的值优先于默认值。

### 第三步：半开状态熔断器（优化 1）

#### 3a. 扩展 `proxyFailureState` 状态机

新增枚举和字段：

```go
type proxyHealthState int

const (
    proxyHealthClosed   proxyHealthState = iota // 正常
    proxyHealthOpen                              // 隔离中
    proxyHealthHalfOpen                          // 半开试探中
)

// proxyFailureState 新增字段：
healthState    proxyHealthState  // 当前状态
cooldownCount  int               // 连续冷却次数（用于加倍冷却）
```

状态转换：
```
Closed ──(failures ≥ threshold)──→ Open
Open ──(cooldown 到期 + 探测成功)──→ HalfOpen
HalfOpen ──(真实请求成功)──→ Closed
HalfOpen ──(真实请求失败 或 探测失败)──→ Open（冷却时间 ×backoff）
```

#### 3b. 新增配置字段

`ProxyFailoverSettings` 新增：

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `half_open_probe_accounts` | int | 2 | 半开状态允许迁回的试探账号数 |
| `cooldown_backoff_factor` | int | 2 | 连续失败时冷却时间的翻倍因子 |
| `max_cooldown_minutes` | int | 120 | 冷却时间上限 |

`normalizeProxyFailoverSettings` 中加校验：`half_open_probe_accounts` 1-10，`cooldown_backoff_factor` 1-4，`max_cooldown_minutes` 不超过 240。

#### 3c. 修改 `isolateProxy` 的 defer 块（第 341-349 行）

冷却时长支持 backoff：

```go
defer func() {
    s.mu.Lock()
    state := s.ensureState(proxyID)
    state.migrationRunning = false
    state.healthState = proxyHealthOpen
    // 计算冷却时长：base × factor^cooldownCount，上限 max
    cooldown := time.Duration(settings.CooldownMinutes) * time.Minute
    for i := 0; i < state.cooldownCount; i++ {
        cooldown *= time.Duration(settings.CooldownBackoffFactor)
    }
    if cooldown > time.Duration(settings.MaxCooldownMinutes)*time.Minute {
        cooldown = time.Duration(settings.MaxCooldownMinutes) * time.Minute
    }
    state.unhealthyUntil = time.Now().Add(cooldown)
    state.cooldownCount++
    s.mu.Unlock()
}()
```

#### 3d. 修改 `runSingleProxyProbe`（第 199-230 行）

探测成功时，如果当前是 Open 且冷却已到期，进入 HalfOpen 并迁回试探账号：

```go
if exitInfo, latencyMs, err := s.proxyProber.ProbeProxy(...); err == nil {
    // ... 存储 latency ...

    s.mu.Lock()
    state := s.ensureState(proxy.ID)
    if state.healthState == proxyHealthOpen && time.Now().After(state.unhealthyUntil) {
        // 冷却到期 + 探测成功 → 进入 HalfOpen
        state.healthState = proxyHealthHalfOpen
        s.mu.Unlock()
        go s.migrateAccountsForHalfOpenProbe(context.Background(), proxy, settings)
    } else {
        s.mu.Unlock()
    }
}
```

#### 3e. 新增 `migrateAccountsForHalfOpenProbe` 方法

从临时封禁的账号中选择最多 `HalfOpenProbeAccounts` 个迁回原代理：

```go
func (s *ProxyFailoverService) migrateAccountsForHalfOpenProbe(
    ctx context.Context, proxy *ProxyWithAccountCount, settings ProxyFailoverSettings,
) {
    // 1. 查找因该代理被临时封禁的账号（Extra 中记录了 source proxy id）
    // 2. 选择最多 halfOpenProbeAccounts 个
    // 3. 迁回原代理，清除临时封禁
    // 4. 更新调度器缓存
}
```

#### 3f. 修改 `recordFailure`（第 263-303 行）

HalfOpen 状态下收到失败 → 立即回退到 Open：

```go
if state.healthState == proxyHealthHalfOpen {
    state.healthState = proxyHealthOpen
    state.halfOpenRemaining = 0
    state.migrationRunning = true
    return true  // 触发重新隔离
}
```

#### 3g. 新增 `RecordUpstreamSuccess` 方法

真实请求成功时，如果代理在 HalfOpen 状态，转为 Closed：

```go
func (s *ProxyFailoverService) RecordUpstreamSuccess(ctx context.Context, account *Account) {
    if account == nil || account.ProxyID == nil { return }
    proxyID := *account.ProxyID
    s.mu.Lock()
    state := s.ensureState(proxyID)
    if state.healthState == proxyHealthHalfOpen {
        // 真实请求成功 → 确认恢复，进入 Closed
        state.healthState = proxyHealthClosed
        state.failCount = 0
        state.windowStartedAt = time.Time{}
        state.cooldownCount = 0
        // ... 清理状态 ...
    }
    s.mu.Unlock()
}
```

#### 3h. 在成功路径挂接调用

在 `ratelimit_service.go` 新增 `recordProxyUpstreamSuccess` 方法：

```go
func (s *RateLimitService) recordProxyUpstreamSuccess(ctx context.Context, account *Account) {
    if s.proxyFailoverService != nil {
        s.proxyFailoverService.RecordUpstreamSuccess(ctx, account)
    }
}
```

调用位置：在 `gateway_service.go` 的主要成功路径中调用（约 10 处 `HandleUpstreamError` 的对应成功分支），优先覆盖 `gateway_service.go`、`openai_gateway_service.go`、`openai_gateway_chat_completions.go`、`openai_gateway_messages.go` 等高频路径。

#### 3i. 修改 `markProxyHealthy`

当前 `markProxyHealthy` 直接清除所有状态。修改为：仅在 Closed 或 HalfOpen 状态时清除（Open 状态下不响应 markProxyHealthy，由 cooldown 到期 + 探测驱动转换）。

#### 3j. 前端变更

`ProxyFailoverSettings` 接口新增 3 个字段，`ProxiesView.vue` 新增对应输入控件。i18n 新增标签。

---

## 验证方案

### 单元测试

- `proxy_failure_state` 状态转换：Closed → Open → HalfOpen → Closed / Open
- backoff 计算正确性
- `listHealthyTargetProxies` 在无地理信息时返回 fallback 候选
- `OnlyOpenAIOAuth=false` 时所有平台账号都能触发 `RecordUpstreamFailure`

### 集成验证

1. 模拟代理故障：手动设置代理为不可用，观察账号迁移和临时封禁
2. 半开恢复：冷却到期后观察是否只迁回 2 个账号试探
3. 跨区降级：删除源代理地理信息后触发隔离，观察是否降级迁移到其他区代理
4. 多平台覆盖：用不同平台账号触发 5xx 错误，观察是否都触发代理级迁移

### 运行命令

```bash
cd backend && go test -tags=unit ./internal/service/... -run ProxyFailover -v
cd backend && go test -tags=unit ./internal/service/... -run TestSchedulingMechanism -v
cd frontend && pnpm run typecheck && pnpm test
```
