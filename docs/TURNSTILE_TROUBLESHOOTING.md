# Cloudflare Turnstile 登录排查 Runbook

本文用于排查登录、注册、发送验证码和忘记密码中的 Turnstile 校验失败。原则是先定位失败层级，再决定处置；不要把 Turnstile 失败自动放行。

## 快速判断

1. 确认登录页可访问：

```bash
curl -I "$REMOTE_BASE_URL/login"
```

2. 确认公开设置返回 Turnstile 状态和 site key：

```bash
curl -s "$REMOTE_BASE_URL/api/v1/settings/public" | jq '.data | {turnstile_enabled, turnstile_site_key}'
```

3. 确认管理员设置中 secret 已配置：

```bash
curl -s -H "x-api-key: $REMOTE_ADMIN_API_KEY" \
  "$REMOTE_BASE_URL/api/v1/admin/settings" | jq '.data.turnstile_secret_key_configured'
```

4. 查看 Turnstile 服务日志：

```bash
curl -s -H "x-api-key: $REMOTE_ADMIN_API_KEY" \
  "$REMOTE_BASE_URL/api/v1/admin/ops/system-logs?component=service.turnstile&time_range=2h" \
  | jq '.data.items[] | {time, level, message, fields}'
```

日志应能看到 `error_codes`、`hostname`、`challenge_ts`、`http_status`、`remoteip_passed`、`remote_ip_private` 等诊断字段，不应出现 secret、完整 response token 或密码。

## Error Code 对照

| error code | 常见原因 | 处置 |
| --- | --- | --- |
| `invalid-input-response` | token 为空、格式异常、前端重复提交已消费 token、页面停留太久后提交旧 token | 刷新登录页并重新验证；检查前端是否 reset 失败 token |
| `timeout-or-duplicate` | token 已过期或被重复使用 | 重新完成验证；检查提交前 token 年龄拦截是否生效 |
| `invalid-input-secret` | secret key 不正确，或 site key 与 secret 不属于同一个 widget | 在 Cloudflare Turnstile 控制台重新核对并更新 secret |
| `missing-input-response` | 请求未携带 Turnstile token | 检查公开设置 site key、前端组件加载和 CSP |
| 域名不授权类客户端错误 | widget hostname 未包含当前访问域名 | 在 Cloudflare Turnstile widget 中加入当前 hostname |

## 真实 IP 链路

`server.trusted_proxies` 只填写你真正控制的反向代理 IP 或 CIDR，例如 Docker bridge 网段、宿主机本地反代地址、内部 Nginx/Caddy/Traefik 地址。不要在生产环境设置 `0.0.0.0/0`。

示例：

```yaml
server:
  trusted_proxies:
    - "172.16.0.0/12"   # Docker bridge 示例，按实际网段收窄更好
    - "127.0.0.1/32"    # 本机 Caddy/Nginx
    - "::1/128"
```

应用会按 `CF-Connecting-IP`、`X-Forwarded-For`、`X-Real-IP` 的顺序解析可信代理链；传给 Cloudflare `siteverify` 前还会清洗 remoteip。私网、回环、链路本地和非法 IP 会置空，因为 Turnstile 的 `remoteip` 是可选参数。

## CSP 与部署资产

默认 CSP 已允许 Turnstile：

- `script-src` 包含 `https://challenges.cloudflare.com`
- `frame-src` 包含 `https://challenges.cloudflare.com`
- `connect-src` 允许站点 API

发布前可检查响应头：

```bash
curl -I "$REMOTE_BASE_URL/login"
```

前端静态资源建议保留至少一个发布周期。带 hash 的 `/assets/*` 可以长期缓存，但部署时不要立刻删除旧 assets，避免用户打开旧登录页后引用的异步 chunk 404。

## 临时处置顺序

1. 先让用户刷新页面并重新完成验证。
2. 检查 Cloudflare Turnstile widget 的 hostname、site key、secret key 是否匹配。
3. 检查 `server.trusted_proxies` 与反代转发头，确认没有把 Docker 内网 IP 当成用户 IP。
4. 检查 CSP 是否阻断 Turnstile 脚本或 iframe。
5. 只有在确认大面积故障且管理员接受风险时，才临时关闭 Turnstile，并记录开始时间、结束时间、操作者和影响范围。恢复后必须重新启用。
