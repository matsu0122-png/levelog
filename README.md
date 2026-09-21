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

ビジネスロジック（`internal/service`）はリポジトリをinterfaceとして扱っており、テストではPostgresを使わないインメモリ実装（`fakes_test.go`）に差し替えてXPの整合性・権限チェック・日付チェックを検証しています。含まれるテスト:

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

### フロントエンド（TypeScript）

```bash
cd frontend
npm install
npx tsc -b        # 型チェック
npm run build      # ビルド確認
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
- レート制限やCSRFトークンなど、より厳格なAPIセキュリティ強化
- Nginxを用いた本番向けリバースプロキシ・HTTPS終端（今回のMVPスコープ外）

## 既知の制約

- 過去日の履歴は、ユーザーがその日にアプリを開いて日次ミッションが生成された日のみ記録されます（深夜バッチ生成は仕様上行わないため）。
- フロントエンドはDocker Compose内では開発用サーバー（`vite --host`）で配信しています。本番相当の静的配信・Nginx等は今回のMVPスコープ外です。
