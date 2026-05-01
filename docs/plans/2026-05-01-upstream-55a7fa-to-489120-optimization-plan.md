# 上游 `55a7fa1e` 到 `48912014` 变更吸收计划

**目标**：分析 `Wei-Shaw/sub2api` 在 `(55a7fa1e07443212681b7ac4b0df56237d7558d5, 48912014a16e2dd1cfca8b7cad785d0e8e7bfeec]` 区间的真实变化，判断这些变化对当前个人 `sub2api` 项目的启发、必要吸收项与暂缓项。本计划只沉淀后续优化方案，不修改当前业务源码。

**方法**：本轮按 `awesome-code` 流程运行 `agent_coordinator.py`，结果为 `coordination_scope.level=focused-agent`、`dispatch_gate.can_proceed=true`。随后执行 `git fetch upstream`、`git log`、`git diff --stat`、`git diff --name-status`、license 对比和当前仓库代码探针，按“必须吸收 / 条件吸收 / 暂不吸收”给出计划。

**上游比较链接**：`https://github.com/Wei-Shaw/sub2api/compare/55a7fa1e07443212681b7ac4b0df56237d7558d5...48912014a16e2dd1cfca8b7cad785d0e8e7bfeec`

**关键结论**：该区间共有 12 个 commit，其中包含 2 个 merge commit、2 个纯版本同步 commit，实际有效变化主要集中在 2026-04-26 到 2026-04-30 之间；共影响 33 个文件，约 `1019 insertions / 177 deletions`。真正对当前 fork 有价值的重点不在版本号同步，而在两个“正确性补丁”：

- 网关读取压缩请求体的兼容性补丁。
- 调度快照并发安全与粘性会话命中正确性补丁。

**分叉判断**：当前个人仓库 `HEAD=e15d20a1` 并不包含 `48912014`，且已经在认证、支付、风控、代理和运营面板上深度分叉。**不建议 merge / rebase / 整段 cherry-pick 上游区间**，只能按主题做选择性移植。

## License 结论

- 上游该区间没有修改 `LICENSE`；`git diff 55a7fa1e..48912014 -- LICENSE` 为空。
- 因此本轮**不需要**按“上游 license 变化”同步本地 license。
- 额外说明：当前工作树里的 `LICENSE` 与上游 `48912014:LICENSE` 仅存在文件结尾换行差异，正文一致；这不是本次区间新增变化，不作为本轮处理目标。

## 上游区间变化摘要

### 1. 网关请求体解压兼容

相关 commit：

- `798fd673` `feat(httputil): decode compressed request bodies (zstd/gzip/deflate)`
- `40feb86b` `fix(httputil): add decompression bomb guard and fix errcheck lint`

上游把 `ReadRequestBodyWithPrealloc` 从“只读原始 body”升级为“按 `Content-Encoding` 自动解码”，支持：

- `zstd`
- `gzip` / `x-gzip`
- `deflate`

并补了 64 MiB 解压上限，避免 decompression bomb。

### 2. 调度快照并发安全

相关 commit：

- `8bf2a7b8` `fix(scheduler): resolve SetSnapshot race conditions and remove usage throttle`

上游对调度快照做了三件关键事：

- `SetSnapshot` 改为“先写新版本快照，再用 Lua CAS 原子切换 active 指针”，避免并发写入时版本回滚。
- 旧快照不再立刻 `DEL`，而是给 60 秒宽限期，避免读侧在切换瞬间读到空 `ZRANGE`。
- 新增 `UnlockBucket`，重建完成后立即释放分桶锁，不再被动等待 30 秒 TTL。

### 3. 粘性会话调度命中修复

相关 commit：

- `733627cf` `fix: improve sticky session scheduling`

上游这次不仅补了调试日志，更重要的是修正了快照元数据与粘性选择逻辑：

- 调度快照里补写 `AccountGroups` 和 `GroupIDs`。
- 解决“快照反序列化后分组信息丢失，`isAccountInGroup` 永远返回 false，导致粘性账号明明可用却命不中”的问题。
- 调整粘性绑定刷新时机，避免粘性账号暂时不可用时被过早覆盖。

### 4. OpenAI Responses / WS 续链修复

相关 commit：

- `094e1171` `fix(openai): infer previous response for item references`

上游把“客户端只传 `item_reference`、未显式传 `previous_response_id`”的 WS ingress 场景从“拒绝自动推断”改为“允许推断”，防止 tool continuation 在 `store=false` 路径掉链。

### 5. Anthropic 缓存 TTL 1h 注入开关

相关 commit：

- `73b87299` `feat: 添加 Anthropic 缓存 TTL 注入开关`

上游新增一个全局系统设置，用于对 Anthropic OAuth / Setup Token 请求体里**已有的** `cache_control: {type: "ephemeral"}` 自动补写 `ttl: "1h"`，同时在 usage 归因时仍能按 5m/1h 口径做校正。

### 6. 管理端分页大小本地持久化恢复

相关 commit：

- `f084d30d` `fix: 恢复表格分页大小 localStorage 持久化`

上游恢复了页大小的本地记忆能力，系统默认值仅作为 fallback。

### 7. 其它变化

- `9d801595`：管理员设置契约测试同步。
- `9c448f89` / `f972a2fa`：merge commit，无需单独吸收。
- `8ad099ba` / `48912014`：版本号同步，无需跟随当前 fork 的发布节奏。
- 上游同时删除了前端 `usageLoadQueue` 的串行限流；当前 fork 已无该文件，这一项对本仓库已是 **N/A**。

## 当前个人仓库现状判断

### 已明确缺失，且会影响正确性

#### A. 压缩请求体解码仍然缺失

当前 `backend/internal/pkg/httputil/body.go` 的 `ReadRequestBodyWithPrealloc` 只会直接 `io.Copy` 原始 `req.Body`，不会看 `Content-Encoding`。而它已被多个核心入口复用，包括：

- `backend/internal/handler/gateway_handler.go`
- `backend/internal/handler/openai_gateway_handler.go`
- `backend/internal/handler/gateway_handler_responses.go`
- `backend/internal/handler/openai_chat_completions.go`

这意味着如果客户端像上游 commit 描述那样发送 `Content-Encoding: zstd`，当前 fork 很可能会把压缩后的二进制直接当 JSON 解析，导致请求失败。这个问题属于**兼容性与正确性缺陷**，不是可选优化。

#### B. 调度快照仍然保留上游已修复的竞态窗口

当前代码探针显示：

- `backend/internal/repository/scheduler_cache.go` 里的 `SetSnapshot` 仍是“读旧版本 -> `INCR` -> 写新快照 -> 直接 `SET active` -> 立即 `DEL old snapshot`”。
- `backend/internal/service/scheduler_cache.go` 的接口还没有 `UnlockBucket`。
- `backend/internal/service/scheduler_snapshot_service.go` 的 `rebuildBucket` 在成功重建后不会主动解锁，只能等 30 秒锁 TTL 自然过期。

这几处都说明当前 fork 仍保留上游 `8bf2a7b8` 想解决的竞态风险。

#### C. 粘性会话命中仍可能被快照分组信息缺失拖垮

当前 `backend/internal/repository/scheduler_cache.go:374` 的 `buildSchedulerMetadataAccount(...)` 没有序列化 `AccountGroups` / `GroupIDs`；但 `backend/internal/service/gateway_service.go` 的多处粘性路径仍在对缓存恢复出来的账号执行 `isAccountInGroup(...)` 校验。

这与上游 `733627cf` 的修复动机完全一致，说明当前 fork 仍有同类风险：

- 快照账号可能已经在正确分组里。
- 但因为缓存快照缺少分组元数据，粘性重用时会被错误判为“不在组内”。
- 结果就是命中率下降，甚至出现“同一会话反复回退到负载均衡”的行为。

### 已有同类能力，但吸收方式需要按本地业务改造

#### D. 已有账号级 Cache TTL Override，但没有全局注入开关

当前 fork 已经具备：

- 账号 DTO 暴露 `cache_ttl_override_enabled` / `cache_ttl_override_target`
- `GatewayService` 内的 usage TTL 改写逻辑
- 前后端对 `cache_creation_5m_tokens` / `cache_creation_1h_tokens` 的展示链路

说明本地在“TTL 归因与计费”上已经走得比上游更深。但系统设置层面仍没有上游的“Anthropic 1h 请求体注入”总开关，所以这项如果要吸收，必须按当前 fork 的计费语义来接线，而不是原样复制。

#### E. 分页大小当前是有意取消本地记忆

当前 `frontend/src/composables/usePersistedPageSize.ts` 明确写着“不再使用本地持久化缓存，所有页面统一以通用表格设置为准”；`frontend/src/stores/__tests__/app.spec.ts` 也显式断言 `localStorage.getItem('table-page-size') === null`。

因此上游恢复本地记忆不是“漏补丁”，而是与当前 fork 的既有产品决策存在冲突。如果后续要吸收，应该做成“系统默认值 + 用户本地覆盖”的混合策略，而不是直接回滚到上游旧行为。

### 已被当前 fork 覆盖或可视为不必再跟

#### F. OpenAI WS `previous_response_id` 这次不用单独吸收

当前 `backend/internal/service/openai_ws_forwarder.go:1366` 的 `shouldInferIngressFunctionCallOutputPreviousResponseID(...)` 已经采用更简化、更宽松的推断条件：

- `storeDisabled`
- `turn > 1`
- 存在 function_call_output
- 当前 payload 没有显式 `previous_response_id`
- 但上下文里有 `expectedPreviousResponseID`

从行为意图看，它已经覆盖了上游 `094e1171` 想修的主问题。因此这一项本轮不作为独立实施主题，只建议后续在相关测试里顺带补一个 `item_reference` 回归样例。

#### G. usageLoadQueue 删除已不适用

当前仓库已经没有 `frontend/src/utils/usageLoadQueue.ts`，也没有 `enqueueUsageRequest(...)` 的使用点。因此上游“移除串行 usage 队列”在本地是已自然完成的状态，不需要计划。

## 是否有必要吸收

### P0：必须优先吸收

#### 1. 压缩请求体兼容与解压安全上限

必要性：

- 会直接影响 OpenAI / Responses / Claude Code / Codex 等客户端兼容性。
- 一旦客户端默认改发 `zstd`，当前 fork 会出现真实请求失败。
- 这类问题属于核心网关正确性，优先级高于 UI、运营和管理便捷性。

吸收原则：

- 不机械照搬上游文件结构，但必须复用同样的行为语义。
- 支持 `zstd`、`gzip`、`x-gzip`、`deflate`。
- 处理完后要清除 `Content-Encoding` 并重置 `Content-Length`。
- 必须加解压上限，避免把兼容补丁做成安全回归。

#### 2. 调度快照 CAS 切换、旧快照宽限期和主动解锁

必要性：

- 这是调度正确性问题，会影响账号选择、粘性会话、重建延迟和高并发下的快照一致性。
- 当前 fork 的调度链路已经比上游更复杂，越复杂越不能容忍“偶发回滚 / 读空快照 / 锁白等 30 秒”这类隐性竞态。

吸收原则：

- `SetSnapshot` 改为“两阶段写入 + Lua CAS 激活”。
- 旧快照改为 `EXPIRE` 宽限期，而不是立即 `DEL`。
- 在接口层增加 `UnlockBucket`，重建成功或失败后都尽量及时释放锁。
- 回归测试必须覆盖并发写、空快照切换、锁释放和旧读者读期间的稳定性。

#### 3. 粘性会话所需分组元数据补齐

必要性：

- 当前代码同时满足“快照里缺分组元数据”和“选择时依赖分组判断”两个条件，具备真实 bug 形态。
- 该问题会隐蔽地拖低粘性命中率，进一步放大上游限流、模型冷启动和缓存未命中的成本。

吸收原则：

- 调度快照元数据里补写 `AccountGroups` / `GroupIDs`。
- 审查所有粘性路径，避免再对“已从分组筛出来的缓存账号”重复做会出错的分组校验。
- 用单元测试验证“缓存恢复账号仍能命中原分组粘性”。

### P1：有明显启发，但建议按运营需要条件吸收

#### 4. Anthropic `cache_control.ttl=1h` 全局注入开关

适用条件：

- 你实际运营 Anthropic OAuth / Setup Token 账号池。
- 希望用**系统级开关**统一改变请求体 TTL 策略，而不是逐账号配置。
- 能接受这是一项“产品与计费策略联动”的能力，不只是技术补丁。

为什么值得考虑：

- 当前 fork 已有 TTL 统计、展示和账号级 override 基础，落地成本比完全从零接入低。
- 这类能力更适合通过系统设置做灰度开关，便于你对不同站点策略做 AB 验证。

为什么不列入 P0：

- 这不是 correctness bug。
- 它会影响产品策略、成本分布和运营口径，必须由你自己的商业模式来决定是否启用。

### P2：可以吸收，但应先重做产品语义

#### 5. 分页大小本地持久化恢复

为什么有启发：

- 对管理后台高频使用者来说，页大小偏好记忆确实能减少重复操作。
- 上游恢复 `localStorage`，说明“完全统一到系统默认值”在真实使用中可能带来摩擦。

为什么不能直接照搬：

- 当前 fork 已明确把“系统设置下发默认值”作为产品规则。
- 直接回滚会与现有测试、文案和配置心智冲突。

建议方向：

- 保留系统设置里的默认页大小和选项集。
- 新增“本地覆盖仅影响当前浏览器”的轻量语义。
- 支持“重置为系统默认值”，避免管理员误以为全站设置失效。

## 暂不建议吸收

- 上游 `VERSION` 同步 commit：当前 fork 有自己的版本节奏，不跟随 `0.1.120 -> 0.1.121`。
- 纯测试契约同步 commit：除非实施对应功能，否则不单独搬测试文件。
- OpenAI WS `previous_response_id` 修复：当前本地逻辑已覆盖主要场景，本轮不单列。
- usageLoadQueue 删除：本地已无该实现。

## 建议实施顺序

### Task A：压缩请求体兼容补丁

范围：

- `backend/internal/pkg/httputil/body.go`
- 新增该包专属单元测试
- 必要时补 handler 层回归测试

完成标准：

- 压缩与非压缩请求都能被统一解析。
- `zstd` / `gzip` / `deflate` 行为一致。
- 超大解压结果会被安全拒绝。

建议验证：

- `cd backend && go test -tags=unit ./internal/pkg/httputil ./internal/handler/...`

### Task B：调度快照并发安全与粘性修复

范围：

- `backend/internal/repository/scheduler_cache.go`
- `backend/internal/service/scheduler_cache.go`
- `backend/internal/service/scheduler_snapshot_service.go`
- `backend/internal/service/gateway_service.go`
- 对应 unit / integration tests

完成标准：

- 并发 `SetSnapshot` 不会回滚 active 版本。
- 快照切换期间 reader 不会稳定读到空结果。
- 重建完成后锁及时释放，不再无意义等待 TTL。
- 粘性会话在缓存账号路径下仍能命中正确分组账号。

建议验证：

- `cd backend && go test -tags=unit ./internal/repository ./internal/service/...`
- `cd backend && go test -tags=integration ./internal/repository/...`

### Task C：Anthropic 全局 TTL 注入开关

范围：

- 系统设置 key / DTO / handler / service
- `GatewayService` 请求体改写与 usage 口径联动
- 管理端设置页开关与文案

完成标准：

- 默认关闭，不改变现有行为。
- 开启后仅改写“已有 ephemeral cache_control”的 TTL，不新增断点。
- usage 展示与计费口径保持自洽。

建议验证：

- `cd backend && go test -tags=unit ./internal/service/...`
- `cd frontend && pnpm run typecheck && pnpm test`

### Task D：分页大小偏好混合策略

范围：

- `frontend/src/composables/usePersistedPageSize.ts`
- `frontend/src/components/common/Pagination.vue`
- 受影响页面与测试

完成标准：

- 系统默认值仍有效。
- 用户本地覆盖仅影响本浏览器。
- 可重置回系统默认值。

建议验证：

- `cd frontend && pnpm run typecheck && pnpm test`
- 如涉及 UI 文案或交互改动，按项目规范输出 `tmp/screenshots/run-{timestamp}/before.png`、`after.png`、`compare.png`

## 风险与实施约束

- 不要整段 cherry-pick 上游调度相关代码；当前 fork 在认证、代理、风控、计费、OpenAI WS 等链路上已有大量自定义。
- 调度修复优先保证“正确性 > 性能 > 日志丰富度”；上游的 debug 日志不是本次主要迁移目标。
- `cache ttl 1h` 开关必须默认关闭，并明确区分“请求体 TTL 注入”和“usage / billing TTL 归因”两个层面。
- 分页大小偏好如果要做，必须先统一产品语义，否则容易出现“管理员改了系统默认值但前端看不见效果”的误解。

## 本轮结论

最值得立刻吸收的是两类 **P0 正确性补丁**：

1. 压缩请求体兼容。
2. 调度快照并发安全 + 粘性会话分组元数据修复。

其余变化里，`Anthropic 1h TTL` 更像可灰度的运营策略能力，`分页大小本地记忆` 更像交互取舍，不适合在没有产品语义重设计的前提下直接搬运。
