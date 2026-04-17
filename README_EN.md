# Sub2API

<div align="center">

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8.svg)](https://golang.org/)
[![Vue](https://img.shields.io/badge/Vue-3.4+-4FC08D.svg)](https://vuejs.org/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-15+-336791.svg)](https://www.postgresql.org/)
[![Redis](https://img.shields.io/badge/Redis-7+-DC382D.svg)](https://redis.io/)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED.svg)](https://www.docker.com/)

<a href="https://trendshift.io/repositories/21823" target="_blank"><img src="https://trendshift.io/api/badge/repositories/21823" alt="Wei-Shaw%2Fsub2api | Trendshift" width="250" height="55"/></a>

**An integrated AI API gateway for subscription distribution and API quota management**

[中文](README.md) | English | [日本語](README_JA.md)

</div>

> **Sub2API officially uses only the domains `sub2api.org` and `pincc.ai`. Other websites using the Sub2API name may be third-party deployments or services and are not affiliated with this project. Please verify independently.**

## Positioning

Sub2API is an AI API gateway built for subscription-backed resource distribution. It connects upstream accounts, user API keys, billing, scheduling, payments, subscriptions, self-service purchasing, and admin operations into one workflow.

For a maintained fork, the project is now much more than a thin relay layer. It is closer to a production-ready AI API Gateway SaaS foundation.

## What It Does Now

- Unified multi-upstream account access with OAuth and API key based credentials
- User API key issuance with group, quota, rate limit, and concurrency isolation
- Token-level billing, usage tracking, cost accounting, and reporting
- Smart scheduling with sticky sessions, account rotation, failover, and model mapping
- Built-in payments with EasyPay, official Alipay, official WeChat Pay, and Stripe
- Self-service top-up, subscription purchase, renewal, upgrade-with-difference payment, order lookup, and payment status pages
- Admin console for users, groups, channels, subscriptions, announcements, promo codes, and system settings
- External page embedding through iframe for tickets, docs, purchase flows, and custom integrations
- Online update checks, release automation, Docker image publishing, and data-management integration

## Typical Use Cases

- Centralize Claude, Codex, Gemini, and other upstream accounts and issue API keys to a team
- Sell API access, subscription plans, and balance-based services to end users
- Apply different model permissions, pricing strategies, and concurrency limits by user group
- Operate account pools, orders, payments, announcements, and settings from one admin backend

## Demo

Try it here: **[https://demo.sub2api.org/](https://demo.sub2api.org/)**

Demo account for the shared environment only:

| Email | Password |
|------|------|
| admin@sub2api.org | admin123 |

## Quick Start

### Option 1: One-Command Docker Compose

This is the recommended deployment method for most users.

```bash
mkdir -p sub2api-deploy && cd sub2api-deploy
curl -sSL https://raw.githubusercontent.com/Wei-Shaw/sub2api/main/deploy/docker-deploy.sh | bash
docker compose up -d
```

This prepares:

- `docker-compose.yml`
- `.env`
- PostgreSQL / Redis / Sub2API containers
- generated security secrets

For full deployment details, see `deploy/README.md`.

### Option 2: Binary Install Script

For a more traditional Linux + systemd deployment:

```bash
curl -sSL https://raw.githubusercontent.com/Wei-Shaw/sub2api/main/deploy/install.sh | sudo bash
sudo systemctl enable --now sub2api
```

After first start, finish initialization through the web setup wizard.

## Documentation Map

| Document | Purpose |
|------|------|
| `deploy/README.md` | Deployment overview, Docker and binary install, release automation |
| `docs/PAYMENT_CN.md` | Chinese payment guide covering top-up, subscriptions, webhook, and provider config |
| `docs/PAYMENT.md` | English payment guide |
| `deploy/DATAMANAGEMENTD_CN.md` | Host-side integration for the data management feature |
| `docs/GITHUB_REPOSITORY_SETUP_TUTORIAL.md` | GitHub Release and Docker image automation setup |
| `CHANGELOG.md` | Version history and important project changes |

## Capabilities Worth Highlighting

### Self-Service Subscription Flow

Built-in payments now cover more than balance top-ups. End users can directly purchase, renew, and upgrade subscription plans with difference payment.

- Subscription products can be managed directly in the admin panel
- Users get purchase, payment, result, and order pages out of the box
- Delayed webhook scenarios can recover through active order verification
- Balance payment for subscriptions is supported to reduce unnecessary third-party redirects
- Higher-tier upgrades within the same upgrade family can reuse the remaining value of the current billing cycle as credit
- Legacy subscriptions without plan snapshots are clearly marked as not upgradeable to keep billing traceable
- User balances and payment amounts are shown in CNY, while API usage, quota, and limit data stay in USD to keep the two money semantics separate

See `docs/PAYMENT_CN.md` for detailed payment and subscription configuration.

### Built-In Payments Instead of an External Add-On

Payment no longer needs to live in a separate companion service. Sub2API now keeps the payment loop inside the main system, reducing deployment, reconciliation, and integration overhead.

Available providers:

- EasyPay
- Official Alipay
- Official WeChat Pay
- Stripe

### More Complete Ops and Release Flow

The repository now has a version-driven release flow centered around `backend/cmd/server/VERSION`, including:

- version sync checks
- GitHub Release creation
- Docker image publishing and backfill
- local verification with `make verify-release-automation`

This is especially useful if you are maintaining your own fork long term.

### Data Management Integration

If you want to enable the admin-side data management capability, deploy the host process `datamanagementd`. The main service enables the feature through a Unix socket handshake. See `deploy/DATAMANAGEMENTD_CN.md`.

## Tech Stack

| Layer | Technology |
|------|------|
| Backend | Go 1.26+, Gin, Ent, Wire |
| Frontend | Vue 3, TypeScript, Pinia, Vue Router, Tailwind CSS, Vite |
| Database | PostgreSQL 15+ |
| Cache | Redis 7+ |
| Deployment | Docker Compose, systemd, GoReleaser |

## Local Development

### Backend

```bash
cd backend
go run ./cmd/server/
go test -tags=unit ./...
go test -tags=integration ./...
```

If you change `backend/ent/schema/*.go`, regenerate Ent:

```bash
cd backend
go generate ./ent
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
make build
make test
```

## Nginx Reverse Proxy Note

If you proxy Sub2API through Nginx and need headers containing underscores such as `session_id`, enable this in the `http` block:

```nginx
underscores_in_headers on;
```

Otherwise Nginx drops those headers by default, which can break sticky session routing in multi-account setups.

## Ecosystem

| Project | Description |
|------|------|
| ~~[Sub2ApiPay](https://github.com/touwaeriol/sub2apipay)~~ | Its role is now largely covered by Sub2API built-in payments |
| [sub2api-mobile](https://github.com/ckken/sub2api-mobile) | Mobile admin console with multi-backend switching |

## ❤️ Sponsors

> [Want to appear here?](mailto:support@pincc.ai)

<table>
<tr>
<td width="180" align="center" valign="middle"><a href="https://shop.pincc.ai/"><img src="assets/partners/logos/pincc-logo.png" alt="pincc" width="150"></a></td>
<td valign="middle"><b><a href="https://shop.pincc.ai/">PinCC</a></b> is the official relay service built on Sub2API, offering stable access to Claude Code, Codex, Gemini, and other popular models.</td>
</tr>

<tr>
<td width="180"><a href="https://www.packyapi.com/register?aff=sub2api"><img src="assets/partners/logos/packycode.png" alt="PackyCode" width="150"></a></td>
<td>Thanks to PackyCode for supporting this project. Register through <a href="https://www.packyapi.com/register?aff=sub2api">this link</a> and use promo code `sub2api` on your first top-up for a discount.</td>
</tr>

<tr>
<td width="180"><a href="https://poixe.com/i/sub2api"><img src="assets/partners/logos/poixe.png" alt="PoixeAI" width="150"></a></td>
<td>Thanks to Poixe AI for supporting this project. Register via the <a href="https://poixe.com/i/sub2api">referral link</a> for an extra bonus.</td>
</tr>

<tr>
<td width="180"><a href="https://ctok.ai"><img src="assets/partners/logos/ctok.png" alt="CTok" width="150"></a></td>
<td>Thanks to CTok.ai for supporting this project with AI programming services and developer community resources.</td>
</tr>

<tr>
<td width="180"><a href="https://code.silkapi.com/"><img src="assets/partners/logos/silkapi.png" alt="silkapi" width="150"></a></td>
<td>Thanks to SilkAPI for supporting this project with high-speed Codex relay services built on Sub2API.</td>
</tr>

<tr>
<td width="180"><a href="https://ylscode.com/"><img src="assets/partners/logos/ylscode.png" alt="ylscode" width="150"></a></td>
<td>Thanks to YLS Code for supporting this project with enterprise-grade Coding Agent services and multi-model plans.</td>
</tr>

<tr>
<td width="180"><a href="https://www.aicodemirror.com/register?invitecode=KMVZQM"><img src="assets/partners/logos/AICodeMirror.jpg" alt="AICodeMirror" width="150"></a></td>
<td>Thanks to AICodeMirror for supporting this project with user benefits and enterprise technical support.</td>
</tr>

</table>
