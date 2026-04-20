# Upstream `51af8df3..23def40b` 选择性吸收优化计划

> **For Codex / Claude:** 本文档只负责规划，不直接修改业务源码。后续实施应按主题分批吸收、分批验证，禁止把当前 fork 直接整段追平上游。

**Goal:** 基于上游 `Wei-Shaw/sub2api` 在 `51af8df31d12fbce6b91c1dc940b5559f7abcdbc..23def40bc5415c04ca3a05bb6d67a6ff1e4a3566` 之间的演进，筛出对当前个人 fork 最值得吸收的优化项，并把“为什么值得做、怎么低风险做、如何验证”沉淀为可执行计划。

**Method:** 本轮按 `awesome-code` 的协调思路先跑了 `agent_coordinator.py` 做任务拆解；门禁结果 `dispatch_gate.can_proceed = true`，并补读了 `git-workflow` 子技能。随后使用本地 `git diff`、`git show` 与当前 fork 源码对照，判断“已吸收 / 未吸收 / 需要改写吸收 / 只需同步 License”。

## 范围结论

- 该区间共 **7** 个非 merge 提交，涉及 **15** 个文件，约 **395** 行新增、**62** 行删除。
- 变化集中在 4 个主题：
  - Claude Messages `output_config.effort=xhigh` 的兼容与展示
  - API Key / Bedrock 配额超限后的调度、粘性会话清理与后台状态展示
  - 删除账号时同步清理 `scheduled_test_plans` 孤儿记录
  - 项目许可证从 `MIT` 切换到 `LGPL v3.0`
- 对当前 fork 来说，真正值得吸收的是 correctness / stability / data hygiene 类改动；README 里的上游文案同步价值不高，但 License 变更必须跟上。

## 上游变化摘要

### 主题 1：Claude `xhigh` 推理强度兼容

对应提交：

- `6530776a` `fix: support xhigh reasoning effort in usage records for Claude Messages API`

核心变化：

- `backend/internal/service/gateway_request.go` 的 `NormalizeClaudeOutputEffort` 新增接受 `xhigh`
- `backend/internal/service/gateway_request_test.go` 补了 `xhigh` 与大小写归一化测试
- `frontend/src/utils/format.ts` 把 `xhigh` 展示调整为 `XHigh`，并补了 `max -> Max`

### 主题 2：配额超限账号不应继续参与调度

对应提交：

- `258fd145` `fix(account): prevent quota-exceeded API key/Bedrock accounts from being scheduled`

核心变化：

- `Account.IsSchedulable()` 把 API Key / Bedrock 的 `IsQuotaExceeded()` 纳入统一判定
- `shouldClearStickySession()` 不再手写一套零散条件，而是委托 `IsSchedulable()`，额外只保留模型级限流判断
- 新增 `backend/internal/service/account_quota_schedulable_test.go`
- 补强 `backend/internal/service/sticky_session_test.go`
- 后台账号状态徽标增加 `quotaExceeded` 文案与视觉提醒

### 主题 3：删除账号时清理孤儿的定时测试计划

对应提交：

- `6579f28b` `fix: delete scheduled test plans when account is deleted`

核心变化：

- 在 `backend/internal/repository/account_repo.go` 的删除事务里，显式执行 `DELETE FROM scheduled_test_plans WHERE account_id = $1`
- 原因是账号使用软删除，数据库层 `ON DELETE CASCADE` 不会触发

### 主题 4：许可证切换

对应提交：

- `23def40b` `chore: change license from MIT to LGPL v3.0`

核心变化：

- 根目录 `LICENSE` 从 MIT 文本替换为 GNU Lesser General Public License v3.0
- 上游 README 系列文件的 License 描述同步改为 LGPL 口径

## 当前 fork 的差距判断

### 已确认未吸收且值得处理

#### 1. `xhigh` 兼容仍然存在语义断点

当前 fork 现状：

- `backend/internal/service/gateway_request.go` 的 `NormalizeClaudeOutputEffort` 只接受 `low / medium / high / max`
- `backend/internal/service/gateway_request_test.go` 仍把 `xhigh` 视为 `nil`
- `frontend/src/utils/format.ts` 已能展示 `xhigh`，但后端归一化不认，前后端语义不一致

判断：

- 这是低成本、低风险、直接提升兼容性的修补项
- 若当前用户开始传 `xhigh`，本地 fork 可能出现“请求能走、记录不准或 UI 语义漂移”的问题

建议优先级：

- `P2`

#### 2. 配额超限账号的“可调度性单一真相”仍未收口

当前 fork 现状：

- `backend/internal/service/account.go` 已实现 `IsQuotaExceeded()`，但 `IsSchedulable()` 还没有把它纳入
- `backend/internal/service/gateway_service.go` 里额外存在 `isAccountSchedulableForQuota()`，说明项目已经意识到这个风险，但规则分散在多个入口
- `shouldClearStickySession()` 仍只看 `status / schedulable / temp unschedulable / model rate limit`，没有复用 `IsSchedulable()`，也没有覆盖 `OverloadUntil / RateLimitResetAt / quota exceeded`
- `frontend/src/components/account/AccountStatusIndicator.vue` 还不会单独标识 `quota exceeded`

判断：

- 这是当前区间里最值得吸收的改动，因为它修的是调度正确性和状态一致性
- 继续维持“调度入口 A 判断 quota，入口 B 不判断，粘性会话入口 C 又有自己的一套规则”，长期会引入幽灵 sticky、状态误判和排障成本
- 但不能直接照搬上游 patch，因为当前 fork 额外引入了 `AutoPauseOnExpired` 与更多调度条件，应该把“可调度性”统一收口到现有语义，而不是简单复制上游实现

建议优先级：

- `P0`

#### 3. 删除账号后仍可能遗留 `scheduled_test_plans`

当前 fork 现状：

- `backend/internal/repository/account_repo.go` 的 `Delete()` 只删除 `account_groups` 和 `accounts`
- 仓库中已经存在 `backend/internal/repository/scheduled_test_repo.go`，说明这个能力在本 fork 里是活跃功能，不是死代码
- 账号软删除时，纯靠数据库外键级联并不能清掉 `scheduled_test_plans`

判断：

- 这是明确的数据卫生问题
- 风险不一定立刻让主链路报错，但会留下孤儿计划、脏数据和后续定时任务噪音
- 修复成本很低，而且天然适合和删除事务一起做原子收口

建议优先级：

- `P0`

### 已同步处理，不再放入后续业务实现计划

#### 4. License 变更

当前 fork 原状：

- 根目录 `LICENSE` 仍是 MIT 文本

本轮处理：

- 已将根目录 `LICENSE` 同步为 LGPL v3.0，以跟上游 `23def40b`

说明：

- 本地 README 当前没有公开写死旧的 MIT 许可证文本，因此这次无需额外改 README 才能避免误导
- 若后续你准备在 README 新增 License 章节，应直接使用 LGPL 口径，不要回写 MIT

## 选择性吸收建议

## P0：建议优先吸收

### 主题 A：统一“账号可调度性”语义，消除 quota / sticky / UI 三套规则漂移

建议目标：

- 让 `Account.IsSchedulable()` 成为账号级调度资格的单一真相
- `shouldClearStickySession()` 只复用这个真相，并保留模型级限流判断
- 后台状态展示能明确告诉管理员“这个账号是配额超限，而不是普通暂停”

建议实施文件：

- `backend/internal/service/account.go`
- `backend/internal/service/gateway_service.go`
- `backend/internal/service/sticky_session_test.go`
- `backend/internal/service/account_quota_schedulable_test.go`
- `frontend/src/components/account/AccountStatusIndicator.vue`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`

实施要点：

- 仅把 API Key / Bedrock 的 `IsQuotaExceeded()` 纳入 `IsSchedulable()`，不要误伤 OAuth 流程
- 统一覆盖 `OverloadUntil / RateLimitResetAt / TempUnschedulableUntil / quota exceeded`
- 迁移完成后，评估 `isAccountSchedulableForQuota()` 是否还能保留；若只是重复包装，应考虑收口，避免双重语义源
- 前端状态文案不要把 quota exceeded 混成 generic paused，否则管理员仍难以定位问题

验证方式：

- `cd backend && go test -tags=unit ./internal/service -run 'AccountIsSchedulable|Quota|Sticky'`
- `cd frontend && pnpm run typecheck`
- 手工验证：
  - API Key 总额度耗尽后，不再被调度
  - 已绑定 sticky 的 quota-exceeded 账号在下一次请求会被正确释放
  - 后台列表能区分“暂停”和“配额超限”

### 主题 B：删除账号事务里同步清理 `scheduled_test_plans`

建议实施文件：

- `backend/internal/repository/account_repo.go`
- `backend/internal/repository/account_repo_integration_test.go` 或对应删除流程测试

实施要点：

- 在现有删除事务中加入显式 SQL 删除，保持和账号删除原子一致
- 优先沿用现有事务风格，不要为这个修复引入新的 repository 分层复杂度

验证方式：

- `cd backend && go test -tags=integration ./internal/repository -run Delete`
- 手工验证：
  - 创建带计划的账号后执行删除
  - 确认 `scheduled_test_plans` 不再残留对应 `account_id`

## P1：建议吸收，但可排在 P0 之后

当前区间没有明确独立的 P1 项；如果后续实施时发现 `P0` 会触及大量调度调用点，可先把“后端调度语义统一”和“前端状态展示优化”拆成两个 PR，以降低回归面。

## P2：建议顺手补齐

### 主题 C：补上 Claude `xhigh` 输出强度归一化

建议实施文件：

- `backend/internal/service/gateway_request.go`
- `backend/internal/service/gateway_request_test.go`
- `frontend/src/utils/format.ts`

实施要点：

- 接受 `xhigh` 与大小写变体
- 保持对历史 `max` 的兼容展示
- 不要扩大白名单到未知值，继续维持显式枚举

验证方式：

- `cd backend && go test -tags=unit ./internal/service -run 'ParseGatewayRequest_OutputEffort|NormalizeClaudeOutputEffort'`
- `cd frontend && pnpm run typecheck`

## 不建议直接照搬的地方

- 上游这次配额修复不是纯“抄 patch”题，因为当前 fork 已经有 `isAccountSchedulableForQuota()`、`AutoPauseOnExpired` 和更多运营侧状态语义
- 正确做法是“借鉴上游方向，收口当前 fork 自己的单一真相”，而不是把多个条件再平铺复制一遍
- 上游 README 的 License 文案改动不需要机械移植；本 fork 的 README 结构已分化，当前只需保证根目录 `LICENSE` 与未来新增的 License 说明不再保留 MIT 口径

## 推荐实施顺序

1. 先做 `P0 / 主题 B`：删除账号时清理 `scheduled_test_plans`
2. 再做 `P0 / 主题 A`：统一 quota-aware schedulability、sticky session 与后台状态语义
3. 最后补 `P2 / 主题 C`：Claude `xhigh` 兼容

这样排序的原因：

- 主题 B 改动面最小，收益明确，适合先拿到一个低风险正确性修复
- 主题 A 风险最高，应在已有事务型小修复之后集中验证
- 主题 C 成本最低，适合作为收尾兼容性补丁

## 验收标准

- 删除账号后不会遗留孤儿的 `scheduled_test_plans`
- API Key / Bedrock 的 quota exceeded 状态在调度、sticky 清理与后台展示上保持一致
- Claude `xhigh` 输出强度可以被后端识别并在前端稳定展示
- 本 fork 保持现有功能不回退，不引入额外支付、路由或计费链路变化
