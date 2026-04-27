# 调度机制与 IP 自动容错参数归属问题梳理

**Date:** 2026-04-27

**Scope:** 只梳理当前“调度机制 - 自动巡检、隔离与迁移”与“IP管理 - 代理自动测试与账号迁移”的配置归属、重复入口和潜在风险，不包含本轮代码改造。

## 结论

`自动巡检、隔离与迁移` 这组参数本质上是代理/IP 池的自动容错策略，不是通用调度机制。

它现在出现在 `调度机制` 页，主要原因是后端把临时不可调度规则和代理自动容错策略一起放进了同一个 `SchedulingMechanismSettings` 配置对象里：

- `backend/internal/service/scheduling_mechanism_settings.go:50` 将 `mechanisms` 与 `proxy_failover` 放在同一个 settings 结构中
- `backend/internal/service/setting_service.go:2435` 的注释也把它描述成“全局调度机制与代理自动容错配置”
- 前端 `frontend/src/views/admin/SchedulingMechanismsView.vue:1` 在同一个页面顶部直接渲染 `proxy_failover` 参数

但从用户心智看，这组参数控制的是“代理健康巡检、代理冷却、账号从坏代理迁移到好代理、无可用代理时临时不可调度”，应该以 `IP管理` 为主入口。

## 当前重复入口

### IP管理页已有自动容错设置

`frontend/src/views/admin/ProxiesView.vue:6` 已经在 IP 管理页顶部放了“代理自动测试与账号迁移”卡片，并提供这些字段：

- `enabled`
- `auto_test_enabled`
- `probe_interval_minutes`
- `failure_threshold`
- `cooldown_minutes`

保存时会读取并写回同一个 `/admin/settings/scheduling-mechanisms` 配置：

- `frontend/src/views/admin/ProxiesView.vue:1181`
- `frontend/src/views/admin/ProxiesView.vue:1194`

### 调度机制页又完整展示同一组配置

`frontend/src/views/admin/SchedulingMechanismsView.vue:4` 也在页面顶部展示 `自动巡检、隔离与迁移`，并提供完整 `proxy_failover` 字段：

- `enabled`
- `auto_test_enabled`
- `prefer_same_country`
- `only_openai_oauth`
- `probe_interval_minutes`
- `failure_threshold`
- `failure_window_minutes`
- `cooldown_minutes`
- `temp_unsched_minutes`
- `max_accounts_per_proxy`
- `max_migrations_per_cycle`

这些字段与 IP 管理页字段部分重叠，且保存时同样写回整个 `SchedulingMechanismSettings`：

- `frontend/src/views/admin/SchedulingMechanismsView.vue:488`

## 主要问题

### P0：同一配置存在两个编辑入口，容易互相覆盖

两个页面都不是只提交自己负责的字段，而是提交完整 `SchedulingMechanismSettings`：

- IP 管理页保存 `proxy_failover` 时，会带上 `schedulingSettings.value?.mechanisms || []`：`frontend/src/views/admin/ProxiesView.vue:1197`
- 调度机制页保存时，会 clone 整个 `settings`：`frontend/src/views/admin/SchedulingMechanismsView.vue:491`

这会带来两个风险：

- 如果用户在两个标签页分别打开 `IP管理` 和 `调度机制`，其中一个页面保存旧快照，可能覆盖另一个页面刚改过的字段。
- 如果 IP 管理页加载机制列表失败或使用空 fallback，保存代理容错配置时存在把 `mechanisms` 写成空数组的风险。

### P1：信息架构混乱

`调度机制` 页下半部分是“临时不可调度规则”：按平台、账号类型、HTTP 状态码和关键词决定账号是否进入临时不可调度状态。

`自动巡检、隔离与迁移` 则是“代理/IP 池运维策略”：定时探测代理、统计代理失败窗口、冷却代理、迁移绑定该代理的账号。

两者都会影响最终调度结果，但管理对象不同：

- 调度机制：账号错误策略、账号临时不可调度
- IP 自动容错：代理健康、代理承载、账号代理绑定迁移

把两者放在一个主页面中，会让管理员误以为“代理巡检参数是调度规则的一部分”，也解释了“为什么这些参数不应该在 IP 管理界面”的困惑。

### P1：字段重复但不完整，用户不知道哪个入口是主入口

IP 管理页展示了 5 个字段；调度机制页展示了 11 个字段。两边文案都可以保存“巡检设置”，但能力范围不同。

这会造成几个体验问题：

- 用户在 IP 管理页只能改部分策略，想改 `failure_window_minutes`、`max_accounts_per_proxy`、`max_migrations_per_cycle` 还要跳去调度机制页。
- 用户在调度机制页看到代理迁移参数，会疑惑这些 IP 资源治理参数为什么不在 IP 管理中。
- 两个入口都叫保存巡检/容错设置，缺少明确主从关系。

### P1：后端配置边界也混在一起

`SchedulingMechanismSettings` 同时包含：

- `Mechanisms []SchedulingMechanism`
- `ProxyFailover ProxyFailoverSettings`

这让 API 契约把两类不同生命周期的配置绑在一起。`ProxyFailoverService` 实际消费的是 `settings.ProxyFailover`：

- 自动巡检间隔与开关：`backend/internal/service/proxy_failover_service.go:112`
- 上游失败触发代理隔离：`backend/internal/service/proxy_failover_service.go:162`
- 代理隔离、账号迁移、无目标代理时临时不可调度：`backend/internal/service/proxy_failover_service.go:335`

因此，当前后端模型是“能用但边界不清”：代理容错是独立服务的配置，却借用了调度机制 settings 的持久化容器。

### P2：默认值在多处重复维护

同一组默认值至少出现在：

- 后端默认值：`backend/internal/service/scheduling_mechanism_settings.go:55`
- IP 管理页本地默认值：`frontend/src/views/admin/ProxiesView.vue:1067`
- 调度机制页本地默认值：`frontend/src/views/admin/SchedulingMechanismsView.vue:353`
- 内置导入规则：`docs/rules/调度机制.json`

短期看这只是重复；长期会导致默认值漂移。例如后端调整默认值后，前端加载失败或初始化时仍可能显示旧默认。

## 建议调整方向

### 推荐主方案

把 `proxy_failover` 的主编辑入口收敛到 `IP管理`：

- `IP管理` 页展示完整 11 个代理自动容错字段。
- `调度机制` 页移除顶部 `自动巡检、隔离与迁移` 编辑卡片。
- `调度机制` 页最多保留一个只读状态摘要或跳转链接，例如“代理自动容错设置已迁移到 IP 管理”。

这样最符合用户心智：凡是代理/IP 健康、代理承载、账号代理迁移，都在 IP 管理里处理。

### 后端接口建议

优先拆出独立接口，避免整包覆盖：

- `GET /api/v1/admin/settings/proxy-failover`
- `PUT /api/v1/admin/settings/proxy-failover`

初期可以仍复用当前 `SettingKeySchedulingMechanismSettings` 的存储结构，降低迁移成本；但 API 层要做到只读写 `proxy_failover` 子对象，不覆盖 `mechanisms`。

后续再考虑把持久化 key 拆为独立的 `proxy_failover_settings`。

### 文案建议

建议统一术语：

- 页面主入口：`IP管理`
- 模块名：`代理自动容错`
- 动作链路：`自动巡检、隔离与迁移`
- 调度机制页的规则模块：`临时不可调度规则`

避免在同一个菜单下同时使用“调度机制”和“代理自动容错”承载同一类配置。

## 验收标准

- `proxy_failover` 只有一个主编辑入口。
- 保存代理自动容错设置时，不会覆盖临时不可调度规则。
- 保存临时不可调度规则时，不会覆盖代理自动容错设置。
- 前端不再维护多份同字段默认值，或默认值只作为加载失败时的只读 fallback。
- 管理员可以在 IP 管理页完成代理巡检、隔离、迁移、承载上限、无目标代理冷却等全部配置。
