# Sub2API

<div align="center">

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8.svg)](https://golang.org/)
[![Vue](https://img.shields.io/badge/Vue-3.4+-4FC08D.svg)](https://vuejs.org/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-15+-336791.svg)](https://www.postgresql.org/)
[![Redis](https://img.shields.io/badge/Redis-7+-DC382D.svg)](https://redis.io/)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED.svg)](https://www.docker.com/)
[![License](https://img.shields.io/badge/License-LGPL%20v3-blue.svg)](https://www.gnu.org/licenses/lgpl-3.0)

**AI サブスクリプション配分と API クォータ管理のための統合ゲートウェイ**

[中文](README.md) | [English](README_EN.md) | 日本語

</div>

## 🙏 謝辞

本プロジェクトは [Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api) のフォークです。原作者のオープンソースへの貢献に感謝します。このプロジェクトが役に立つと思ったら、**[上流プロジェクト](https://github.com/Wei-Shaw/sub2api)** に Star をお願いします。本フォークの改良も評価していただけたら、[このリポジトリの Star](https://github.com/huangwb8/sub2api) も歓迎です。

## ✨ 本フォークの特色

上流プロジェクトを基に継続的にメンテナンスと改良を行っています：

- **収益パネルの再構築**：サブスクリプションコストをクエリ時間窓内の実際の比率で按分し、過去の累積分母によるコストの水増しとクロスサイクル混同を解消
- **サブスクリプション課金倍率の修正**：サブスクリプション枠消費を ActualCost で決済し、グループ倍率とユーザー固有倍率が実際に反映されるように修正
- **決済セキュリティの強化**：管理画面の決済プロバイダーで秘密鍵などの機密情報を除去、編集時の空欄は「元の値を保持」として扱う
- **サードパーティ上流互換性**：OpenAI 公式とサードパーティ互換上流の URL 構築を区別し、テストの偽陽性を修正
- **OpenAI ctx_pool の修正**：フロントエンドの off / ctx_pool / passthrough 三態オプションを復元し、回帰テストを追加
- **利用規約とプライバシーポリシー**：管理画面で Markdown コンテンツを編集でき、`/legal/terms` と `/legal/privacy` の公開リンクを自動生成
- **為替レートの透明化**：為替変動影響分析ドキュメントとパラメータのベストプラクティス、管理画面のキャッシュ TTL 設定を追加
- **サブスクリプション推定コスト**：サブスクリプション usage の estimated_cost_cny を自動計算
- **決済 UI の改善**：Alipay デスクトップを直接リダイレクトに変更、決済モーダルを画面サイズに動的に調整
- **運用ドキュメントの充実**：主要モデルパラメータ設定、為替影響分析、管理者パラメータのベストプラクティスなど

詳細は [CHANGELOG.md](CHANGELOG.md) をご覧ください。

## 🎯 プロジェクトの位置づけ

Sub2API は、AI サブスクリプション型リソース配分のための API ゲートウェイです。上流アカウント、ユーザー API キー、課金、スケジューリング、決済、サブスクリプション、セルフサービス購入、管理運用を一つの流れにまとめます。

### 主な機能

- OAuth や API キーなど複数の認証形態に対応したマルチ上流アカウント接続
- グループ、残高、レート制限、同時実行数を含むユーザー API キー管理
- トークン単位の課金、利用量追跡、コスト計算、統計表示
- スティッキーセッション、ローテーション、フェイルオーバー、モデルマッピングを含むスマートスケジューリング
- EasyPay、公式 Alipay、公式 WeChat Pay、Stripe に対応した内蔵決済
- ユーザーによる残高チャージ、サブスクリプション購入、更新、差額支払いによるアップグレード、注文確認
- 招待リベート機能はデフォルト無効で、管理者がリベート率、凍結期間、有効期間、専用招待コードを設定可能
- ユーザー、グループ、チャネル、サブスクリプション、告知、クーポン、リベート、システム設定を扱える管理画面
- iframe による外部ページ埋め込みでチケット、ドキュメント、購入フローを統合
- オンライン更新確認、リリース自動化、Docker イメージ公開、データ管理連携

### 対象ユーザー

- Claude、Codex、Gemini などの上流アカウントを一元化し、チームに API キーを配布したい人
- エンドユーザー向けに API、サブスクリプション、残高サービスを提供したい人
- ユーザーグループごとにモデル権限、価格戦略、同時実行制限を変えたい人
- アカウントプール、注文、決済、告知、設定を一つの管理画面で運用したい人

## 🚀 クイックスタート

### Docker Compose

```bash
mkdir -p sub2api-deploy && cd sub2api-deploy
curl -sSL https://raw.githubusercontent.com/huangwb8/sub2api/main/deploy/docker-deploy.sh | bash
docker compose up -d
```

### バイナリインストール

```bash
curl -sSL https://raw.githubusercontent.com/huangwb8/sub2api/main/deploy/install.sh | sudo bash
sudo systemctl enable --now sub2api
```

> ライブデモは上流プロジェクトのデモサイトをご覧ください：[https://demo.sub2api.org/](https://demo.sub2api.org/)（admin@sub2api.org / admin123）

詳細は [deploy/README.md](deploy/README.md) を参照してください。

## 📄 ドキュメント案内

| 文書 | 用途 |
|------|------|
| [deploy/README.md](deploy/README.md) | デプロイ全体、Docker とバイナリ導入、リリース自動化 |
| [docs/PAYMENT_CN.md](docs/PAYMENT_CN.md) | 中国語の決済設定ガイド |
| [docs/PAYMENT.md](docs/PAYMENT.md) | 英語の決済ガイド |
| [deploy/DATAMANAGEMENTD_CN.md](deploy/DATAMANAGEMENTD_CN.md) | データ管理機能のためのホスト側連携説明 |
| [docs/GITHUB_REPOSITORY_SETUP_TUTORIAL.md](docs/GITHUB_REPOSITORY_SETUP_TUTORIAL.md) | GitHub Release と Docker イメージ自動化の設定手順 |
| [CHANGELOG.md](CHANGELOG.md) | バージョン履歴と重要な変更記録 |

## 🛠 技術スタック

| レイヤー | 技術 |
|------|------|
| バックエンド | Go 1.26+, Gin, Ent, Wire |
| フロントエンド | Vue 3, TypeScript, Pinia, Vue Router, Tailwind CSS, Vite |
| データベース | PostgreSQL 15+ |
| キャッシュ | Redis 7+ |
| デプロイ | Docker Compose, systemd, GoReleaser |

## 💻 ローカル開発

### バックエンド

```bash
cd backend
go run ./cmd/server/
go test -tags=unit ./...
go test -tags=integration ./...
```

`backend/ent/schema/*.go` を変更した場合は再生成します：

```bash
cd backend && go generate ./ent
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
make build && make test
```

## ⚠️ Nginx リバースプロキシの注意

`session_id` のようにアンダースコアを含むヘッダーを扱う場合は、Nginx の `http` ブロックで次を有効にしてください：

```nginx
underscores_in_headers on;
```

有効にしないと Nginx がその種のヘッダーを捨て、マルチアカウント構成でスティッキーセッションが壊れる場合があります。

## 🔗 エコシステム

| プロジェクト | 説明 |
|------|------|
| [Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api) | 上流プロジェクト、本フォークの元 |
| [sub2api-mobile](https://github.com/ckken/sub2api-mobile) | マルチバックエンド切り替え対応のモバイル管理コンソール |

## 📜 ライセンス

本プロジェクトは [GNU Lesser General Public License v3.0](LICENSE) の下でライセンスされています。
