# Upstream `6c73b621..51af8df3` 选择性吸收优化计划

> **For Codex / Claude:** 本文档仅用于规划，不直接修改业务源码。后续实施必须按主题分批吸收、分批验证，禁止整段 `cherry-pick` 或试图把当前 fork 直接追平上游。

**Goal:** 基于上游 `Wei-Shaw/sub2api` 在 `6c73b6212cee5bb78fb4a70ead7a4ab70ee6102b..51af8df31d12fbce6b91c1dc940b5559f7abcdbc` 之间的演进，筛出对当前个人 fork 最有价值、且能低风险落地的优化项，并形成一份“不改源码、先定路线”的吸收计划。

**Method:** 本轮已按 `awesome-code` 的规划思路完成拆解：先对上游 commit 主题做聚类，再对照当前 fork 的真实实现判断“是否已吸收 / 是否值得吸收 / 是否需要改写后再吸收”。

---

## 范围结论

- 该区间共 **8** 个非 merge 提交，涉及 **30** 个文件，约 **585** 行新增、**186** 行删除。
- 上游变化主要集中在 5 个主题：
  - 支付 provider 配置的可靠性与安全性
  - 订阅倍率计费正确性
  - 原生支付宝支付流程与前端弹窗体验
  - 上游响应体读取上限
  - 管理台表单的浏览器自动填充干扰
- 当前 fork 中，真正值得吸收的是“correctness / stability / secret safety”类补丁，而不是把支付体系整体往上游形态回滚。

## 上游变化摘要

### 主题 1：支付 provider 配置可靠性与脱敏

对应提交：

- `fd0c9a13` `fix(payment): store provider config as plaintext JSON with legacy ciphertext fallback`
- `235f7108` `feat(payment): redact provider secrets in admin config API`
- `61a008f7` `chore(payment): mark legacy AES ciphertext fallback as deprecated`

核心变化：

- 新写入的支付 provider 配置改为明文 JSON 存库，不再强依赖 `TOTP_ENCRYPTION_KEY`
- 读取路径优先尝试 JSON，再回退兼容旧 AES 密文
- 管理端读取 provider 配置时不再把敏感字段原样返回给浏览器
- 编辑时空敏感字段表示“保持原值”，不是“清空”

### 主题 2：订阅倍率计费正确性

对应提交：

- `44cdef79` `fix(usage): subscription billing honours group rate multiplier`

核心变化：

- 订阅模式不再按 `TotalCost` 扣减额度，而是按 `ActualCost`
- 分组倍率、用户专属倍率、免费订阅等语义终于能真实反映到订阅消耗

### 主题 3：倍率语义收口

对应提交：

- `df57d277` `fix(billing): reject rate_multiplier <= 0 on save; clamp negatives to 0 in compute`

核心变化：

- 上游把分组倍率 / 用户专属倍率的保存限制为 `> 0`
- 计算层把负数倍率按 `0` 处理，不再静默按 `1.0` 计费

### 主题 4：支付宝流程与弹窗体验

对应提交：

- `c3cb0280` `fix(payment): alipay redirect-only flow, H5 detection and popup sizing`

核心变化：

- PC 支付宝不再把 `pay_url` 当二维码内容渲染
- 移动端可显式传 `is_mobile`
- 弹窗尺寸改为更适配支付宝收银台的动态计算

### 主题 5：响应体读取与表单防误填

对应提交：

- `bf0bbe0b` `feat(gateway): raise upstream response read limit 8MB -> 128MB (configurable)`
- `948d8e6d` `fix(admin): prevent browser password manager from autofilling account API key`

核心变化：

- 图片/大响应场景默认读取上限从 8MB 提到 128MB
- 管理员编辑 API Key 账号时，阻止浏览器密码管理器误填旧密码

---

## 当前 fork 的差距判断

### 已确认仍未吸收、且值得处理

- `backend/internal/service/payment_config_providers.go`
  - 仍然是 AES-only 写入/读取
  - `ListProviderInstancesWithConfig` 仍会把敏感配置返回到前端
- `backend/internal/payment/load_balancer.go`
  - 读取 provider 配置时没有 JSON-first / AES-fallback 兼容逻辑
- `backend/internal/service/gateway_service.go`
  - 订阅扣费链路仍多处使用 `TotalCost`
- `backend/internal/payment/provider/alipay.go`
  - PC 支付宝仍返回 `QRCode: payURL`
- `frontend/src/views/user/PaymentView.vue`
  - 当前分支优先走 `qr_code` 分支，因此桌面支付宝仍可能展示“不可扫码的伪二维码”
- `backend/internal/service/upstream_response_limit.go`
  - 默认上限仍为 `8MB`
- `backend/internal/config/config.go`
  - `gateway.upstream_response_read_max_bytes` 默认值仍为 `8MB`
- `frontend/src/components/account/EditAccountModal.vue`
  - 还没有浏览器密码管理器忽略属性

### 不建议直接照搬，需要按当前 fork 语义改写

- `df57d277` 不能直接吸收

原因：

- 当前 fork 明确允许 `account.rate_multiplier >= 0`，且 `0` 表示账号免费计费
- `backend/internal/service/group.go` 仍保留了 `IsFreeSubscription()` 语义
- 分组页面前端会把 `<=0` 自动纠正回 `1.0`，但用户专属倍率编辑入口仍允许输入 `0`
- 这意味着当前 fork 的“0 倍率”语义并未完全统一，直接套用上游的 `>0` 规则，有概率误伤现有数据或未来的“免费订阅”设计

结论：

- 这不是“直接 cherry-pick”的题，而是“先定义单一真相，再做语义收口”的题

---

## 选择性吸收建议

## P0：建议优先吸收

### 主题 1：支付 provider 配置可靠性与敏感信息脱敏

**吸收来源：**

- `fd0c9a13`
- `235f7108`
- `61a008f7`

**Why：**

- 这是当前最硬的稳定性 + 安全性缺口。
- 现状下，如果部署环境缺少或变更 `TOTP_ENCRYPTION_KEY`，支付 provider 配置可能出现“保存了但重启后读不回”的问题。
- 同时，管理端 GET provider 配置时把 `privateKey` / `secretKey` / `apiV3Key` 等直接返回到浏览器，风险不必要地高。

**当前受影响文件：**

- `backend/internal/service/payment_config_providers.go`
- `backend/internal/payment/load_balancer.go`
- `backend/internal/payment/crypto.go`
- `frontend/src/components/payment/PaymentProviderDialog.vue`

**建议实施方式：**

- 新写入路径改为明文 JSON 存储
- 读取路径统一为：
  - 先尝试 JSON 解析
  - JSON 失败后，再在有合法 key 时尝试 AES 解密
  - 两者都失败时，按“空配置待重新录入”处理，不要让服务直接因为旧脏数据失效
- 管理端返回 provider 配置时，对敏感字段执行“服务端剔除而非前端遮罩”
- 编辑 provider 时，敏感字段的空值语义改为“保持原值”
- 明确保留 AES fallback，但标记为迁移兼容层，不再作为长期主路径

**验证方式：**

- `cd backend && go test -tags=unit ./internal/service -run 'PaymentConfig|Provider'`
- `cd backend && go test -tags=unit ./internal/payment -run LoadBalancer`
- `cd frontend && pnpm run typecheck`
- 手工验证：
  - 新增 provider 后重启服务，配置仍可用
  - 编辑 provider 的非敏感字段时，无需重新填写 secret
  - 管理端网络响应中不再出现完整私钥/secret 内容

### 主题 2：订阅模式改为按 `ActualCost` 扣减

**吸收来源：**

- `44cdef79`

**Why：**

- 这是实打实的 correctness 缺口。
- 当前 fork 虽然已经做了大量“分组倍率 / 用户专属倍率 / 订阅展示”能力，但订阅扣费链路仍按 `TotalCost` 扣减，等于把倍率规则绕开了。
- 这会直接影响：
  - 分组倍率不生效
  - 用户专属倍率不生效
  - 免费订阅 / 折扣订阅行为不符合后台配置预期

**当前受影响文件：**

- `backend/internal/service/gateway_service.go`

**建议实施方式：**

- 只改订阅扣费链路，不动余额计费链路
- 至少统一这三处：
  - 构建 billing command
  - 最终 subscription usage 落库/缓存更新
  - legacy fallback 路径
- 新增覆盖 `2x / 0.5x / 0x` 的表驱动测试，锁死语义

**验证方式：**

- `cd backend && go test -tags=unit ./internal/service -run 'Subscription|Billing|Gateway'`
- 回归用例至少覆盖：
  - 订阅组倍率 `2.0`
  - 订阅组倍率 `0.5`
  - 订阅实际扣减为 `0`
  - 余额模式不受影响

### 主题 3：原生支付宝改为 redirect-only，去掉伪二维码

**吸收来源：**

- `c3cb0280`

**Why：**

- 当前 `backend/internal/payment/provider/alipay.go` 在 PC 场景把 `pay_url` 同时塞进 `QRCode`
- 当前 `frontend/src/views/user/PaymentView.vue` 又优先处理 `qr_code`
- 两者组合起来，会把“支付宝收银台 URL”当二维码渲染，属于高概率真实可见的用户问题

**当前受影响文件：**

- `backend/internal/payment/provider/alipay.go`
- `backend/internal/handler/payment_handler.go`
- `frontend/src/types/payment.ts`
- `frontend/src/views/user/PaymentView.vue`
- `frontend/src/components/payment/providerConfig.ts`
- `frontend/src/components/payment/PaymentQRDialog.vue`
- `frontend/src/components/payment/PaymentStatusPanel.vue`
- `frontend/src/components/payment/StripePaymentInline.vue`

**建议实施方式：**

- PC 原生支付宝只返回 `pay_url`，不再返回 `qr_code`
- 前端对支付宝一律走 popup / redirect waiting 态，不再自己生成二维码
- 新增 `is_mobile` 可选字段，由前端显式声明移动端状态；后端保留 UA fallback 兼容旧客户端
- 用动态 popup features 替代固定 `1000x750`

**验证方式：**

- `cd frontend && pnpm run typecheck`
- 手工验证：
  - 桌面端原生支付宝弹出收银台，不再展示前端二维码
  - 移动端原生支付宝直接跳转
  - 小屏笔记本上弹窗不会被裁掉主要区域

## P1：建议本轮顺手吸收

### 主题 4：把默认上游响应读取上限提高到 128MB

**吸收来源：**

- `bf0bbe0b`

**Why：**

- 当前 fork 已经在做更多图片/多模态能力，`8MB` 默认值对 base64 图片响应明显偏小
- 当前其实已经有 `ReadUpstreamResponseBody(...)` 抽象，说明架构准备已经具备，这次主要是调默认值和共享常量

**当前受影响文件：**

- `backend/internal/config/config.go`
- `backend/internal/service/upstream_response_limit.go`

**建议实施方式：**

- 引入配置层单一常量
- 默认值提升到 `128 * 1024 * 1024`
- 保留 `gateway.upstream_response_read_max_bytes` 覆盖能力

**验证方式：**

- `cd backend && go test -tags=unit ./internal/service -run UpstreamResponse`
- 补一个较大响应体读取回归测试

### 主题 5：阻止管理员编辑账号时被浏览器密码管理器误填 API Key

**吸收来源：**

- `948d8e6d`

**Why：**

- 这是一个小改动，但非常贴近真实运营场景
- 当前多平台共用 Base URL 的账号编辑表单，确实容易被浏览器误判成登录表单

**当前受影响文件：**

- `frontend/src/components/account/EditAccountModal.vue`

**建议实施方式：**

- 给 API Key 输入框补齐：
  - `autocomplete="new-password"`
  - `data-1p-ignore`
  - `data-lpignore`
  - `data-bwignore`

**验证方式：**

- `cd frontend && pnpm run typecheck`
- 手工验证 Chrome / 1Password / Bitwarden 不再主动填充

## P1：需要单独设计后再吸收

### 主题 6：倍率语义统一收口，不直接照搬上游 `df57d277`

**吸收来源：**

- `df57d277`

**Why：**

- 这个 commit 触及的是“业务语义”，不是普通 bugfix
- 直接吸收虽然简单，但风险在于把当前 fork 的历史数据和隐含语义一起打断

**建议路线：**

- 第一步：先做数据盘点
  - 检查 `groups.rate_multiplier <= 0`
  - 检查 `user_group_rate_multipliers.rate_multiplier <= 0`
  - 检查是否存在依赖 `0` 作为免费逻辑的真实线上数据
- 第二步：明确单一真相
  - `account.rate_multiplier` 保持 `>= 0`，`0` 表示账号免费计费
  - `group.rate_multiplier` 与 `user_group_rate_multipliers.rate_multiplier` 不要立即改规则，先根据真实数据确定
- 第三步：若线上不存在合法 `0` 依赖，再逐步收口为：
  - 写入层禁止负数
  - 计算层把负数按 `0` 或拒绝处理，而不是静默回退 `1.0`
  - 是否禁止 `0`，在数据清洗后再决定
- 第四步：若线上确实存在“免费订阅”需求，不用倍率字段隐式表达，改为显式业务开关或显式免费套餐语义

**不建议本轮直接做的事：**

- 直接把 `CreateGroup / UpdateGroup / SyncUserGroupRates` 全部切成 `> 0`
- 在没有盘点现网数据前移除 `IsFreeSubscription` 相关语义

---

## 不建议本轮处理的内容

- 不做整段 commit 区间追平
- 不顺手重构支付页视觉体系
- 不在本轮就删除 AES fallback
- 不把倍率语义争议和订阅扣费 bug 混在同一个提交里解决

---

## 推荐实施顺序

### 阶段 1：先补 correctness 与 secret safety

- 主题 1：provider 配置可靠性与脱敏
- 主题 2：订阅按 `ActualCost` 扣减
- 主题 3：原生支付宝 redirect-only

### 阶段 2：再补容量与运维细节

- 主题 4：上游响应读取上限
- 主题 5：管理员表单防误填

### 阶段 3：最后做倍率语义收口专项

- 主题 6：先盘点数据，再决定 `0` 的业务语义

---

## 成功标准

- 支付 provider 配置不再因密钥缺失/变更而“重启后失忆”
- 管理端接口不再向浏览器返回完整支付私钥/secret
- 订阅额度消耗与倍率配置一致
- 桌面支付宝不再展示不可扫码的伪二维码
- 大图/多图响应不再轻易触发 `8MB` 上限
- 倍率语义从“隐含且分散”变为“单一真相、可测试、可迁移”

## 备注

- 本计划是对 [docs/plans/2026-04-18-upstream-v2-v1-optimization-plan.md](/Volumes/2T01/Github/sub2api/docs/plans/2026-04-18-upstream-v2-v1-optimization-plan.md) 的增量补充，聚焦 `6c73b621..51af8df3` 这一小段上游新增补丁。
- 真正实施时，建议按“一个主题一个 commit / PR”的粒度推进，避免把 correctness、security、payment UX 混成一次大改。
