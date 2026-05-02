# sub2api 加用户容量分析源码核对地图

使用本技能时，线上数据只能说明现象；最终“适合加多少用户”的建议必须结合源码确认口径。优先核对以下位置：

| 主题 | 文件 | 关注点 |
|------|------|--------|
| 管理员鉴权 | `backend/internal/server/middleware/admin_auth.go` | Admin API Key 使用 `x-api-key`，管理员 JWT 使用 `Authorization: Bearer` |
| 容量推荐接口 | `backend/internal/handler/admin/dashboard_handler.go`、`backend/internal/server/routes/admin.go` | `/api/v1/admin/dashboard/recommendations` 是只读 GET 接口 |
| 容量推荐服务 | `backend/internal/service/dashboard_recommendation_service.go` | 推荐口径使用订阅分组、30d 活跃用户、7d 增长、可调度账号和管理员紧张度设置 |
| 容量池聚合 | `backend/internal/service/dashboard_recommendation_pool.go` | 多分组共享账号会被合并为同一容量池；不能把不同容量池的冗余随意互相抵消 |
| 可调度账号定义 | `backend/internal/service/dashboard_recommendation_service.go` | 账号需 active、schedulable、未过期、未 rate limit、未 overload、未临时不可调度 |
| 分组实时容量 | `backend/internal/handler/admin/group_handler.go`、`backend/internal/service/group_capacity_service.go` | `/api/v1/admin/groups/capacity-summary` 提供 concurrency/sessions/RPM 当前使用量 |
| 用量统计接口 | `backend/internal/handler/admin/usage_handler.go` | `start_date/end_date/timezone` 的日期桶口径 |
| 用量日志结构 | `backend/ent/schema/usage_log.go` | token、成本、耗时、账号、分组、模型、请求类型等字段 |
| Dashboard 前端类型 | `frontend/src/api/admin/dashboard.ts`、`frontend/src/types` | 推荐响应字段兼容新旧结构，尤其是 `pools` 与 legacy `items` |
| 分组前端类型 | `frontend/src/api/admin/groups.ts` | `getCapacitySummary()` 对应分组容量 GET 接口 |

## 换算原则

- `recommended_additional_schedulable_accounts > 0` 表示当前容量池存在可调度账号缺口，本技能将其按该池的 `active_subscriptions_per_schedulable` 换算为负的用户数。
- 当容量池没有账号缺口时，才评估可新增用户；可新增用户 = 扣除安全保留账号后的富余可调度账号数 × 该池每个可调度账号承载的活跃订阅基线。
- 如果某个容量池高利用率或低置信度，必须降低或归零该池的新增用户建议。
- 不同平台、不同容量池之间不能默认互相支援；只要任一容量池存在缺口，总结论应优先呈现缺口，执行时以分池建议为准。
