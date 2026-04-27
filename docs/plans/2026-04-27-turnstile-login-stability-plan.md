# Turnstile 登录稳定性改良 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 将登录页 Cloudflare Turnstile 偶发 `turnstile verification failed` 问题从“无法解释的波动”改造成可观测、可自愈、可回滚的稳定登录链路。

**Architecture:** 先补齐服务端 Turnstile 校验结果与真实访问 IP 的观测，再收敛反向代理真实 IP 信任链，最后在前端管理 token 生命周期并给用户明确恢复路径。后端保持安全兜底，不绕过 Turnstile，不记录密钥或完整 token。

**Tech Stack:** Go / Gin / Ent / Redis / Vue 3 / TypeScript / Pinia / Vitest / Cloudflare Turnstile / Docker 反向代理。

**Minimal Change Scope:** 允许修改 `backend/internal/service/turnstile_service.go`、`backend/internal/repository/turnstile_service.go`、`backend/internal/pkg/ip/ip.go`、`backend/internal/handler/auth_handler.go`、`backend/internal/config/config.go`、`deploy/config.example.yaml`、`frontend/src/components/TurnstileWidget.vue`、`frontend/src/views/auth/LoginView.vue` 及对应测试。避免改动认证模型、用户密码登录逻辑、2FA 流程、后台代理池、支付/网关调度逻辑。

**Success Criteria:** 登录 Turnstile 失败时能在日志中看到脱敏后的失败原因、Cloudflare error code、hostname、challenge timestamp、是否传递 remoteip；用户页面停留过久后不会提交过期 token；生产环境不再把 Docker 内网 IP 作为用户真实 IP 传给 Turnstile；所有变更有单元测试覆盖，并能通过前后端最小测试集。

**Verification Plan:** 后端运行 `cd backend && go test -tags=unit ./internal/service ./internal/repository ./internal/pkg/ip ./internal/handler`；前端运行 `cd frontend && pnpm test:run` 和 `cd frontend && pnpm run typecheck`；部署后用 `remote.env` 只读查询 `/api/v1/admin/ops/system-logs?component=service.turnstile&time_range=2h`，确认失败日志具备诊断字段且不含 secret/token。

---

## 背景

远程站点在 `2026-04-27 10:23:40` 到 `10:30:34` 出现连续 11 次 `POST /api/v1/auth/login` 返回 400；对应服务日志均为：

```text
[Turnstile] Verification failed, error codes: [invalid-input-response]
```

随后 `2026-04-27 10:40:00` 同一路径恢复 200，`10:40:19` 的 `/api/v1/auth/login/2fa` 也成功。这说明请求已经到达 Sub2API，失败点不是 Cloudflare 代理拦截页面访问，而是 Sub2API 调用 Cloudflare `siteverify` 后得到 token 无效结果。

目前还发现一个部署隐患：访问日志里的 `client_ip` 为 Docker 内网地址 `172.18.0.2`。这不是后台“IP 管理/代理池”的代理，而是浏览器请求经 Cloudflare 与反向代理进入容器时，应用没有稳定解析真实用户 IP。Turnstile 的 `remoteip` 是可选参数，但错误传递私网 IP 会降低排查质量，并可能放大边缘风控波动。

## 目标

- 保持 Turnstile 登录保护，不引入“失败即放行”的安全后门。
- 将 `invalid-input-response` 细分为可判断的原因：过期、重复使用、域名不匹配、客户端异常、remoteip 异常或 Cloudflare 瞬时失败。
- 登录页对过期 token 自动恢复，减少用户在同一页面停留过久后提交失败。
- 生产部署正确保留真实访问 IP，同时避免把私网 IP 传给 Cloudflare。
- 为管理员提供稳定的排查 runbook，而不是依赖人工猜测。

## 非目标

- 不修改用户密码认证、JWT、Refresh Token、TOTP 2FA 的核心语义。
- 不把 Turnstile 失败自动降级为通过。
- 不把后台账号代理池或上游 API 代理机制接入登录页。
- 不记录 Turnstile secret、完整 response token、用户密码或其它敏感字段。

## 根因假设

| 假设 | 当前证据 | 验证方式 | 优先级 |
|------|----------|----------|--------|
| token 过期或重复使用 | Cloudflare 返回 `invalid-input-response`，且 10 分钟后恢复 | 记录 `challenge_ts`、提交时间差、前端 token 生成时间 | 高 |
| 前端页面部署切换导致旧页面/新资源混用 | 同一天日志出现多个 `TurnstileWidget-*.js` hash | 保留旧 assets，检查 chunk load 日志 | 中 |
| 真实 IP 解析异常放大风控波动 | 后端看到 `172.18.0.2` | 修正 trusted proxies 后对比 remoteip | 中 |
| site key / secret 静态不匹配 | 10:40 已成功登录，不像持续配置错误 | 记录 Cloudflare 返回 hostname 与 action | 低 |
| Cloudflare 服务短时波动 | 失败集中在短时间窗口 | 结合 Cloudflare dashboard 与服务端请求耗时 | 低到中 |

## 方案总览

执行顺序遵循“观测优先、行为收敛、体验兜底、运维固化”：

1. 增强后端 Turnstile 观测，不改变放行策略。
2. 修正真实 IP 信任链，并对私网 remoteip 做服务端兜底。
3. 增强前端 token 生命周期管理，避免明显过期 token 被提交。
4. 加入聚合指标与 runbook，形成生产排查闭环。
5. 灰度部署并保留回滚路径。

## 任务：增强 Turnstile 服务端观测

**Files:**

- Modify: `backend/internal/service/turnstile_service.go`
- Modify: `backend/internal/repository/turnstile_service.go`
- Test: `backend/internal/repository/turnstile_service_test.go`
- Test: 新增或扩展 `backend/internal/service/turnstile_service_test.go`

**实施步骤：**

1. 扩展 `TurnstileVerifyResponse` 解析字段，保留现有 `success`、`error-codes`、`hostname`、`challenge_ts`、`action`、`cdata`。
2. 在 `turnstileVerifier.VerifyToken` 中记录 Cloudflare HTTP 状态码、响应 decode 失败、请求耗时，但不记录 secret 与 response token。
3. 在 `TurnstileService.VerifyToken` 失败日志中追加结构化字段：
   - `error_codes`
   - `hostname`
   - `challenge_ts`
   - `action`
   - `remote_ip_redacted`
   - `remote_ip_private`
4. 对 `invalid-input-response` 保持现有 400 行为，只改善日志和诊断信息。
5. 增加测试覆盖：
   - Cloudflare 返回 `success=false,error-codes=["invalid-input-response"]` 时错误仍为 `ErrTurnstileVerificationFailed`
   - 日志辅助函数不会输出 token/secret
   - 非 2xx 但 JSON 可解析时仍能暴露 Cloudflare error codes

**验证命令：**

```bash
cd backend
go test -tags=unit ./internal/service ./internal/repository
```

**预期结果：** 测试通过；失败日志能解释“Cloudflare 为什么拒绝 token”，但不泄漏敏感信息。

## 任务：修正真实 IP 与 remoteip 兜底

**Files:**

- Modify: `backend/internal/pkg/ip/ip.go`
- Modify: `backend/internal/pkg/ip/ip_test.go`
- Modify: `backend/internal/server/http.go`
- Modify: `backend/internal/config/config.go`
- Modify: `deploy/config.example.yaml`
- Optionally Modify: `docs/DEPLOYMENT.md` 或现有部署文档

**实施步骤：**

1. 明确 `server.trusted_proxies` 的生产配置示例：
   - Docker bridge 网段或宿主机反代 IP
   - 内部 Nginx/Caddy/Traefik IP
   - 不建议直接信任 `0.0.0.0/0`
2. 在 `ip` 包新增公开函数：
   - `IsPrivateOrLoopbackIP(ip string) bool`
   - `SanitizeTurnstileRemoteIP(ip string) string`
3. `SanitizeTurnstileRemoteIP` 规则：
   - 空值返回空
   - 私网、回环、链路本地、非法 IP 返回空
   - 公网 IPv4/IPv6 返回规范化 IP
4. 登录、注册、发送验证码、忘记密码等 Turnstile 调用点使用 sanitization 后的 remoteip。
5. 保持 API Key 风控类逻辑继续使用可信代理链，不和 Turnstile remoteip 兜底混用。
6. 增加测试覆盖：
   - `172.18.0.2` 被置空，不传给 Cloudflare
   - `CF-Connecting-IP: 1.2.3.4` 在可信代理配置正确时保留
   - 非法 IP 不传递

**验证命令：**

```bash
cd backend
go test -tags=unit ./internal/pkg/ip ./internal/handler
```

**预期结果：** 后端不再把 Docker 内网 IP 当用户 IP 传给 Turnstile；部署文档能指导管理员正确设置真实 IP 链路。

## 任务：前端 token 生命周期自恢复

**Files:**

- Modify: `frontend/src/components/TurnstileWidget.vue`
- Modify: `frontend/src/views/auth/LoginView.vue`
- Test: 新增 `frontend/src/components/__tests__/TurnstileWidget.spec.ts`
- Test: 扩展 `frontend/src/components/__tests__/LoginForm.spec.ts` 或新增登录页测试

**实施步骤：**

1. `TurnstileWidget` 在 `callback` 时向父组件传递 token，并可选暴露 `reset()` 与 `getWidgetState()`。
2. `LoginView` 保存 `turnstileVerifiedAt`。
3. 提交登录前检查 token 年龄：
   - 小于 240 秒：正常提交
   - 大于等于 240 秒：调用 `reset()`，清空 token，提示重新验证
4. `expired-callback` 到达时清空 token 并提示验证已过期。
5. `error-callback` 到达时清空 token 并提示刷新或重新验证。
6. 登录失败且错误 reason 为 `TURNSTILE_VERIFICATION_FAILED` 时使用更明确文案：
   - “验证已失效，请重新完成验证后登录”
7. 不在前端复用已经提交过的 token；每次失败后都 reset。

**验证命令：**

```bash
cd frontend
pnpm test:run
pnpm run typecheck
```

**预期结果：** 页面停留过久后不会提交旧 token；用户看到的是可执行恢复提示，而不是模糊的登录失败。

## 任务：部署资产与 CSP 稳定性检查

**Files:**

- Modify: `deploy/` 下实际反代或部署模板（按当前部署入口选择）
- Optionally Modify: `docs/DEPLOYMENT.md`
- Optionally Modify: `docs/plans/2026-04-27-turnstile-login-stability-plan.md` 的执行记录

**实施步骤：**

1. 确认响应头允许 Turnstile：
   - `script-src` 包含 `https://challenges.cloudflare.com`
   - `frame-src` 包含 `https://challenges.cloudflare.com`
   - `connect-src` 不阻断站点 API
2. 部署时保留旧前端 assets 至少一个发布周期，避免旧页面引用的 chunk 404。
3. 为 chunk load error 保留现有自动刷新机制，并在必要时记录一次前端 telemetry。
4. 检查 Cloudflare Turnstile 控制台：
   - Site key 与 Secret key 属于同一个 widget
   - Hostname 包含 `api.benszresearch.com`
   - Widget mode 与站点使用方式一致

**验证命令：**

```bash
curl -I https://api.benszresearch.com/login
curl -I https://api.benszresearch.com/assets/index-B2d_KcYV.js
```

**预期结果：** CSP 不阻断 Turnstile；发布切换不会让登录页引用的异步组件资产短时间失效。

## 任务：运维排查 runbook

**Files:**

- Create: `docs/TURNSTILE_TROUBLESHOOTING.md`
- Modify: `docs/DEPLOYMENT.md` 或 README 相关部署章节（如已有）

**实施步骤：**

1. 写明 Turnstile 登录失败的快速判断：
   - `/login` 是否 200
   - `/api/v1/settings/public` 是否返回 `turnstile_enabled` 与 site key
   - `/api/v1/admin/settings` 是否显示 secret 已配置
   - `/api/v1/admin/ops/system-logs?component=service.turnstile` 的 error code
2. 给出 error code 对照：
   - `invalid-input-response`
   - `invalid-input-secret`
   - `timeout-or-duplicate`
   - domain not authorized 类客户端错误
3. 给出 `remote.env` 只读排查命令模板，但不写入真实密钥。
4. 给出生产反代真实 IP 示例配置和安全注意事项。
5. 写明临时处置优先级：
   - 优先刷新页面重新验证
   - 检查 Cloudflare widget 域名/key/secret
   - 检查真实 IP 信任链
   - 最后才考虑临时关闭 Turnstile，且需要管理员明确记录风险窗口

**验证命令：**

```bash
rg -n "TURNSTILE|Turnstile|trusted_proxies|CF-Connecting-IP" docs README.md deploy
```

**预期结果：** 下一次出现波动时，管理员能在 5 分钟内定位到客户端 token、Cloudflare 配置、反代 IP 或服务端请求哪一层。

## 回滚策略

- 后端日志增强可直接保留，风险低。
- remoteip sanitization 如出现误判，可临时改为传空 remoteip；Cloudflare 支持不传该参数。
- 前端 token 过期阈值可通过常量回退到 300 秒或关闭提交前检查，但不建议复用失败 token。
- `trusted_proxies` 配置错误时，先回滚配置而不是代码；不要用 `0.0.0.0/0` 作为长期方案。
- 如 Turnstile 大面积故障，临时关闭必须记录开始/结束时间、操作者、影响范围，并在恢复后重新启用。

## 风险与防护

| 风险 | 防护 |
|------|------|
| 记录过多敏感信息 | 只记录 error code、hostname、时间、脱敏 IP；禁止 token/secret/password |
| 真实 IP 信任链被伪造 | 只信任明确反代 IP/CIDR；不信任公网任意来源的 X-Forwarded-For |
| 前端 token 过期阈值过短影响体验 | 先用 240 秒，低于 Cloudflare 5 分钟有效期并留出网络余量 |
| 临时关闭 Turnstile 造成撞库风险 | 不做自动降级；仅管理员手动、限时、可审计 |
| 发布过程中旧 chunk 失效 | 保留旧 assets，chunk load error 自动刷新 |

## 验收清单

- [x] Turnstile 失败日志包含 `error_codes`、`hostname`、`challenge_ts`、`remote_ip_private`。
- [x] 日志中不包含 Turnstile secret、完整 response token、密码。
- [x] `172.18.0.2` 等 Docker 内网 IP 不再传给 Cloudflare `siteverify`。
- [x] 登录页 token 超过阈值时会要求重新验证。
- [x] 登录失败后 Turnstile widget 会 reset，下一次提交使用新 token。
- [x] 后端单元测试通过。
- [x] 前端类型检查与 Vitest 通过。
- [ ] 远程只读日志查询可在 2 小时窗口内解释成功/失败登录。

## 建议执行顺序

1. 先做服务端日志增强，不改变用户行为。
2. 再做 remoteip sanitization 和 trusted proxy 文档。
3. 然后做前端 token 年龄检查和提示优化。
4. 最后补 runbook 与部署资产保留策略。

这样即使第一阶段上线后问题再次出现，也能拿到足够证据；后续阶段再逐步减少波动本身。
