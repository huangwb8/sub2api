# Sub2API

<div align="center">

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8.svg)](https://golang.org/)
[![Vue](https://img.shields.io/badge/Vue-3.4+-4FC08D.svg)](https://vuejs.org/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-15+-336791.svg)](https://www.postgresql.org/)
[![Redis](https://img.shields.io/badge/Redis-7+-DC382D.svg)](https://redis.io/)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED.svg)](https://www.docker.com/)
[![License](https://img.shields.io/badge/License-LGPL%20v3-blue.svg)](https://www.gnu.org/licenses/lgpl-3.0)

**面向 AI 订阅分发与 API 配额管理的一体化网关平台**

中文 | [English](README_EN.md) | [日本語](README_JA.md)

</div>

## 🙏 致谢

本项目 fork 自 [Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api)，感谢原作者的开源贡献。如果你觉得这个项目有用，欢迎给 **[上游项目](https://github.com/Wei-Shaw/sub2api)** 点个 Star —— 这是对原作者最好的认可。如果你也认可本 fork 的改进，也欢迎 [Star 本项目](https://github.com/huangwb8/sub2api)，非常感谢。

## ✨ 本 Fork 的特色

基于上游项目持续维护和改进，以下是本 fork 的主要差异：

- **盈利面板重构**：订阅成本按查询时间窗内的真实比例分摊，解决历史累计分母导致的成本虚高与跨周期混淆
- **订阅计费倍率修复**：订阅配额消耗改为按 ActualCost 结算，分组倍率与用户专属倍率真实生效，0x 免费订阅正确反映到用量
- **支付安全加固**：管理端支付 provider 按类型剔除 privateKey / secretKey 等敏感凭据，编辑时空敏感字段默认保持原值
- **第三方上游兼容**：区分 OpenAI 官方与第三方兼容上游的 Responses URL 拼接策略，修复后台测试通过但真实网关请求失败的问题
- **OpenAI ctx_pool 修复**：恢复前端 off / ctx_pool / passthrough 三态选项，补齐回归测试覆盖
- **服务条款与隐私政策**：内置法律文档页面，管理员可在后台维护 Markdown 内容，站点自动生成 `/legal/terms` 与 `/legal/privacy` 公开链接
- **汇率系统透明化**：新增汇率波动影响分析文档与参数最佳实践，管理后台可调节缓存 TTL
- **订阅估算成本**：自动计算订阅 usage 的 estimated_cost_cny，盈利面板可展示完整成本曲线
- **支付前端优化**：支付宝桌面端改为纯跳转收银台，支付弹窗按可用屏幕动态计算尺寸，前端识别 is_mobile 传给后端提升 H5/PC 分流准确性
- **运营文档完善**：关键模型参数设置、汇率影响分析、管理员参数最佳实践等配套文档

完整变更记录见 [CHANGELOG.md](CHANGELOG.md)。

## 🎯 项目定位

Sub2API 是一个为 AI 订阅资源管理场景设计的 API 网关平台，把上游账号、用户 API Key、计费、调度、支付、订阅、自助购买和后台运营串成完整闭环。

### 核心能力

- 多上游账号统一接入，支持 OAuth、API Key 等不同凭证形态
- 用户 API Key 分发与权限隔离，支持分组、额度、速率与并发限制
- Token 级别计费、用量追踪、成本核算与统计展示
- 智能调度与粘性会话，适配多账号轮转、失败切换和模型映射
- 内置支付系统，支持 EasyPay、支付宝官方、微信官方、Stripe
- 用户自助充值、购买订阅套餐、续费、补差价升级、查看订单与支付状态
- 邀请返利系统默认关闭，可由管理员开启并配置返利比例、冻结期、有效期和用户专属邀请码
- 管理后台维护用户、分组、渠道、订阅、公告、兑换码、返利与系统设置
- 支持 iframe 嵌入外部页面，便于工单、文档、购买页集成
- 在线更新、版本检测、Docker 镜像发布自动化与数据管理联动

### 适合谁

- 想自建 AI 中转平台、统一团队 API 出口的人
- 面向终端用户提供 API 售卖与订阅套餐服务
- 按用户组配置不同模型权限、价格策略和并发限制
- 在一个后台里处理账号池调度、订单、支付、公告和运营配置

## 🚀 快速开始

### Docker Compose 一键部署

```bash
mkdir -p sub2api-deploy && cd sub2api-deploy
curl -sSL https://raw.githubusercontent.com/huangwb8/sub2api/main/deploy/docker-deploy.sh | bash
docker compose up -d
```

### 脚本安装二进制

```bash
curl -sSL https://raw.githubusercontent.com/huangwb8/sub2api/main/deploy/install.sh | sudo bash
sudo systemctl enable --now sub2api
```

> 在线体验可前往上游项目的演示站：[https://demo.sub2api.org/](https://demo.sub2api.org/)（admin@sub2api.org / admin123）

更多部署细节见 [deploy/README.md](deploy/README.md)。

## 📄 文档导航

| 文档 | 用途 |
|------|------|
| [deploy/README.md](deploy/README.md) | 部署总览、Docker 与二进制安装、版本发布自动化 |
| [docs/PAYMENT_CN.md](docs/PAYMENT_CN.md) | 支付配置文档（充值、订阅、Webhook、服务商） |
| [docs/PAYMENT.md](docs/PAYMENT.md) | English payment guide |
| [docs/关键模型参数设置.md](docs/关键模型参数设置.md) | 账号、分组与调度关键参数配置教程 |
| [docs/汇率波动如何影响用户购买行为与权益.md](docs/汇率波动如何影响用户购买行为与权益.md) | 人民币余额、美元 usage 与汇率波动关系说明 |
| [docs/管理员参数设置最佳实践.md](docs/管理员参数设置最佳实践.md) | 管理员系统参数配置推荐值 |
| [deploy/DATAMANAGEMENTD_CN.md](deploy/DATAMANAGEMENTD_CN.md) | 数据管理能力宿主机联动说明 |
| [CHANGELOG.md](CHANGELOG.md) | 版本历史与重要变更记录 |

## 🛠 技术栈

| 层级 | 技术 |
|------|------|
| 后端 | Go 1.26+, Gin, Ent, Wire |
| 前端 | Vue 3, TypeScript, Pinia, Vue Router, Tailwind CSS, Vite |
| 数据库 | PostgreSQL 15+ |
| 缓存 | Redis 7+ |
| 部署 | Docker Compose, systemd, GoReleaser |

## 💻 本地开发

### 后端

```bash
cd backend
go run ./cmd/server/
go test -tags=unit ./...
go test -tags=integration ./...
```

修改 `backend/ent/schema/*.go` 后需重新生成：

```bash
cd backend && go generate ./ent
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
make build && make test
```

## ⚠️ Nginx 反代注意

如果用 Nginx 反代且需要处理带下划线的请求头（如 `session_id`），在 `http` 块中启用：

```nginx
underscores_in_headers on;
```

否则 Nginx 默认丢弃此类请求头，影响多账号场景下的粘性会话路由。

## 🔗 生态与相关项目

| 项目 | 说明 |
|------|------|
| [Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api) | 上游项目，本 fork 的来源 |
| [sub2api-mobile](https://github.com/ckken/sub2api-mobile) | 移动端管理控制台，支持多后端切换 |

## 📜 许可证

本项目遵循 [GNU Lesser General Public License v3.0](LICENSE)。
