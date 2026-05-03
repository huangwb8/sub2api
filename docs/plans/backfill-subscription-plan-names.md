# 订阅套餐名同步优化计划

## Context

管理员已将套餐名从 "GPT-Standard" 改为 "G-Standard" 等，但已有订阅仍显示旧名。
原因：改名未通过 admin API 的 `UpdatePlan`（该接口自带同步逻辑），导致订阅快照未更新。

当前受影响数据：
- 5 个活跃订阅（plan_id=1）仍显示 "GPT-Standard"，应更新为 "G-Standard"
- 2 个订阅（plan_id=NULL）无套餐信息，属于迁移前遗留数据，不做处理

## 方案

仅需一次性数据修复——现有同步机制（`UpdatePlan` 中的 `req.Name != nil` 分支）已覆盖未来改名场景。

### 一次性数据修复

新增迁移文件 `backend/migrations/126_backfill_subscription_plan_names.sql`：

```sql
-- 将所有订阅的 current_plan_name 同步为对应套餐表的最新名称
UPDATE user_subscriptions us
SET current_plan_name = sp.name
FROM subscription_plans sp
WHERE us.current_plan_id = sp.id
  AND us.current_plan_name IS DISTINCT FROM sp.name;
```

使用 `IS DISTINCT FROM` 跳过已一致的行，避免无意义的写入。

## 修改文件

| 文件 | 操作 |
|------|------|
| `backend/migrations/126_backfill_subscription_plan_names.sql` | 新增 |

## 验证

1. 在远程站点通过 admin API 确认迁移前受影响订阅的 `current_plan_name`
2. 执行迁移后重新查询，确认所有订阅的 plan_name 已同步为套餐表最新名称
3. 后续改名通过 admin UI 操作，验证同步自动生效
