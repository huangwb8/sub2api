# sub2api 源码核对地图

使用本技能时，采集数据只能说明现象；最终建议需要结合源码理解机制。优先核对以下位置：

| 主题 | 文件 | 关注点 |
|------|------|--------|
| 管理员鉴权 | `backend/internal/server/middleware/admin_auth.go` | Admin API Key 使用 `x-api-key`，管理员 JWT 使用 `Authorization: Bearer` |
| 管理端用量列表与统计 | `backend/internal/handler/admin/usage_handler.go` | `start_date/end_date/timezone`、分页、过滤与 `exact_total` 语义 |
| Dashboard 汇总 | `backend/internal/handler/admin/dashboard_handler.go` | 统计、趋势、模型、分组、用户排行、snapshot-v2 的接口口径 |
| 用量日志结构 | `backend/ent/schema/usage_log.go` | token、成本、耗时、账号、分组、模型、请求类型、住宅代理流量字段 |
| 用量查询实现 | `backend/internal/repository/usage_log_repo.go` | 聚合 SQL、分页性能、统计字段来源 |
| 前端管理端 API | `frontend/src/api/admin/usage.ts` | 管理端 usage 接口的参数与返回类型 |
| 前端 Dashboard API | `frontend/src/api/admin/dashboard.ts` | Dashboard 接口的参数与返回类型 |
