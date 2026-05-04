# sub2api 源码核对地图

使用本技能时，采集数据只能说明现象；最终建议需要结合源码理解机制。优先核对以下位置：

| 主题 | 文件 | 关注点 |
|------|------|--------|
| 管理员鉴权 | `backend/internal/server/middleware/admin_auth.go` | Admin API Key 使用 `x-api-key`，管理员 JWT 使用 `Authorization: Bearer` |
| 管理端用量列表与统计 | `backend/internal/handler/admin/usage_handler.go` | `start_date/end_date/timezone`、分页、过滤与 `exact_total` 语义 |
| Dashboard 汇总 | `backend/internal/handler/admin/dashboard_handler.go` | 统计、趋势、模型、分组、用户排行、snapshot-v2 的接口口径 |
| 运维监控观察窗口 | `backend/internal/handler/admin/ops_handler.go`、`backend/internal/handler/admin/ops_dashboard_handler.go`、`backend/internal/handler/admin/ops_snapshot_v2_handler.go`、`frontend/src/views/admin/ops/OpsDashboard.vue` | Ops Dashboard、错误列表、请求明细和 snapshot-v2 的默认观察窗口为最近 `5m`；显式 `time_range/start_time/end_time` 仍按请求参数覆盖 |
| 管理端插件设置 | `backend/internal/handler/admin/plugin_handler.go`、`backend/internal/server/routes/admin.go`、`backend/internal/service/plugin_service.go` | `/api/v1/admin/settings/plugins` 列表/创建/更新/启停/检查配置接口、插件实例目录规则：源码仓库优先使用项目根 `./plugins/{插件名}`，Docker/非仓库运行环境回退到 `${DATA_DIR}/plugins/{插件名}` 或 `/app/data/plugins/{插件名}`；以及 `api-prompt` 本地模板读写、字段校验、“从 `backend/` 启动时仍回到项目根 `plugins/`”的目录解析逻辑，和启动时确保默认 `api-prompt` 实例存在、默认实例模板为空时补回内置模板的自愈行为 |
| 用户 API Key 插件绑定 | `backend/ent/schema/api_key.go`、`backend/internal/service/api_key_service.go`、`backend/internal/service/plugin_service.go`、`backend/internal/handler/api_key_handler.go` | `api_keys.plugin_settings` 字段结构、校验逻辑；`api-prompt` 绑定要求本地插件和模板均启用，请求期模板不可用时保持原请求体不变 |
| `api-prompt` 本地插件说明 | `docs/api-prompt-插件协议.md`、`plugins/api-prompt/manifest.json`、`plugins/api-prompt/config.json` | 本地插件实例元数据和模板配置；`config.json` 保存可绑定 Prompt 模板，`source` 固定为 `local` |
| 用量日志结构 | `backend/ent/schema/usage_log.go` | token、成本、耗时、账号、分组、模型、请求类型、`proxy_id`、住宅代理流量字段 |
| 用量查询实现 | `backend/internal/repository/usage_log_repo.go`、`backend/internal/repository/dashboard_aggregation_repo.go`、`backend/internal/service/dashboard_recommendation_service.go` | 聚合 SQL、分页性能、统计字段来源；管理端运营统计默认只纳入 `active` 用户，自动禁用用户不进入成本、盈利、排行和容量建议口径 |
| 住宅代理流量计量 | `backend/internal/service/residential_ip_traffic_meter.go`、`backend/internal/service/residential_ip_estimator.go`、`backend/internal/service/gateway_service.go`、`backend/internal/service/gateway_forward_as_chat_completions.go`、`backend/internal/service/gateway_forward_as_responses.go`、`backend/internal/service/openai_gateway_service.go` | `proxy_traffic_*` 字段的来源、streaming 响应字节捕获、校准值默认下限与 Dashboard oversell calculator 的住宅 IP 流量估算口径 |
| 代理巡检历史结构 | `backend/ent/schema/proxy_probe_log.go` | `proxy_probe_logs` 短期历史、巡检来源、出口信息与错误截断字段 |
| 代理巡检写入 | `backend/internal/service/proxy_failover_service.go`、`backend/internal/service/admin_service.go` | 自动巡检与手动测试旁路写入，不改变迁移和探测主流程 |
| 代理可靠性分析 | `backend/internal/repository/proxy_probe_log_repo.go`、`backend/internal/handler/admin/proxy_handler.go` | `/admin/proxies/:id/probe-logs` 与 `/admin/proxies/:id/reliability` 的只读分析口径 |
| 代理列表质量排序 | `backend/internal/repository/proxy_repo.go`、`backend/internal/service/admin_service.go`、`frontend/src/views/admin/ProxiesView.vue`、`frontend/src/components/common/ProxySelector.vue` | `sort_by=quality_score` 走服务层 Redis 质量缓存排序；代理选择器默认按质量分、延迟、ID 展示，并显示 A-F 质量等级 |
| 余额邀请码注册 | `backend/internal/service/auth_service.go`、`backend/internal/service/temporary_invitation.go`、`backend/internal/service/admin_service.go`、`frontend/src/views/admin/RedeemView.vue` | `邀请码（余额）` 使用 `redeem_codes.value` 作为注册赠送余额；普通注册与 OAuth 首次注册会在默认余额基础上叠加非负赠送金额，管理员兑换码页可生成和筛选 `invitation_balance` |
| 临时邀请用户状态 | `backend/internal/service/auth_service.go`、`backend/internal/service/temporary_invitation_service.go`、`backend/internal/service/admin_service.go` | `邀请码（临时）` 注册后的 24h 充值观察、自动禁用/删除与管理员重新启用重置窗口语义 |
| 邀请返利审计 | `backend/internal/handler/admin/affiliate_handler.go`、`backend/internal/service/affiliate_service.go`、`backend/internal/repository/affiliate_repo.go`、`backend/migrations/127_affiliate_ledger_audit_snapshots.sql` | `/admin/affiliates/invites`、`/admin/affiliates/rebates`、`/admin/affiliates/transfers` 与 `/admin/affiliates/users/:user_id/overview` 的只读审计口径；`source_order_id` 关联返利订单，转余额 ledger 记录余额与返利额度快照 |
| 前端管理端 API | `frontend/src/api/admin/usage.ts` | 管理端 usage 接口的参数与返回类型 |
| 前端返利管理 API | `frontend/src/api/admin/affiliate.ts`、`frontend/src/views/admin/AffiliateView.vue` | 管理端返利专属配置、邀请记录、返利入账和转余额记录的前端请求路径与展示字段 |
| 前端插件 API | `frontend/src/api/admin/plugins.ts`、`frontend/src/api/plugins.ts` | 管理端插件管理接口与用户侧 `api-prompt` 模板目录接口的入参与返回值 |
| 前端代理 API | `frontend/src/api/admin/proxies.ts` | 代理巡检历史与可靠性接口类型 |
| 前端 Dashboard API | `frontend/src/api/admin/dashboard.ts` | Dashboard 接口的参数与返回类型 |
