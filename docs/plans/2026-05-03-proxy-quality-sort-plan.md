# 代理节点可用性排序优化计划

## 背景

当前代理节点列表按 `id`（默认）、`name`、`status`、`created_at`、`account_count` 排序，没有利用已有的质量评分数据。系统已具备完整的质量评分基础设施（Redis 缓存 + 探测日志），但未接入排序和前端展示。

## 现状分析

### 已有能力

| 能力 | 位置 | 状态 |
|------|------|------|
| 质量评分（0-100，A-F 等级） | Redis `proxy:latency:{id}` | 已实现，已缓存 |
| 批量获取延迟/质量数据 | `ProxyLatencyCache.GetProxyLatencies()` | 已实现，单次 Redis MGet |
| 附加质量数据到列表结果 | `attachProxyLatency()` | 已实现 |
| DTO 返回质量字段 | `AdminProxyWithAccountCount` | 已实现（quality_score/grade/status） |
| 前端 Proxy 类型定义 | `Proxy` interface | 已包含 quality 字段 |
| 管理端代理列表页 | ProxySelector.vue | 展示测试结果徽章，无质量等级 |

### 缺口

1. **后端排序**：`proxyListOrder()` 不支持 `quality_score` 排序
2. **前端展示**：ProxySelector.vue 没有展示质量等级（A-F）
3. **前端排序**：ProxySelector.vue 没有按质量排序的逻辑
4. **默认排序**：代理列表默认按 ID 倒序，非按可用性

### 技术约束

质量分数存在 Redis 而非 PostgreSQL，无法直接用 SQL `ORDER BY` 排序。需要像 `account_count` 一样采用**内存排序**策略：先查全量数据、附加 Redis 质量分数、在内存中排序、再分页。

## 实施方案

### 第一步：后端 — 支持 quality_score 排序字段

**改动文件**：`backend/internal/repository/proxy_repo.go`

仿照 `account_count` 的处理方式，在 `ListWithFiltersAndAccountCount()` 中增加 `quality_score` 分支：

- `sortBy == "quality_score"` 时，走与 `account_count` 相同的内存排序路径
- 调用 `listWithQualitySort()` 新方法
- 排序规则：`quality_score` 降序（高分在前），`quality_score` 为 nil 的排在最后
- 次级排序：`quality_score` 相同时按 `id` 排序保持稳定

**改动文件**：`backend/internal/service/proxy.go`

在 `ListProxiesWithAccountCount()` 中，当 `sortBy == "quality_score"` 时先获取全量数据，调用 `attachProxyLatency()` 附加质量分数，再排序和分页。

### 第二步：后端 — 排序逻辑实现

**新增方法**：`listWithQualitySort()`

排序优先级：
1. 有 `quality_score` 的排在无分数的前面
2. `quality_score` 降序
3. 同分时，`latency_ms` 升序（延迟低的优先）
4. 同分同延迟时，`id` 升序

对于从未探测过的节点（无 Redis 缓存），`quality_score` 为 nil，统一排到最后。

### 第三步：前端 — ProxySelector 展示质量等级

**改动文件**：`frontend/src/components/common/ProxySelector.vue`

在每个代理选项的名称旁增加质量等级徽章：

- A 级：绿色徽章
- B 级：蓝绿色徽章
- C 级：黄色徽章
- D 级：橙色徽章
- F 级：红色徽章
- 未检测：灰色徽章（显示 "—"）

徽章宽度固定，避免选中不同节点时布局跳动。

### 第四步：前端 — ProxySelector 默认按可用性排序

**改动文件**：`frontend/src/components/common/ProxySelector.vue`

在组件内部对传入的 `proxies` 列表做排序（不影响外部数据）：

```typescript
const sortedProxies = computed(() => {
  return [...filteredProxies.value].sort((a, b) => {
    // 有分数的排前面
    const aScore = a.quality_score ?? -1
    const bScore = b.quality_score ?? -1
    if (bScore !== aScore) return bScore - aScore
    // 同分按延迟升序
    const aLatency = a.latency_ms ?? Infinity
    const bLatency = b.latency_ms ?? Infinity
    return aLatency - bLatency
  })
})
```

这样用户打开代理选择器时，最可靠的节点始终在顶部。

### 第五步：管理端代理列表页 — 增加质量排序列

**改动文件**：`frontend/src/views/admin/ProxiesView.vue`（或对应的代理管理页面）

- 表格新增"质量"列，显示等级徽章 + 分数
- 表头支持按质量排序（传递 `sort_by=quality_score`）

## 排序优先级定义

| 优先级 | 字段 | 方向 | 说明 |
|--------|------|------|------|
| 1 | quality_score | 降序 | 高分优先 |
| 2 | latency_ms | 升序 | 延迟低优先 |
| 3 | id | 升序 | 稳定排序 |
| 特殊 | quality_score = nil | 最后 | 未检测的节点排末尾 |

## 性能影响评估

| 场景 | 影响 | 说明 |
|------|------|------|
| 代理列表分页查询 | 低 | 质量排序走内存路径，与 `account_count` 一致 |
| Redis 批量查询 | 低 | 已有 `MGet` 批量获取，一次网络往返 |
| ProxySelector 内部排序 | 极低 | 纯前端内存排序，节点数量通常 < 100 |
| 探测周期 | 无变化 | 不改变现有探测逻辑 |

## 涉及文件清单

| 文件 | 改动类型 |
|------|----------|
| `backend/internal/repository/proxy_repo.go` | 修改：增加 quality_score 排序分支 |
| `backend/internal/service/proxy.go` | 修改：质量排序时附加 Redis 数据 |
| `frontend/src/components/common/ProxySelector.vue` | 修改：增加等级徽章 + 内部排序 |
| `frontend/src/views/admin/ProxiesView.vue` | 修改：增加质量列（如适用） |

## 不做的事

- 不修改数据库 Schema（质量数据保持 Redis-only）
- 不修改探测逻辑或评分算法
- 不新增 API 端点（复用现有列表接口）
- 不在 ProxySelector 中增加筛选/过滤功能
