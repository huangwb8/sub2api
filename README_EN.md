# Sub2API

<div align="center">

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8.svg)](https://golang.org/)
[![Vue](https://img.shields.io/badge/Vue-3.4+-4FC08D.svg)](https://vuejs.org/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-15+-336791.svg)](https://www.postgresql.org/)
[![Redis](https://img.shields.io/badge/Redis-7+-DC382D.svg)](https://redis.io/)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED.svg)](https://www.docker.com/)
[![License](https://img.shields.io/badge/License-LGPL%20v3-blue.svg)](https://www.gnu.org/licenses/lgpl-3.0)

**An integrated AI API gateway for subscription distribution and API quota management**

[中文](README.md) | English | [日本語](README_JA.md)

</div>

## 🙏 Acknowledgements

This project is forked from [Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api). Many thanks to the original author for the open-source contribution. If you find this project useful, please consider giving the **[upstream project](https://github.com/Wei-Shaw/sub2api)** a Star. And if you also appreciate the improvements in this fork, a [Star for this repo](https://github.com/huangwb8/sub2api) would be greatly appreciated.

## ✨ What This Fork Offers

Maintained and improved on top of the upstream project:

- **Profitability panel refactor**: Subscription costs are now proportionally allocated within the query time window, fixing inflated costs and cross-cycle confusion caused by historical cumulative denominators
- **Subscription billing multiplier fix**: Subscription quota consumption now settles by ActualCost, making group multipliers and user-specific multipliers actually effective
- **Payment security hardening**: Admin-side payment provider responses strip sensitive credentials (privateKey, secretKey, etc.); empty sensitive fields during edits mean "keep original value"
- **Third-party upstream compatibility**: Differentiates URL construction for OpenAI official vs. third-party compatible upstreams, fixing false-positive test results
- **OpenAI ctx_pool fix**: Restored the full off / ctx_pool / passthrough frontend options with regression test coverage
- **Terms of service & privacy policy**: Built-in legal document pages with Markdown editing in the admin panel and auto-generated public links at `/legal/terms` and `/legal/privacy`
- **Exchange rate transparency**: Added exchange rate volatility analysis docs, parameter best practices, and admin cache TTL settings
- **Subscription estimated cost**: Automatically calculates estimated_cost_cny for subscription usage, enabling complete cost curves in the profitability panel
- **Payment UX improvements**: Alipay desktop flow changed to direct redirect; payment modal dynamically sized to screen; frontend is_mobile flag passed to backend for better H5/PC routing
- **Improved operational docs**: Key model parameter settings, exchange rate impact analysis, and admin parameter best practices

See [CHANGELOG.md](CHANGELOG.md) for the full change history.

## 🎯 What It Does

Sub2API is an AI API gateway platform designed for subscription-backed resource distribution. It connects upstream accounts, user API keys, billing, scheduling, payments, subscriptions, self-service purchasing, and admin operations into one workflow.

### Core Capabilities

- Unified multi-upstream account access with OAuth and API key credentials
- User API key issuance with group, quota, rate limit, and concurrency isolation
- Token-level billing, usage tracking, cost accounting, and reporting
- Smart scheduling with sticky sessions, account rotation, failover, and model mapping
- Built-in payments with EasyPay, official Alipay, official WeChat Pay, and Stripe
- Self-service top-up, subscription purchase, renewal, upgrade-with-difference payment, order lookup, and payment status pages
- Admin console for users, groups, channels, subscriptions, announcements, promo codes, and system settings
- External page embedding through iframe for tickets, docs, purchase flows, and custom integrations
- Online update checks, release automation, Docker image publishing, and data-management integration

### Who Is This For

- Centralize Claude, Codex, Gemini, and other upstream accounts and issue API keys to a team
- Sell API access, subscription plans, and balance-based services to end users
- Apply different model permissions, pricing strategies, and concurrency limits by user group
- Operate account pools, orders, payments, announcements, and settings from one admin backend

## 🚀 Quick Start

### Docker Compose

```bash
mkdir -p sub2api-deploy && cd sub2api-deploy
curl -sSL https://raw.githubusercontent.com/huangwb8/sub2api/main/deploy/docker-deploy.sh | bash
docker compose up -d
```

### Binary Install Script

```bash
curl -sSL https://raw.githubusercontent.com/huangwb8/sub2api/main/deploy/install.sh | sudo bash
sudo systemctl enable --now sub2api
```

> For a live demo, visit the upstream project's demo site: [https://demo.sub2api.org/](https://demo.sub2api.org/) (admin@sub2api.org / admin123)

See [deploy/README.md](deploy/README.md) for full deployment details.

## 📄 Documentation

| Document | Purpose |
|------|------|
| [deploy/README.md](deploy/README.md) | Deployment overview, Docker and binary install, release automation |
| [docs/PAYMENT_CN.md](docs/PAYMENT_CN.md) | Chinese payment guide (top-up, subscriptions, webhook, provider config) |
| [docs/PAYMENT.md](docs/PAYMENT.md) | English payment guide |
| [deploy/DATAMANAGEMENTD_CN.md](deploy/DATAMANAGEMENTD_CN.md) | Host-side integration for the data management feature |
| [docs/GITHUB_REPOSITORY_SETUP_TUTORIAL.md](docs/GITHUB_REPOSITORY_SETUP_TUTORIAL.md) | GitHub Release and Docker image automation setup |
| [CHANGELOG.md](CHANGELOG.md) | Version history and important project changes |

## 🛠 Tech Stack

| Layer | Technology |
|------|------|
| Backend | Go 1.26+, Gin, Ent, Wire |
| Frontend | Vue 3, TypeScript, Pinia, Vue Router, Tailwind CSS, Vite |
| Database | PostgreSQL 15+ |
| Cache | Redis 7+ |
| Deployment | Docker Compose, systemd, GoReleaser |

## 💻 Local Development

### Backend

```bash
cd backend
go run ./cmd/server/
go test -tags=unit ./...
go test -tags=integration ./...
```

After modifying `backend/ent/schema/*.go`, regenerate:

```bash
cd backend && go generate ./ent
```

### Frontend

```bash
cd frontend
pnpm install
pnpm dev
pnpm run lint:check
pnpm run typecheck
```

### Full Checks

```bash
make build && make test
```

## ⚠️ Nginx Reverse Proxy Note

If you proxy Sub2API through Nginx and need headers containing underscores such as `session_id`, enable this in the `http` block:

```nginx
underscores_in_headers on;
```

Otherwise Nginx drops those headers by default, which can break sticky session routing in multi-account setups.

## 🔗 Ecosystem

| Project | Description |
|------|------|
| [Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api) | Upstream project, the source of this fork |
| [sub2api-mobile](https://github.com/ckken/sub2api-mobile) | Mobile admin console with multi-backend switching |

## 📜 License

This project is licensed under the [GNU Lesser General Public License v3.0](LICENSE).
