# Sub2API

<div align="center">

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8.svg)](https://golang.org/)
[![Vue](https://img.shields.io/badge/Vue-3.4+-4FC08D.svg)](https://vuejs.org/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-15+-336791.svg)](https://www.postgresql.org/)
[![Redis](https://img.shields.io/badge/Redis-7+-DC382D.svg)](https://redis.io/)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED.svg)](https://www.docker.com/)

<a href="https://trendshift.io/repositories/21823" target="_blank"><img src="https://trendshift.io/api/badge/repositories/21823" alt="Wei-Shaw%2Fsub2api | Trendshift" width="250" height="55"/></a>

**面向 AI 订阅分发与 API 配额管理的一体化网关平台**

中文 | [English](README_EN.md) | [日本語](README_JA.md)

</div>

> **Sub2API 官方仅使用 `sub2api.org` 与 `pincc.ai` 两个域名。其他使用 Sub2API 名义的网站可能为第三方部署或服务，与本项目无关，请自行甄别。**

## 项目定位

Sub2API 是一个为 AI 订阅资源管理场景设计的 API 网关平台。它把上游账号、用户 API Key、计费、调度、支付、订阅、自助购买和后台运营串成一个完整闭环，适合做自建 AI 中转平台、团队内部统一出口，或面向终端用户提供 API 与订阅服务。

如果你是从上游项目 fork 过来继续维护，这个仓库当前已经不只是“简单转发层”，而是更接近一个可直接落地运营的 AI API Gateway SaaS 基础设施。

## 现在版本能做什么

- 多上游账号统一接入，支持 OAuth、API Key 等不同凭证形态
- 用户 API Key 分发与权限隔离，支持分组、额度、速率与并发限制
- Token 级别计费、用量追踪、成本核算与统计展示
- 智能调度与粘性会话，适配多账号轮转、失败切换和模型映射
- 内置支付系统，支持 EasyPay、支付宝官方、微信官方、Stripe
- 普通用户自助充值、购买订阅套餐、续费、查看订单与支付状态
- 管理后台可直接维护用户、分组、渠道、订阅、公告、兑换码与系统设置
- 支持把外部系统通过 iframe 嵌入后台或用户侧页面，便于工单、文档、购买页集成
- 支持在线更新、版本检测、Docker 镜像发布自动化与数据管理联动能力

## 典型使用场景

- 把 Claude、Codex、Gemini 等上游账号统一收口，对团队成员发放 API Key
- 做面向终端用户的 API 售卖、订阅套餐和站内余额体系
- 给不同用户组配置不同模型权限、价格策略和并发限制
- 在一个后台里同时处理账号池调度、订单、支付、公告和运营配置

## 在线体验

体验地址：**[https://demo.sub2api.org/](https://demo.sub2api.org/)**

演示账号（共享演示环境；自建部署不会自动创建该账号）：

| 邮箱 | 密码 |
|------|------|
| admin@sub2api.org | admin123 |

## 快速开始

### 方式一：Docker Compose 一键部署

这是当前最适合大多数人的部署方式。

```bash
mkdir -p sub2api-deploy && cd sub2api-deploy
curl -sSL https://raw.githubusercontent.com/Wei-Shaw/sub2api/main/deploy/docker-deploy.sh | bash
docker compose up -d
```

默认会准备好：

- `docker-compose.yml`
- `.env`
- PostgreSQL / Redis / Sub2API 容器
- 自动生成的安全密钥

更多部署细节见 `deploy/README.md`。

### 方式二：脚本安装二进制

适合更偏传统的 Linux + systemd 部署方式。

```bash
curl -sSL https://raw.githubusercontent.com/Wei-Shaw/sub2api/main/deploy/install.sh | sudo bash
sudo systemctl enable --now sub2api
```

首次启动后可通过浏览器完成初始化向导。

## 文档导航

| 文档 | 用途 |
|------|------|
| `deploy/README.md` | 部署总览、Docker 与二进制安装、版本发布自动化 |
| `docs/PAYMENT_CN.md` | 中文支付配置文档，涵盖充值、订阅、Webhook 与服务商配置 |
| `docs/PAYMENT.md` | English payment guide |
| `deploy/DATAMANAGEMENTD_CN.md` | 启用“数据管理”能力的宿主机联动说明 |
| `docs/GITHUB_REPOSITORY_SETUP_TUTORIAL.md` | GitHub Release / Docker 镜像自动发布仓库配置教程 |
| `CHANGELOG.md` | 项目版本与重要变更记录 |

## 近期比较值得关注的能力

### 用户自助订阅闭环

现在的内置支付已经不只是余额充值，还支持普通用户直接购买和续费订阅套餐。对运营侧来说，这意味着：

- 订阅商品可直接在后台维护
- 用户端有完整购买、支付、结果页与订单页
- 支持 webhook 延迟场景下的主动补单恢复
- 支持余额支付订阅，减少无意义的第三方跳转

支付与订阅配置详见 `docs/PAYMENT_CN.md`。

### 内置支付而非外挂支付

过去社区常见做法是额外部署支付系统；现在 Sub2API 已经把支付闭环内置到主系统里，减少单独部署、对账和联调成本。你可以按业务需要选择：

- EasyPay 聚合支付
- 支付宝官方直连
- 微信官方直连
- Stripe 国际支付

### 运维与发布链路更完整

当前仓库已经补齐了版本驱动的发布思路，围绕 `backend/cmd/server/VERSION` 建立了：

- 版本同步校验
- GitHub Release 创建
- Docker 镜像自动发布与补发
- 本地校验脚本 `make verify-release-automation`

如果你打算长期维护自己的 fork，这一套流程会很省心。

### 数据管理联动

若要启用后台里的“数据管理”能力，可以额外部署宿主机进程 `datamanagementd`，主程序通过 Unix Socket 联动启用对应功能。详细说明见 `deploy/DATAMANAGEMENTD_CN.md`。

## 技术栈

| 层级 | 技术 |
|------|------|
| 后端 | Go 1.26+, Gin, Ent, Wire |
| 前端 | Vue 3, TypeScript, Pinia, Vue Router, Tailwind CSS, Vite |
| 数据库 | PostgreSQL 15+ |
| 缓存 | Redis 7+ |
| 部署 | Docker Compose, systemd, GoReleaser |

## 本地开发

### 后端

```bash
cd backend
go run ./cmd/server/
go test -tags=unit ./...
go test -tags=integration ./...
```

如果修改了 `backend/ent/schema/*.go`，记得重新生成：

```bash
cd backend
go generate ./ent
```

### 前端

```bash
cd frontend
pnpm install
pnpm dev
pnpm run lint:check
pnpm run typecheck
```

### 整体检查

```bash
make build
make test
```

## 使用 Nginx 反代时的一个坑

如果你用 Nginx 反代 Sub2API，并且会处理带下划线的请求头（例如 `session_id`），需要在 `http` 块中显式打开：

```nginx
underscores_in_headers on;
```

否则 Nginx 默认会丢弃这类请求头，进而影响多账号场景下的粘性会话路由。

## 生态项目

| 项目 | 说明 |
|------|------|
| ~~[Sub2ApiPay](https://github.com/touwaeriol/sub2apipay)~~ | 其能力现在已被 Sub2API 内置支付覆盖，通常不再需要额外部署 |
| [sub2api-mobile](https://github.com/ckken/sub2api-mobile) | 移动端管理控制台，支持多后端切换 |

## ❤️ 赞助商

> [想出现在这里？](mailto:support@pincc.ai)

<table>
<tr>
<td width="180" align="center" valign="middle"><a href="https://shop.pincc.ai/"><img src="assets/partners/logos/pincc-logo.png" alt="pincc" width="150"></a></td>
<td valign="middle"><b><a href="https://shop.pincc.ai/">PinCC</a></b> 是基于 Sub2API 搭建的官方中转服务，提供 Claude Code、Codex、Gemini 等主流模型的稳定中转，开箱即用，免去自建部署与运维烦恼。</td>
</tr>

<tr>
<td width="180"><a href="https://www.packyapi.com/register?aff=sub2api"><img src="assets/partners/logos/packycode.png" alt="PackyCode" width="150"></a></td>
<td>感谢 PackyCode 赞助本项目。使用<a href="https://www.packyapi.com/register?aff=sub2api">此链接</a>注册并在首次充值时填写优惠码 `sub2api`，可享受专属折扣。</td>
</tr>

<tr>
<td width="180"><a href="https://poixe.com/i/sub2api"><img src="assets/partners/logos/poixe.png" alt="PoixeAI" width="150"></a></td>
<td>感谢 Poixe AI 赞助本项目。通过 <a href="https://poixe.com/i/sub2api">专属链接</a> 注册可获得额外赠金。</td>
</tr>

<tr>
<td width="180"><a href="https://ctok.ai"><img src="assets/partners/logos/ctok.png" alt="CTok" width="150"></a></td>
<td>感谢 CTok.ai 赞助本项目，提供面向开发者的 AI 编程工具服务与社区支持。</td>
</tr>

<tr>
<td width="180"><a href="https://code.silkapi.com/"><img src="assets/partners/logos/silkapi.png" alt="silkapi" width="150"></a></td>
<td>感谢丝绸 API 赞助本项目，提供基于 Sub2API 的 Codex 高速稳定中转服务。</td>
</tr>

<tr>
<td width="180"><a href="https://ylscode.com/"><img src="assets/partners/logos/ylscode.png" alt="ylscode" width="150"></a></td>
<td>感谢伊莉思 Code 赞助本项目，提供企业级 Coding Agent 生产力服务与多模型订阅方案。</td>
</tr>

<tr>
<td width="180"><a href="https://www.aicodemirror.com/register?invitecode=KMVZQM"><img src="assets/partners/logos/AICodeMirror.jpg" alt="AICodeMirror" width="150"></a></td>
<td>感谢 AICodeMirror 赞助本项目，为 sub2api 用户提供专属注册优惠与企业级技术支持。</td>
</tr>

</table>

## English Note

Chinese is now the primary README for this fork. If you mainly need deployment and payment details in English, start here:

- `README_EN.md`
- `deploy/README.md`
- `docs/PAYMENT.md`
- `README_JA.md`
