# Sub2API

<div align="center">

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8.svg)](https://golang.org/)
[![Vue](https://img.shields.io/badge/Vue-3.4+-4FC08D.svg)](https://vuejs.org/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-15+-336791.svg)](https://www.postgresql.org/)
[![Redis](https://img.shields.io/badge/Redis-7+-DC382D.svg)](https://redis.io/)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED.svg)](https://www.docker.com/)

<a href="https://trendshift.io/repositories/21823" target="_blank"><img src="https://trendshift.io/api/badge/repositories/21823" alt="Wei-Shaw%2Fsub2api | Trendshift" width="250" height="55"/></a>

**AI サブスクリプション配分と API クォータ管理のための統合ゲートウェイ**

[中文](README.md) | [English](README_EN.md) | 日本語

</div>

> **Sub2API が公式に使用しているドメインは `sub2api.org` と `pincc.ai` のみです。その他の Sub2API 名義のサイトは第三者によるデプロイやサービスの可能性があり、本プロジェクトとは無関係です。ご利用の際はご自身でご確認ください。**

## プロジェクトの位置づけ

Sub2API は、AI サブスクリプション型リソース配分のために設計された API ゲートウェイです。上流アカウント、ユーザー API キー、課金、スケジューリング、決済、サブスクリプション、セルフサービス購入、管理運用を一つの流れにまとめます。

継続的にメンテナンスする fork にとって、このプロジェクトは単なる薄い中継レイヤーではなく、実運用しやすい AI API Gateway SaaS 基盤に近い形になっています。

## 現在できること

- OAuth や API キーなど複数の認証形態に対応したマルチ上流アカウント接続
- グループ、残高、レート制限、同時実行数を含むユーザー API キー管理
- トークン単位の課金、利用量追跡、コスト計算、統計表示
- スティッキーセッション、ローテーション、フェイルオーバー、モデルマッピングを含むスマートスケジューリング
- EasyPay、公式 Alipay、公式 WeChat Pay、Stripe に対応した内蔵決済
- ユーザーによる残高チャージ、サブスクリプション購入、更新、差額支払いによるアップグレード、注文確認、支払い状態確認
- ユーザー、グループ、チャネル、サブスクリプション、告知、クーポン、システム設定を扱える管理画面
- iframe による外部ページ埋め込みで、チケット、ドキュメント、購入フローを統合可能
- オンライン更新確認、リリース自動化、Docker イメージ公開、データ管理連携

## 主な利用シーン

- Claude、Codex、Gemini など複数の上流アカウントを一元化し、チームに API キーを配布する
- 一般ユーザー向けに API、サブスクリプション、残高型サービスを提供する
- ユーザーグループごとにモデル権限、価格戦略、同時実行制限を変える
- アカウントプール、注文、決済、告知、設定を一つの管理画面で運用する

## デモ

体験版: **[https://demo.sub2api.org/](https://demo.sub2api.org/)**

共有デモ環境用アカウント:

| メール | パスワード |
|------|------|
| admin@sub2api.org | admin123 |

## クイックスタート

### 方法1: Docker Compose ワンコマンド導入

ほとんどの利用者におすすめの方法です。

```bash
mkdir -p sub2api-deploy && cd sub2api-deploy
curl -sSL https://raw.githubusercontent.com/Wei-Shaw/sub2api/main/deploy/docker-deploy.sh | bash
docker compose up -d
```

これで次が準備されます。

- `docker-compose.yml`
- `.env`
- PostgreSQL / Redis / Sub2API コンテナ
- 自動生成されたセキュリティシークレット

詳しくは `deploy/README.md` を参照してください。

### 方法2: バイナリインストールスクリプト

従来型の Linux + systemd 運用向けです。

```bash
curl -sSL https://raw.githubusercontent.com/Wei-Shaw/sub2api/main/deploy/install.sh | sudo bash
sudo systemctl enable --now sub2api
```

初回起動後は Web セットアップウィザードで初期化します。

## ドキュメント案内

| 文書 | 用途 |
|------|------|
| `deploy/README.md` | デプロイ全体、Docker とバイナリ導入、リリース自動化 |
| `docs/PAYMENT_CN.md` | 中国語の決済設定ガイド。チャージ、サブスクリプション、Webhook、プロバイダ設定を含む |
| `docs/PAYMENT.md` | 英語の決済ガイド |
| `deploy/DATAMANAGEMENTD_CN.md` | データ管理機能のためのホスト側連携説明 |
| `docs/GITHUB_REPOSITORY_SETUP_TUTORIAL.md` | GitHub Release と Docker イメージ自動化の設定手順 |
| `CHANGELOG.md` | バージョン履歴と重要な変更記録 |

## 注目したい機能

### セルフサービス型サブスクリプション導線

内蔵決済は残高チャージだけでなく、一般ユーザーによるサブスクリプション購入、更新、差額支払いによるアップグレードにも対応しています。

- 管理画面でサブスクリプション商品を直接管理できる
- ユーザー側に購入、支払い、結果、注文ページが用意されている
- Webhook 遅延時も注文照会で回復できる
- サブスクリプションに対する残高支払いに対応している
- 同じ upgrade family 内で、現在の課金周期の残存価値を充当して上位プランへアップグレードできる
- プランスナップショットを持たない旧サブスクリプションは「アップグレード不可」と明示され、課金追跡性を守る
- ユーザー残高と決済金額は CNY、API usage / quota / limit は USD で表示し、金額の意味が混ざらないようにしている

詳細は `docs/PAYMENT_CN.md` を参照してください。

### 外付けではなく内蔵決済

決済を別サービスとして分ける必要が減り、Sub2API 本体の中で決済の閉ループを完結できます。

利用可能な決済方式:

- EasyPay
- 公式 Alipay
- 公式 WeChat Pay
- Stripe

### より整った運用とリリースフロー

現在のリポジトリでは `backend/cmd/server/VERSION` を中心としたバージョン駆動の公開フローが整っています。

- バージョン整合性チェック
- GitHub Release 作成
- Docker イメージの公開と補完
- `make verify-release-automation` によるローカル検証

長期的に fork を保守する場合に特に便利です。

### データ管理連携

管理画面の「データ管理」機能を有効にするには、ホスト側で `datamanagementd` を追加デプロイします。メインプロセスは Unix Socket を通じて連携します。詳細は `deploy/DATAMANAGEMENTD_CN.md` を参照してください。

## 技術スタック

| レイヤー | 技術 |
|------|------|
| バックエンド | Go 1.26+, Gin, Ent, Wire |
| フロントエンド | Vue 3, TypeScript, Pinia, Vue Router, Tailwind CSS, Vite |
| データベース | PostgreSQL 15+ |
| キャッシュ | Redis 7+ |
| デプロイ | Docker Compose, systemd, GoReleaser |

## ローカル開発

### バックエンド

```bash
cd backend
go run ./cmd/server/
go test -tags=unit ./...
go test -tags=integration ./...
```

`backend/ent/schema/*.go` を変更した場合は再生成します。

```bash
cd backend
go generate ./ent
```

### フロントエンド

```bash
cd frontend
pnpm install
pnpm dev
pnpm run lint:check
pnpm run typecheck
```

### 全体チェック

```bash
make build
make test
```

## Nginx リバースプロキシの注意

`session_id` のようにアンダースコアを含むヘッダーを扱う場合は、Nginx の `http` ブロックで次を有効にしてください。

```nginx
underscores_in_headers on;
```

有効にしないと Nginx がその種のヘッダーを捨て、マルチアカウント構成でスティッキーセッションが壊れる場合があります。

## エコシステム

| プロジェクト | 説明 |
|------|------|
| ~~[Sub2ApiPay](https://github.com/touwaeriol/sub2apipay)~~ | その役割の多くは Sub2API の内蔵決済で代替可能です |
| [sub2api-mobile](https://github.com/ckken/sub2api-mobile) | マルチバックエンド切り替え対応のモバイル管理コンソール |

## ❤️ スポンサー

> [掲載希望はこちら](mailto:support@pincc.ai)

<table>
<tr>
<td width="180" align="center" valign="middle"><a href="https://shop.pincc.ai/"><img src="assets/partners/logos/pincc-logo.png" alt="pincc" width="150"></a></td>
<td valign="middle"><b><a href="https://shop.pincc.ai/">PinCC</a></b> は Sub2API ベースの公式中継サービスで、Claude Code、Codex、Gemini などへの安定したアクセスを提供します。</td>
</tr>

<tr>
<td width="180"><a href="https://www.packyapi.com/register?aff=sub2api"><img src="assets/partners/logos/packycode.png" alt="PackyCode" width="150"></a></td>
<td>PackyCode の支援に感謝します。<a href="https://www.packyapi.com/register?aff=sub2api">このリンク</a>から登録し、初回チャージ時に `sub2api` を使うと割引があります。</td>
</tr>

<tr>
<td width="180"><a href="https://poixe.com/i/sub2api"><img src="assets/partners/logos/poixe.png" alt="PoixeAI" width="150"></a></td>
<td>Poixe AI の支援に感謝します。<a href="https://poixe.com/i/sub2api">紹介リンク</a>から登録すると特典があります。</td>
</tr>

<tr>
<td width="180"><a href="https://ctok.ai"><img src="assets/partners/logos/ctok.png" alt="CTok" width="150"></a></td>
<td>CTok.ai の支援に感謝します。AI プログラミングサービスと開発者コミュニティを提供しています。</td>
</tr>

<tr>
<td width="180"><a href="https://code.silkapi.com/"><img src="assets/partners/logos/silkapi.png" alt="silkapi" width="150"></a></td>
<td>SilkAPI の支援に感謝します。Sub2API ベースの高速な Codex 中継サービスを提供しています。</td>
</tr>

<tr>
<td width="180"><a href="https://ylscode.com/"><img src="assets/partners/logos/ylscode.png" alt="ylscode" width="150"></a></td>
<td>YLS Code の支援に感謝します。企業向け Coding Agent サービスと複数モデルのプランを提供しています。</td>
</tr>

<tr>
<td width="180"><a href="https://www.aicodemirror.com/register?invitecode=KMVZQM"><img src="assets/partners/logos/AICodeMirror.jpg" alt="AICodeMirror" width="150"></a></td>
<td>AICodeMirror の支援に感謝します。sub2api ユーザー向け特典と企業向け技術サポートを提供しています。</td>
</tr>

</table>
