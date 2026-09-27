# levelog

現実のデイリーミッションを達成すると経験値(XP)を獲得し、自分自身がレベルアップしていく、成長型Todo管理Webアプリです。

**「不正を監視するのではなく、自分に嘘をつかず成長することが一番得になるアプリ」** をコンセプトに、他人と競うランキングは持たず、過去の自分との比較と継続だけを扱います。ミッションの完了は自己申告制で、確認ダイアログなしのワンタップで完了できます。未達成によるXP減少や罰則もありません。

## 技術構成

| レイヤー | 技術 |
| --- | --- |
| フロントエンド | React 19 + TypeScript + Vite + Tailwind CSS v4 + React Router |
| バックエンド | Go 1.23 標準ライブラリ中心（`net/http` の `ServeMux`）+ REST API |
| DB ドライバ | `github.com/jackc/pgx/v5`（`database/sql` 経由） |
| パスワードハッシュ | `golang.org/x/crypto/bcrypt` |
| データベース | PostgreSQL 16 |
| 実行環境 | Docker Compose |

バックエンドは標準ライブラリの `net/http`（Go 1.22+ の `ServeMux` メソッド・パスパターン）のみでルーティングしており、外部Webフレームワークは使用していません。追加の依存はPostgresドライバとbcryptのみです。

## ディレクトリ構成

```
levelog/
├── docker-compose.yml
├── .env.example
├── backend/
│   ├── cmd/api/main.go            # エントリポイント
│   ├── internal/
│   │   ├── config/                # 環境変数の読み込み
│   │   ├── db/                    # 接続 + マイグレーション実行
│   │   ├── model/                 # ドメインモデル（User, MissionTemplate, DailyMission ...）
│   │   ├── repository/            # PostgreSQL実装（database/sql）
│   │   ├── service/                # ビジネスロジック（認証・XP・レベル判定・重複防止）
│   │   ├── handler/               # HTTPハンドラー + ルーティング
│   │   ├── middleware/            # 認証・CORS・ロギング
│   │   ├── httpx/                 # JSONレスポンス/エラー共通処理
│   │   └── levelup/               # レベル計算の共通ロジック（テスト済み）
│   ├── migrations/                # SQLマイグレーション（embedで同梱）
│   └── Dockerfile
├── monitoring/alerts/             # Grafana Cloud用アラートルール + promtool単体テスト
└── frontend/
    ├── src/
    │   ├── api/                   # APIクライアント・型定義
    │   ├── context/AuthContext.tsx
    │   ├── components/            # UIコンポーネント（進捗バー・レベルアップ演出等）
    │   ├── pages/                 # ホーム/ミッション管理/履歴/設定/ログイン
    │   └── App.tsx / main.tsx
    └── Dockerfile
```

## 必要な環境変数

`.env.example` を参照してください。

| 変数名 | 説明 | デフォルト |
| --- | --- | --- |
| `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` | PostgreSQLの認証情報 | `levelog` |
| `POSTGRES_PORT` | ホスト側に公開するPostgresポート | `5432` |
| `API_PORT` | ホスト側に公開するAPIポート | `8080` |
| `DATABASE_URL` | バックエンドが接続するPostgres DSN | `postgres://levelog:levelog@db:5432/levelog?sslmode=disable` |
| `FRONTEND_ORIGIN` | CORSで許可するフロントエンドのオリジン | `http://localhost:5173` |
| `COOKIE_SECURE` | セッションCookieに`Secure`属性を付けるか（HTTPS配信時は`true`） | `false` |
| `COOKIE_DOMAIN` | セッションCookieの`Domain`属性（空欄可） | (空) |
| `FRONTEND_PORT` | ホスト側に公開するフロントエンドポート | `5173` |
| `VITE_API_BASE_URL` | ブラウザがAPIへアクセスするベースURL | `http://localhost:8080` |

## 起動方法（Docker Compose）

```bash
cp .env.example .env
docker compose up --build
```

- フロントエンド: http://localhost:5173
- バックエンドAPI: http://localhost:8080/api/health
- PostgreSQL: `localhost:5432`

起動時にバックエンドが自動でDBマイグレーションを適用するため、追加の初期化コマンドは不要です。初回はブラウザで http://localhost:5173 を開き、アカウント登録から始めてください。

ポートが既に使用中の場合は `.env` の `API_PORT` / `FRONTEND_PORT` / `POSTGRES_PORT` を変更し、`VITE_API_BASE_URL` もAPIポートに合わせて変更してください。

停止する場合は `docker compose down`（DBデータを保持したまま停止）、データも削除する場合は `docker compose down -v` を実行してください。

## マイグレーション方法

マイグレーションSQLは `backend/migrations/*.sql` にあり、`backend/migrations/embed.go` でバイナリに埋め込まれています。バックエンド起動時（`cmd/api/main.go`）に `schema_migrations` テーブルを見て未適用のファイルだけを自動的に適用するため、通常は手動実行不要です。

手動でマイグレーションだけを適用したい場合は、Postgresを起動した状態でバックエンドを起動すれば適用されます。

```bash
docker compose up db -d
cd backend
DATABASE_URL="postgres://levelog:levelog@localhost:5432/levelog?sslmode=disable" go run ./cmd/api
```

新しいマイグレーションを追加する場合は `backend/migrations/000N_xxx.sql` のようにファイル名の連番を進めて追加してください（ファイル名の辞書順に適用されます）。

## テスト方法

### バックエンド（Go）

```bash
cd backend
go test ./...
go vet ./...
```

ビジネスロジック（`internal/service`）はリポジトリをinterfaceとして扱っており、テストではPostgresを使わないインメモリ実装（`fakes_test.go`）に差し替えてXPの整合性・権限チェック・日付チェックを検証しています。`config` / `httpx` / `middleware` パッケージにも依存なしのユニットテストがあります。含まれるテスト:

- `levelup` パッケージ: XPからレベル・進捗を計算するテスト
- `service` パッケージ:
  - ミッション完了で正しいXPが付与される
  - 同じミッションを二重完了してもXPが増えない
  - 完了取り消しでXPが相殺される
  - 未完了の再取り消しでXPが増えない（何度トグルしても0/満額の往復に収まる）
  - 過去日のミッションを変更できない
  - 他ユーザーのミッションを変更できない
  - デイリーミッションが重複生成されない
  - 難易度とXPの対応が正しい
- `config` パッケージ: 環境変数の読み込み・デフォルト値・不正な値でのエラー
- `httpx` パッケージ: JSON応答・エラー変換（apperror/タイムアウト/内部エラーの詳細を漏らさないこと）・リクエストデコード
- `middleware` パッケージ: CORS（許可/非許可オリジン、プリフライト）、リクエストID、リクエストタイムアウト、構造化ログ出力

#### 統合テスト（実際のPostgresを使用）

`internal/handler` には、ルーター全体（ミドルウェア＋ハンドラー＋サービス＋リポジトリ）を実際のPostgresに対して検証する統合テストがあります。`TEST_DATABASE_URL` が未設定の場合はすべてスキップされるため、通常の `go test ./...` には影響しません。

```bash
docker compose up -d db
TEST_DATABASE_URL="postgres://levelog:levelog@localhost:5432/levelog?sslmode=disable" \
  go test ./internal/handler/... -v
```

含まれるテスト:

- ヘルスチェック（`/health/live` / `/health/ready` / `/api/health`）、未定義ルートの404
- 認証フロー（未認証アクセスの拒否、登録直後のログイン状態、誤ったパスワードの拒否、ログアウト後のセッション失効）
- ミッションのライフサイクル（作成→今日のミッション生成→完了でXP付与→二重完了で追加付与なし→取り消しでXP相殺）
- **XP二重付与防止（実DB・並行リクエスト）**: 同一デイリーミッションへ15並行で完了リクエストを送り、`SELECT ... FOR UPDATE`による行ロックにより最終的なXPが単発分しか加算されないことを検証（インメモリfakeでは検証できない、実トランザクションに依存するテスト）
- 認可（他ユーザーのミッションテンプレート／デイリーミッションを閲覧・編集・削除・完了できないこと）

### フロントエンド（TypeScript）

```bash
cd frontend
npm install
npx tsc -b        # 型チェック
npm run lint       # oxlint
npm run test       # Vitest（AuthContextのセッション切れフロー、ErrorBoundary、404ページ、APIクライアント等）
npm run build      # 本番ビルド確認
```

## 現在のMVPの機能

- ユーザー登録・ログイン・ログアウト（httpOnly Cookieによるセッション維持、bcryptによるパスワードハッシュ化）
- タイムゾーンを保持したユーザー情報（初期値 `Asia/Tokyo`）
- 繰り返しデイリーミッションの作成・編集・有効化/無効化・削除（論理削除）
- 難易度固定XP（EASY:10 / NORMAL:20 / HARD:40 / EXTREME:80）
- 今日のミッション一覧（ユーザーのタイムゾーンでの「今日」の分だけを初回アクセス時に生成、重複生成なし）
- ワンタップでの完了・当日中に限り取り消し可能
- 同一ミッションの二重完了防止（DBトランザクション内でのXP付与）
- XP履歴から導出される合計XP・レベル・次のレベルまでの進捗バー
- レベルアップ時のゲーム風モーダル演出（オリジナルデザイン、CSSアニメーションのみ）
- 過去7日間の履歴（日別の達成数・ミッション名・完了状態・獲得XP）
- 自己申告の思想を伝えるバナー（ホーム画面上部、閉じると再表示しない）
- スマートフォン優先のダークテーマUI + 下部ナビゲーション、デスクトップでは中央寄せの最大幅レイアウト

## 今後の拡張候補

- ミッションの並び替え・カテゴリ分け
- 週間/月間の集計グラフ（現状は直近7日間の一覧表示のみ）
- タイムゾーンの設定画面からの変更
- パスワードリセット・メール確認フロー
- 2要素認証

アプリケーションレベルのレート制限・HTTPS終端は実装済みです（それぞれ「セキュリティ」節を参照）。

## 既知の制約

- 過去日の履歴は、ユーザーがその日にアプリを開いて日次ミッションが生成された日のみ記録されます（深夜バッチ生成は仕様上行わないため）。

## 本番用コンテナ構成

`docker-compose.yml`（開発用、`db`込み・フロントエンドはVite devサーバー）とは別に、`docker-compose.prod.yml`を用意しています。

- `backend/Dockerfile`: マルチステージビルド、`CGO_ENABLED=0`、非rootユーザー実行、`HEALTHCHECK`付き（`/health/live`）
- `frontend/Dockerfile`: `vite build`の成果物を`nginxinc/nginx-unprivileged`（非root）で配信するマルチステージビルド。`nginx.conf`が`/api/`・`/health/`を同一オリジンでバックエンドへプロキシするため、ブラウザは常に単一オリジンとのみ通信し、本番のクロスオリジンCORS/Cookie設定を回避できます
- `frontend/nginx.conf`: ポート8080（平文HTTP、`/health/`・`/healthz`以外はHTTPSへ301リダイレクト）とポート8443（TLS終端。証明書は`/etc/nginx/tls/`から読み込み）の2構成。TLSはロードバランサではなく各アプリサーバのNginxで終端します（詳細は`docs/tls-design.md`）。証明書はLet's EncryptのDNS-01（certbot公式のさくらのクラウードDNSプラグイン）で初回だけ手動発行し、以降は`certbot.timer`とデプロイフックが更新からNginxのリロードまで自動で行います（`docs/tls-design.md`4節）
- `docker-compose.prod.yml`は`db`を含みません。本番DBは別途管理されるアプライアンス（フェーズ9で構築済み）を`DATABASE_URL`経由で指定してください。`web`サービスは`TLS_CERT_DIR`（`fullchain.pem`・`privkey.pem`を含むディレクトリ）の指定が必須です

```bash
TLS_CERT_DIR=/path/to/certs docker compose -f docker-compose.prod.yml up -d --build
```

`DATABASE_URL` / `FRONTEND_ORIGIN` / `TLS_CERT_DIR`は必須（未設定だとエラーで起動しません）。`COOKIE_SECURE`は既定で`true`です。ホスト側のポートは既定で`HTTP_PORT=80` / `HTTPS_PORT=443`（環境変数で変更可）。ローカルで動作確認する場合は`docker-compose.yml`の`db`サービスを別途起動し、自己署名証明書を`TLS_CERT_DIR`に置いた上で、`docs/production-roadmap.md`のフェーズ4・11に記載した手順を参照してください（`-f`を2つ重ねてマージ起動すると`api`のポート公開設定が意図せず引き継がれるため非推奨です）。

## CI

`main`へのPull Request・push時に`.github/workflows/ci.yml`が以下を自動実行します（`terraform apply`やクラウド認証情報を要する処理は一切含みません）。

- **secret-scan**: gitleaksによるリポジトリ全体（コミット履歴含む）の秘密情報混入チェック
- **backend**: `gofmt`チェック・`go vet`・`go build`・`go test ./... -race`（フェイクリポジトリによるユニットテストのみ、DB接続なし）
- **backend-integration**: Postgresサービスコンテナ（`postgres:16-alpine`）を使い、`go test ./internal/handler/... -v -race`で実DBに対する統合テスト（認証・ミッション・XP二重付与防止・認可）を実行
- **frontend**: `tsc -b`（型チェック）・`npm run lint`（oxlint）・`npm run test -- --run`（Vitest）・`npm run build`
- **docker-build**: `backend/Dockerfile` / `frontend/Dockerfile`のビルドが壊れていないことを確認（イメージのpush・レジストリ認証は行いません）
- **monitoring-rules**: `promtool check rules` / `promtool test rules`で、Grafana Cloud用アラートルール(`monitoring/alerts/`)の構文と、発火する・しないの単体テストを実行
- **shellcheck**: `.github/scripts/`のデプロイ・スモークテスト用シェルスクリプトの静的解析
- **terraform-test**: `terraform test`(モックプロバイダ)でTerraformモジュールの単体テスト(`modules/dns_record`のゾーン名の完全一致チェックなど)
- **terraform-fmt** / **terraform-validate**: `terraform fmt -check -recursive`と、dns/staging/production各環境での`terraform validate`（さくらのクラウード認証情報は一切与えず、静的な構文・スキーマ検証のみ）

## CD

`.github/workflows/cd.yml`が、`main`でのCI成功をトリガーにstaging環境へ自動デプロイします（イメージをGHCRへビルド・push → SSHでアプリサーバへ`docker compose pull && up -d`）。production環境へのデプロイは常に`workflow_dispatch`による手動実行のみで、GitHub Environmentの`production`にRequired reviewersを設定して人の承認を必須にすることを想定しています。ロールバックは`workflow_dispatch`の`image_tag`入力に過去のコミットSHAを指定して再実行します。詳細な設計・必要なSecrets一覧は`docs/deployment-design.md`を参照してください。

各サーバーへのデプロイ後には`.github/scripts/smoke-test.sh`が自動で実行されます(HTTPSリダイレクト・SPA配信・セキュリティヘッダー・未認証時の401・ログインAPIの応答・証明書の残り日数を確認。データは書き込みません)。`/health/ready`はDB疎通しか見ないため、ヘルスチェックは通るのにログインが壊れている、といった退行をここで止めます。

**注意**: アプリサーバ自体がまだ`terraform apply`されておらず実在しないため、このワークフローはまだ実行できません（デプロイ先のSecrets未設定）。

## 監視・ログ

- **アプリケーションメトリクス**: `GET /metrics`（`levelog_http_requests_total`・`levelog_http_request_duration_seconds`、Prometheusテキスト形式）を、アプリ本体（`PORT`、既定`8080`）とは別ポート（`METRICS_PORT`、既定`9090`）で公開しています。`docker-compose.prod.yml`はこのポートを`127.0.0.1`のみに公開し、外部からは到達できません
- **node_exporter**: 各アプリサーバ上に`127.0.0.1:9100`でリッスンするsystemdサービスとして導入済み（`terraform/modules/app_server`）
- **収集・転送**: 各アプリサーバ上で稼働するGrafana Alloy（`docker run --network host`のsystemdサービス）が、node_exporterとアプリの`/metrics`をスクレイプし、`api`/`web`コンテナのログをDockerソケット経由でtailして、Grafana Cloud（Prometheus互換メトリクス + Loki互換ログ）へアウトバウンドで送信します。インバウンドの穴は一切開けません
- **DNS**: `matsu0122.com`はVercelで管理されており、`levelog.matsu0122.com`以下だけをさくらのクラウードDNSに委任します。ゾーンは`terraform/environments/dns`、各環境のAレコードは`terraform/modules/dns_record`でTerraform管理しています(`docs/tls-design.md`6節)
- **外形監視**: さくらのクラウードの`simple_monitor`がstaging/production両環境の公開URLの`/health/live`を外部から定期チェックし、Slackへ通知します。TLS証明書の残り日数も監視し、14日を切ると通知します(自動更新が止まっている合図)
- **アラートルール**: 5xxエラー率・p95レイテンシ・ディスク・メモリ・スクレイプ失敗・サーバーからの送信停止の7ルールを`monitoring/alerts/levelog.rules.yml`に定義し、`promtool`で単体テストしています。Grafana Cloudへは`mimirtool rules load`で登録します(`docs/monitoring-design.md`6.1節)
- 設計の詳細（選定理由・アラートルール案・必要な環境変数一覧・残課題）は`docs/monitoring-design.md`を参照してください。**Grafana Cloudアカウントの作成・APIキー発行はまだ行っていません**（各サーバーの`/etc/levelog/monitoring.env`への設定はアプリサーバ構築後の手動作業です）。

## バックアップと復元

- **PostgreSQL**: 日次バックアップに加え、production環境はPITR（継続バックアップ）を既定で有効化しています。PITR用のNFSストレージはTerraformが自己プロビジョニングするため、事前準備は不要です（`terraform/modules/database`）
- **アプリサーバ**: 各サーバーのboot diskを週次でスナップショットバックアップしています（`sakuracloud_auto_backup`、`terraform/modules/app_server`）。`.env`・`monitoring.env`・TLS秘密鍵など、Git・Terraformいずれの管理下にもない手動投入ファイルを保護する目的です
- 「バックアップ取得 → データ消失 → 復元 → アプリケーションからの疎通確認」という一連の流れは、ローカルの開発用Postgresに対して`pg_dump`/`pg_restore`で実際にリハーサル済みです。詳細・復元runbookは`docs/backup-restore-design.md`と`docs/database-design.md`8節を参照してください

## セキュリティ

- **認証**: bcryptによるパスワードハッシュ、`crypto/rand`製の32バイトセッショントークン（サーバーはハッシュのみ保存）、Cookieは`HttpOnly` / `Secure`（本番） / `SameSite=Strict`。ログイン・登録エンドポイントにはクライアントIP単位のレート制限を実装しています
- **依存パッケージ・コンテナイメージの脆弱性スキャン**: `govulncheck`（Go）・`npm audit`（フロントエンド）・`trivy`（コンテナイメージ、HIGH/CRITICAL）をCI（`.github/workflows/ci.yml`）に組み込み、Pull Requestごとに継続的に検証します
- **フロントエンドのHTTPヘッダー**: `Content-Security-Policy`（`default-src 'self'`）・`Strict-Transport-Security`に加え、既存の`X-Content-Type-Options` / `X-Frame-Options` / `Referrer-Policy`を設定しています
- ネットワーク境界（パケットフィルタ・ホストufw・Nginxルーティングの3層）、秘密情報の管理方針（`.tfvars`のgitignore等）を含む棚卸しの詳細は`docs/security-review.md`を参照してください

## 障害試験・負荷試験

- **片系停止試験**: 2インスタンス+Nginxのローカル構成で、稼働中に1台を強制停止しても失敗リクエスト0件（20万件超で検証）
- **DB障害時の挙動**: DB停止中は`/health/ready`が503・`/health/live`は200を維持し、アプリケーションはクラッシュせず、DB復旧後は再起動なしで自動的に正常化することを確認
- **ロールバック演習**: 実際のデプロイスクリプトのロジックで、新バージョンの不具合発覚後に旧バージョンへ戻して復旧できることを確認
- **負荷試験**: `GET /api/missions`で、同時接続40〜50においてコネクションプール枯渇による性能崩壊（105 req/sまで低下）を発見し修正。修正後は同条件で70,000+ req/sを達成
- 詳細・実機との違い・残課題は`docs/load-test-results.md`を参照してください

## 運用手順書と公開判定

18フェーズにわたる本番対応が完了しました(フェーズ19で、公開前の残課題のうちコードで解消できるもの — デプロイ後スモークテスト・証明書の自動更新・アラートルールのコード化 — も実装済み)。実際の運用手順は`docs/operations-runbook.md`(初回構築手順・日常デプロイ・障害対応・定期メンテナンス)、公開可否の判定は`docs/go-live-readiness.md`(判定結果・公開前必須チェックリスト)を参照してください。

**判定結果: 条件付きGO** — コード・インフラ設計は本番公開の準備が整っていますが、実際の公開にはさくらのクラウードアカウントの用意・DNS設定・各種秘密情報の投入など、実在の認証情報を伴う一連の手動作業が必要です(詳細は`docs/go-live-readiness.md`2節)。
