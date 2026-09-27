# Levelog 本番対応ロードマップ

最終更新: 2026-09-23(フェーズ18完了、全18フェーズ完了)

このドキュメントは、Levelog をさくらのクラウド上で本番運用できるシステムへ段階的に整備していくための進捗管理表です。各フェーズは個別のセッションで実装・検証し、完了ごとに状態を更新します。

状態は次の4種類のいずれかです。

- 未着手
- 進行中
- 完了
- ブロック中

## フェーズ一覧と状態

| # | フェーズ | 状態 |
| --- | --- | --- |
| 1 | 現状調査と本番対応設計 | 完了 |
| 2 | Go APIの本番対応 | 完了 |
| 3 | Frontendの本番対応 | 完了 |
| 4 | 本番用コンテナ構成 | 完了 |
| 5 | テスト整備 | 完了 |
| 6 | staging環境設計 | 完了 |
| 7 | Terraform基盤 | 完了 |
| 8 | ネットワーク構築準備 | 完了 |
| 9 | PostgreSQL本番構成 | 完了 |
| 10 | アプリサーバ構築 | 完了 |
| 11 | ロードバランサ・DNS・HTTPS | 完了 |
| 12 | CI | 完了 |
| 13 | CD | 完了 |
| 14 | 監視・ログ・通知 | 完了 |
| 15 | バックアップと復元 | 完了 |
| 16 | セキュリティ確認 | 完了 |
| 17 | 障害試験・負荷試験 | 完了 |
| 18 | 運用手順書と公開判定 | 完了 |

---

## フェーズ1：現状調査と本番対応設計（完了）

### リポジトリ構成

- Git: 単一コミット（`50da77f Initial commit: build Levelog MVP`）、ブランチは `main` のみ、作業ツリーはクリーン。リモート `origin` あり。
- 構成: `backend/`（Go）、`frontend/`（React+Vite+TS）、ルートに `docker-compose.yml` と `.env.example`。`docs/`・`.github/`・`terraform/` は今回新規作成（存在しなかった）。
- `.env` はリポジトリに存在するがコミット履歴には含まれておらず、`.gitignore` で除外済み。過去コミットにも秘密情報らしきファイル名は見つからなかった。

### 現在の機能

READMEおよびコード確認の結果、以下がMVPとして実装済み。

- ユーザー登録・ログイン・ログアウト（bcryptハッシュ、httpOnly Cookieセッション）
- タイムゾーン保持ユーザー、繰り返しデイリーミッションのCRUD
- 難易度固定XP、当日限定の完了/取り消し、DBトランザクションによる二重付与防止（`xp_transactions` の COMPLETE/UNCOMPLETE ペアリング）
- 直近7日間の履歴、レベル進捗計算（`internal/levelup`）
- バックエンド: `internal/service` に対するユニットテストあり（XP整合性・権限・日付境界を検証）、フェイクリポジトリで実施
- フロントエンド: `tsc -b` の型チェックと `vite build` の本番ビルドは現時点で成功することを確認済み

### 認証方式

- パスワードは `golang.org/x/crypto/bcrypt` でハッシュ化（`internal/service/auth_service.go`）。
- セッションはDBテーブル `sessions`（`token_hash` で照合）で管理しており、**サーバー側にインメモリ状態を持たない**。これはアプリサーバを複数台に増やす上で有利な設計（セッションのアフィニティ不要）。
- Cookie属性（`Secure` / `Domain` / `SameSite=Lax` / `HttpOnly`）はすでに環境変数 `COOKIE_SECURE` / `COOKIE_DOMAIN` で制御可能な設計になっている（`internal/handler/auth_handler.go`）。本番化にあたり値を切り替えるだけで対応できる見込み。

### セッション保存方法

- 上記の通りDBベース。ただしセッションの有効期限切れレコードを削除する仕組み（GC）は現状存在しない（`sessions` テーブルが際限なく増える）。将来的に定期クリーンアップまたは `expires_at` に基づくクエリ側フィルタの確認が必要。

### 環境変数

`.env.example` に定義済みの変数は以下の通りで、ダミー値のみが書かれている（実秘密情報なし）。

- `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` / `POSTGRES_PORT`
- `API_PORT` / `DATABASE_URL` / `FRONTEND_ORIGIN` / `COOKIE_SECURE` / `COOKIE_DOMAIN`
- `FRONTEND_PORT` / `VITE_API_BASE_URL`

不足していると考えられる本番向け環境変数（フェーズ2以降で追加予定）:

- ログレベル（`LOG_LEVEL`）
- HTTPサーバーのタイムアウト値（read/write/idle timeout）
- DBの接続タイムアウトやプール上限の外部化（現状 `internal/db/db.go` にハードコード: `MaxOpenConns=20`, `MaxIdleConns=5`, `ConnMaxLifetime=30分`）
- CORSの複数オリジン対応（staging/production分離用）

### Docker構成

- `docker-compose.yml`: `db`（Postgres 16、ヘルスチェックあり）、`api`（Go）、`web`（Vite dev server）の3サービス。ポートを直接ホストへ公開する構成で、本番のロードバランサ配下・アプリサーバ分離構成とは異なる（開発用構成として妥当）。
- `backend/Dockerfile`: すでにマルチステージビルド・`CGO_ENABLED=0`・非rootユーザー（`appuser`, uid 10001）・`alpine`ベースと、本番レベルに近い品質。ただし `HEALTHCHECK` 命令なし。
- `frontend/Dockerfile`: `npm run dev -- --host 0.0.0.0` で起動しており、**開発用サーバーをそのまま本番相当コンテナとして使っている**。本番用には静的ビルド（`vite build`）成果物をNginx等で配信する構成へ作り直す必要がある（フェーズ3・4の主要課題）。

### DBマイグレーション方式

- `backend/migrations/*.sql` を `embed.go` でバイナリに埋め込み、`cmd/api/main.go` の起動シーケンス内で `db.Migrate()` を毎回呼び出し、`schema_migrations` テーブルを見て未適用分だけ適用。
- **複数アプリサーバ構成での懸念点**: 現在のマイグレーションはアプリ起動のたびに自動実行される。ローリングデプロイで複数のAPIコンテナ/インスタンスがほぼ同時に起動すると、マイグレーション適用処理が競合する可能性がある（DBアドバイザリロック等の排他制御が未実装）。本番では「デプロイパイプライン側で1回だけマイグレーションを実行し、アプリ起動時は適用しない（またはロックを取る）」設計への変更を検討する必要がある（フェーズ13で対応予定）。

### テスト構成

- バックエンド: `go test ./...` は成功（`internal/levelup` と `internal/service` にユニットテストあり）。`internal/handler`・`internal/repository`・`internal/middleware` にはテストなし（HTTP層・DB層・認証ミドルウェアの統合テストが未整備）。`go vet ./...` も成功。
- フロントエンド: `npx tsc -b`（型チェック）、`npm run build`（本番ビルド）ともに成功。ただしテストランナー（Vitest等）は `package.json` に存在せず、フロントエンドの自動テストは未整備。lintは `oxlint` のみ導入済み。

### 複数アプリサーバで動かない要因（調査結果まとめ）

1. **マイグレーション自動実行の競合**: 上記の通り、起動時マイグレーションが複数インスタンス同時起動時に競合しうる。
2. **CORSの単一オリジン前提**: `middleware.CORS` は完全一致の単一オリジンのみ許可する設計。staging/productionや複数ドメインを扱うには拡張が必要（ロードバランサ配下では影響小だが、staging分離時に対応要）。
3. **フロントエンドの配信方式**: 現状は各コンテナが独自のVite dev serverを立てる構成であり、Nginx等でのリバースプロキシ・静的配信・HTTPS終端を前提とした構成になっていない。
4. **ヘルスチェックの粒度不足**: `/api/health` は存在するがDB接続を確認しない単純な生存確認のみ。ロードバランサが「トラフィックを受けてよい状態か（readiness）」と「プロセスが生きているか（liveness）」を区別できない。
5. **サーバータイムアウト未設定**: `http.ListenAndServe` を生で使っており、read/write/idleタイムアウトが未設定。スロー接続やソケット滞留がアプリサーバのリソースを圧迫するリスクがある。
6. **Graceful shutdownなし**: SIGTERM受信時に処理中のリクエストを待たずにプロセスが終了する可能性があり、ローリングデプロイ時に接続中のユーザーへ影響しうる。
7. **セッション/認証自体は複数サーバー対応済み**: DBベースのセッション管理のため、この点はスケールアウトの障害にはならない。

セッション管理とCookie設計はすでに複数サーバー運用を意識した作りになっており、大きな設計変更は不要と判断。一方でHTTPサーバーのライフサイクル管理（起動・停止・タイムアウト）とマイグレーション運用は本番前に必ず対応が必要。

### セキュリティ上の確認

- `.env` はコミットされておらず、Git履歴にも秘密情報は見つからなかった。
- `.env.example` の値はすべてダミー（`levelog`/`levelog` 等の開発用固定値）で、実秘密情報は含まれていない。
- パスワードは bcrypt でハッシュ化済み。平文保存なし。
- 今回のフェーズでは読み取りのみを行い、コード変更・Git操作・クラウド操作は一切実施していない。

### 本番対応ロードマップ

上記の調査結果を踏まえ、フェーズ2以降は当初計画の順序（本メッセージ冒頭のフェーズ一覧）通りに進めるのが妥当と判断した。特に優先度が高いのは以下の3点であり、フェーズ2・3・4で扱う。

1. Go APIのHTTPサーバーライフサイクル対応（timeout, graceful shutdown, health/live/ready分離）とマイグレーション運用の見直し
2. フロントエンドの本番ビルド配信への切り替え（現状dev serverを使っている点の解消）
3. 本番用コンテナ構成の整備（フロントエンドDockerfileの作り直し、HEALTHCHECK追加）

### 残っている課題（次フェーズ以降で対応）

- HTTPサーバーのタイムアウト・graceful shutdown・`/health/live`・`/health/ready` 分離（フェーズ2）
- 構造化ログ・リクエストID（フェーズ2）
- マイグレーション自動実行の排他制御 or デプロイパイプライン分離（フェーズ2/13）
- フロントエンドの本番ビルド配信、認証切れ時のグローバルハンドリング、404ページ、Error Boundary（フェーズ3）
- フロントエンド用Dockerfileの本番化（マルチステージ・Nginx配信・非root）、HEALTHCHECK追加（フェーズ4）
- ハンドラー層・リポジトリ層の統合テスト、フロントエンドのテストランナー導入（フェーズ5）
- CORSの複数オリジン対応（staging/production分離時、フェーズ6）
- Terraform基盤、ネットワーク、DB冗長構成、ロードバランサ、CI/CD、監視、バックアップ、セキュリティ確認、障害試験、運用手順書（フェーズ7〜18）

### 次のフェーズ(フェーズ1時点の予定)

フェーズ2「Go APIの本番対応」に進み、`/health/live` / `/health/ready`、graceful shutdown、HTTPタイムアウト、DB接続タイムアウト、JSON構造化ログ、リクエストID、エラーハンドリング、本番用CORS、安全なCookie設定(値の見直し)、設定値の環境変数化を実装する。

---

## フェーズ2:Go APIの本番対応(完了)

### 調査結果(フェーズ1からの再確認)

- `cmd/api/main.go` は `http.ListenAndServe` を生で呼んでおり、タイムアウト・graceful shutdownが存在しなかった。
- `/api/health` のみでDB接続を確認しない単純な生存確認のみ。live/ready分離なし。
- ログは `log.Printf` の平文で、リクエストIDや構造化フィールドがなかった。
- CORSは完全一致の単一オリジンのみ許可しており、staging/production分離時に拡張が必要だった。
- DBプール設定(`MaxOpenConns`等)やHTTPタイムアウトがコードにハードコードされており、環境変数化されていなかった。

### 実装内容

- **HTTPサーバーのライフサイクル管理**: `cmd/api/main.go` を書き換え、`http.Server` に `ReadHeaderTimeout` / `ReadTimeout` / `WriteTimeout` / `IdleTimeout` を設定。`signal.NotifyContext` でSIGINT/SIGTERMを捕捉し、`server.Shutdown(ctx)` による graceful shutdown を実装(タイムアウトは `SHUTDOWN_TIMEOUT`)。
- **ヘルスチェックの分離**: `GET /health/live`(プロセス生存確認、DBに触れない)と `GET /health/ready`(DBへの `PingContext` による疎通確認、失敗時は503)を追加。既存の `GET /api/health` はREADME/docker-composeとの後方互換のため `/health/live` 相当の挙動のまま維持。
- **リクエストタイムアウト**: `internal/middleware/timeout.go` を新規作成。各リクエストのcontextに `REQUEST_TIMEOUT`(既定10秒)のデッドラインを設定し、リポジトリ層のDB呼び出し(すべて `*Context` 系メソッドでctxを伝播済み)がこのデッドラインを超えると自動的にキャンセルされるようにした。`httpx.WriteError` は `context.DeadlineExceeded` / `context.Canceled` を検出して504を返すよう対応。
- **JSON構造化ログ**: `internal/logging` パッケージを新規作成し、`log/slog` の `JSONHandler` を使用(レベルは `LOG_LEVEL` で制御)。`internal/middleware/logging.go` を書き換え、method・path・status・duration_ms・request_idを含む構造化ログを出力するようにした。
- **リクエストID**: `internal/reqid` パッケージと `internal/middleware/request_id.go` を新規作成。上流プロキシから `X-Request-Id` が来ればそれを再利用し、なければ生成してレスポンスヘッダーとcontextの両方にセット。ログとエラーレスポンスの両方から同じIDで追跡できる。
- **エラーハンドリング**: `httpx.WriteError` のシグネチャを `(w, r, err)` に変更し、5xxエラー・内部エラー・タイムアウトをすべて `request_id` 付きで構造化ログに出力するようにした(既存の「クライアントへは詳細を漏らさない」方針は維持)。ハンドラー・ミドルウェア側の全22箇所の呼び出しを追従。
- **本番用CORS**: `internal/middleware/cors.go` を書き換え、単一オリジンではなく許可オリジンの集合(`map[string]struct{}`)に対して一致確認するよう変更。`FRONTEND_ORIGIN` はカンマ区切りで複数オリジン(staging/production等)を指定できるように `config.Config.FrontendOrigins` を `[]string` 化。credentialed CORSの原則(オリジン完全一致・ワイルドカード不使用)は維持。
- **安全なCookie設定**: 既存の `HttpOnly` / `Secure`(env駆動) / `SameSite=Lax` / `Domain`(env駆動) は本番運用に対して妥当と判断し、値を変更していない(単一オリジンのSPA+同一ドメイン配下API構成を前提とする限りLaxで十分にCSRF対策と両立できるため)。
- **設定値の環境変数化**: `internal/config/config.go` を拡張し、以下を追加(すべて未設定時は安全なデフォルト値を使用): `LOG_LEVEL`, `HTTP_READ_HEADER_TIMEOUT`, `HTTP_READ_TIMEOUT`, `HTTP_WRITE_TIMEOUT`, `HTTP_IDLE_TIMEOUT`, `REQUEST_TIMEOUT`, `SHUTDOWN_TIMEOUT`, `DB_CONNECT_TIMEOUT`, `DB_MAX_OPEN_CONNS`, `DB_MAX_IDLE_CONNS`, `DB_CONN_MAX_LIFETIME`。`internal/db/db.go` の `Open` / `WaitForReady` もこれらの設定値とcontextを受け取るよう変更。
- `.env.example` に新しい環境変数をコメント付きで追記(実値はコミットしていない)。

このフェーズにより、フェーズ1で指摘した「複数アプリサーバで動かない要因」のうち、CORSの単一オリジン制約・タイムアウト未設定・graceful shutdownなし・ヘルスチェック粒度不足の4点を解消した。マイグレーション自動実行の競合(フェーズ1指摘の5点目)は今回のスコープ外で、フェーズ13(CD)でのデプロイパイプライン設計時に対応する。

### 変更ファイル

- `backend/cmd/api/main.go` — 構造化ロガー初期化、graceful shutdown、タイムアウト付き `http.Server`
- `backend/internal/config/config.go` — 新規環境変数の読み込みとデフォルト値、`FrontendOrigins` の複数値対応
- `backend/internal/db/db.go` — `Open` にプール設定を注入可能化、`WaitForReady` をcontext対応に変更
- `backend/internal/handler/router.go` — `/health/live` / `/health/ready` 追加、ミドルウェアの適用順序(RequestID → Logging → Timeout → CORS)
- `backend/internal/httpx/httpx.go` — `WriteError` のシグネチャ変更、タイムアウト検出、request_id付きログ
- `backend/internal/handler/auth_handler.go` / `mission_handler.go` / `daily_mission_handler.go` / `stats_handler.go` — `httpx.WriteError` 呼び出しを新シグネチャに追従(22箇所)
- `backend/internal/middleware/auth.go` — 同上
- `backend/internal/middleware/cors.go` — 複数オリジン対応に書き換え
- `backend/internal/middleware/logging.go` — 構造化ログへ書き換え
- `backend/internal/middleware/request_id.go`(新規) — リクエストID付与ミドルウェア
- `backend/internal/middleware/timeout.go`(新規) — リクエストタイムアウトミドルウェア
- `backend/internal/logging/logging.go`(新規) — `slog` JSONロガーのビルダー
- `backend/internal/reqid/reqid.go`(新規) — リクエストID生成・context伝搬
- `.env.example` — 新規環境変数(タイムアウト・ログレベル・DBプール設定等)の追記(ダミー/コメントのみ)

### 検証結果

- `gofmt -l .` → 差分なし(1回 `gofmt -w` で自動整形した箇所あり)
- `go build ./...` → 成功
- `go vet ./...` → 成功
- `go test ./...` → 成功(既存の `internal/levelup` / `internal/service` テストは全て通過、新規パッケージにはユニットテストなし)
- `docker compose up -d --build db api` でコンテナを起動し、実際のPostgresに対して動作確認:
  - `GET /health/live` → `200 {"status":"ok"}`
  - `GET /health/ready` → `200 {"status":"ok"}`(DB接続確認込み)
  - `GET /api/health`(後方互換) → `200 {"status":"ok"}`
  - 全レスポンスに `X-Request-Id` ヘッダーが付与されることを確認
  - `docker compose logs api` でJSON構造化ログ(`request_id` / `method` / `path` / `status` / `duration_ms`)が出力されることを確認
  - `POST /api/auth/register` → `201` でユーザー登録・セッションCookie発行が従来通り機能することを確認(回帰なし)
  - 未認証での `GET /api/me` → `401` + `request_id` 付きJSONエラーを確認
  - 許可オリジン(`http://localhost:5173`)からのCORSプリフライト(`OPTIONS`)→ `204` + `Access-Control-Allow-Origin` 付与を確認
  - 許可外オリジン(`http://evil.example.com`)では `Access-Control-Allow-Origin` が付与されないことを確認
  - `docker compose stop api`(SIGTERM送信)で `"shutdown signal received"` → `"server stopped"` のログ順に正常終了することを確認(graceful shutdown動作)
  - 検証後 `docker compose down` でコンテナ・ネットワークを削除済み(DBデータボリュームは残置、破壊的操作は未実施)

### セキュリティ上の確認

- 秘密情報は一切ログに出力していない(構造化ログの出力項目はmethod/path/status/duration/request_idのみで、リクエストボディやCookie値は含まない)。
- `.env` の内容は読み取っていない(認証情報保護のためツール側でブロックされ、Docker Compose経由での間接利用のみで検証した)。
- Cookie設定(`HttpOnly` / `Secure` / `SameSite=Lax`)は変更していない。
- 今回のフェーズでもGit commit・push・クラウド操作・破壊的操作は一切実施していない。

### 残っている課題

- マイグレーション自動実行の排他制御 or デプロイパイプライン分離(フェーズ13で対応)
- `sessions` テーブルの期限切れレコードのクリーンアップ機構が未実装(フェーズ2スコープ外、将来的に定期クリーンアップの検討が必要)
- フロントエンドの本番ビルド配信、認証切れ時のグローバルハンドリング、404ページ、Error Boundary(フェーズ3)
- フロントエンド用Dockerfileの本番化、HEALTHCHECK追加(フェーズ4)
- ハンドラー層・リポジトリ層の統合テスト(新規追加した `middleware` / `httpx` / `config` / `db` パッケージにもユニットテストがまだない、フェーズ5)
- Terraform基盤以降の未着手フェーズ(フェーズ6〜18)

### 次のフェーズ(フェーズ2時点の予定)

フェーズ3「Frontendの本番対応」に進み、production build、API URLの環境変数化(既に対応済みのため確認中心)、認証切れ処理、ローディング、APIエラー表示、404ページ、Error Boundary、スマートフォン表示、本番用セキュリティ設定を実装する。

---

## フェーズ3:Frontendの本番対応(完了)

### 調査結果(フェーズ1からの再確認)

- `npm run build`(`tsc -b && vite build`)は既に成功しており、`API_BASE_URL`も`VITE_API_BASE_URL`で環境変数化済み。
- 各ページ(`HomePage` / `MissionsPage` / `HistoryPage` / `MissionFormPage` / `LoginPage` / `RegisterPage` / `SettingsPage`)はすでに`LoadingSpinner`によるローディング表示と`ErrorBanner`によるAPIエラー表示を実装済みで、品質は高かった。
- 一方で以下が未実装だった: (1) 認証切れ(セッション期限切れ)時のグローバルなハンドリングがなく、各ページが個別に「通信エラーが発生しました」と表示するだけだった、(2) 未定義ルートは`<Navigate to="/" replace />`で単にホームへリダイレクトしており、専用の404ページがなかった、(3) React Error Boundaryが存在せず、レンダリング時の例外で白画面になる可能性があった、(4) `vite.config.ts`でsourcemap設定が未指定(デフォルトでは出力されないが明示されていなかった)。
- モバイル表示(`viewport`メタタグ、`env(safe-area-inset-top)`対応、下部ナビゲーション、最大幅レイアウト)は既に実装済みで問題なかった。

### 実装内容

- **認証切れ処理**: `src/api/client.ts`にモジュールレベルの`setUnauthorizedHandler`を追加し、どのAPI呼び出しでも401が返るとハンドラーを起動するようにした。`AuthContext`がマウント時にこのハンドラーを登録し、直前まで`user`が存在していた場合のみ`sessionExpired`フラグを立てて`user`をクリアする(`App.tsx`の`RequireAuth`が`user===null`を見て自動的に`/login`へリダイレクトする既存の仕組みをそのまま利用)。`LoginPage`は`sessionExpired`が真のとき「セッションの有効期限が切れました。再度ログインしてください。」を表示し、ページを離れる際に自動でフラグを消す。
- **404ページ**: `src/pages/NotFoundPage.tsx`を新規作成し、`App.tsx`の catch-all ルートを`<Navigate to="/" replace />`から`<NotFoundPage />`に変更。ホームへ戻るリンク付き。
- **Error Boundary**: `src/components/ErrorBoundary.tsx`を新規作成(クラスコンポーネント、`getDerivedStateFromError` / `componentDidCatch`)。`main.tsx`で`<BrowserRouter>`の外側から全体をラップし、レンダリング時例外を「予期しないエラーが発生しました」+再読み込みボタンの画面に落とすようにした。
- **本番用セキュリティ設定**: `vite.config.ts`に`build.sourcemap: false`を明示し、本番バンドルにソースマップが出力されないことを保証(元々デフォルトでも出力されていなかったが、明示化により設定ドリフトを防止)。`console.*`呼び出しがソース中に存在しないことを確認済み(新規追加の`ErrorBoundary`の`componentDidCatch`のみ、エラーオブジェクトと`componentStack`をログするが機微情報は含まない)。
- **production build / API URLの環境変数化 / ローディング / APIエラー表示 / スマートフォン表示**: いずれも既存実装が妥当と判断し、コード変更なし(検証のみ実施)。

### 変更ファイル

- `frontend/src/api/client.ts` — `setUnauthorizedHandler`の追加、401検知時のフック呼び出し
- `frontend/src/context/AuthContext.tsx` — `sessionExpired`状態の追加、グローバル401ハンドラーの登録
- `frontend/src/pages/LoginPage.tsx` — セッション切れメッセージの表示
- `frontend/src/pages/NotFoundPage.tsx`(新規) — 404ページ
- `frontend/src/App.tsx` — catch-allルートを`NotFoundPage`に変更
- `frontend/src/components/ErrorBoundary.tsx`(新規) — Reactエラーバウンダリ
- `frontend/src/main.tsx` — `ErrorBoundary`でアプリ全体をラップ
- `frontend/vite.config.ts` — `build.sourcemap: false`を明示

### 検証結果

- `npx tsc -b`(型チェック) → 成功
- `npm run lint`(oxlint) → 成功(exit 0)。警告3件はすべて本フェーズで変更していない既存コードのパターン(`HomePage.tsx` / `MissionsPage.tsx`の`useEffect`内`setState`、`AuthContext.tsx`の複数export)で、フェーズ3で新たに発生したものではない。
- `npm run build`(`tsc -b && vite build`) → 成功。`dist/`にソースマップファイル(`*.map`)が生成されないことを確認。
- 実機検証(Docker Composeで`db`+`api`を起動し、`vite preview`でビルド済み本番バンドルを配信して実施。バックエンドのCORS許可オリジンにpreviewのオリジンを一時的に追加):
  - 未認証で`/`にアクセス → `/login`へリダイレクトすることを確認
  - 存在しないパス(`/this-page-does-not-exist`)→ 404ページが表示されることを確認
  - 新規登録 → ホーム画面へ遷移し、レベル・XP・今日のミッション欄が正しく表示されることを確認(回帰なし)
  - DB上のセッションを削除して疑似的にセッション切れを再現し、認証必須ページへの遷移で自動的に`/login`へリダイレクトされ、「セッションの有効期限が切れました。再度ログインしてください。」が表示されることを確認
  - 再ログイン後、モバイル幅(390×844)でホーム画面・ミッション管理画面を確認し、下部ナビゲーション・レイアウトが崩れないことを確認
  - ブラウザコンソールにエラー・警告が一切出力されないことを確認
  - 検証後、DB/APIコンテナは`docker compose down`で削除、`vite preview`プロセスは終了、フロントエンドは通常のデフォルト環境変数で再ビルドして作業ツリーの`dist/`を検証前の状態に戻した(`dist/`自体は`.gitignore`対象でコミット対象外)

### セキュリティ上の確認

- 本番バンドルにソースマップが含まれないことを`vite.config.ts`で明示し、ビルド成果物でも確認した。
- `console.*`による機微情報の出力はなし。
- セッション切れの検証はローカルDocker上の開発用DBのセッションレコードを削除する形で行い、本番DB・実際の秘密情報には一切触れていない。
- 今回のフェーズでもGit commit・push・クラウド操作・破壊的操作(リポジトリに対して)は実施していない。

### 残っている課題

- マイグレーション自動実行の排他制御 or デプロイパイプライン分離(フェーズ13)
- `sessions`テーブルの期限切れレコードのクリーンアップ機構が未実装(将来的に検討)
- フロントエンド用Dockerfileの本番化(現状は`npm run dev`のdevサーバーを使用しており、本番ビルド成果物をNginx等で配信する構成になっていない)、HEALTHCHECK追加(フェーズ4)
- ハンドラー層・リポジトリ層・新規追加のフロントエンドロジック(`AuthContext`の401ハンドリング等)に対する自動テストがまだない(フェーズ5)
- Terraform基盤以降の未着手フェーズ(フェーズ6〜18)

### 次のフェーズ(フェーズ3時点の予定)

フェーズ4「本番用コンテナ構成」に進み、Frontendの本番用Dockerfile(マルチステージビルド、Nginx等での静的配信への切り替え)、Backendの本番用Dockerfileの見直し、非root実行の徹底、本番用Compose構成(PostgreSQLの分離含む)、ヘルスチェック、`.dockerignore`、イメージサイズと安全性の確認を実施する。

---

## フェーズ4:本番用コンテナ構成(完了)

### 調査結果(フェーズ1からの再確認)

- `backend/Dockerfile`は既にマルチステージ・非root(uid 10001)・alpineベースで本番品質に近かったが、`HEALTHCHECK`命令がなかった。
- `frontend/Dockerfile`は`npm run dev -- --host 0.0.0.0`でVite devサーバーをそのまま起動しており、本番用の静的配信(Nginx等)になっていなかった(README記載の既知の制約と一致)。
- `docker-compose.yml`はdb/api/webの3サービスを1ファイルにまとめ、全ポートをホストへ直接公開する開発用構成で、DBをアプリサーバから分離した本番構成にはなっていなかった。
- `backend/.dockerignore`は`*.md`のみ、`frontend/.dockerignore`は`node_modules`/`dist`/`.vite`のみで、`.git`・`.env*`の除外がなかった(実害はなかった。ビルドコンテキストが`backend/`・`frontend/`各ディレクトリ単位のため、リポジトリルートの`.env`はそもそも含まれない)。

### 実装内容

- **Backendの本番用Dockerfile**: `HEALTHCHECK`を追加(`wget --spider http://127.0.0.1:8080/health/live`、30秒間隔)。マルチステージ・非root実行は既存のまま維持。
- **Frontendの本番用Dockerfile**: 完全に書き換え、2段階マルチステージ化。ビルド段階(`node:20-alpine`)で`npm ci && npm run build`を実行し、配信段階は`nginxinc/nginx-unprivileged:1.27-alpine`(非rootで8080番待受するNginx公式イメージ)を使用。`ARG VITE_API_BASE_URL=""`をビルド時に注入可能にし、空(既定)の場合は同一オリジンの`/api/`プロキシ経由になるよう設計。
- **Nginx設定(`frontend/nginx.conf`、新規)**: 静的アセット(`/assets/*`)に`Cache-Control: public, max-age=31536000, immutable`、`index.html`には`no-cache`を明示。`X-Content-Type-Options` / `X-Frame-Options` / `Referrer-Policy`のセキュリティヘッダーを付与。`/api/`・`/health/`はバックエンドコンテナ(`api:8080`)へプロキシし、ブラウザは常に単一オリジンとのみ通信する設計とした(本番ドメイン`levelog.matsu0122.com`配下でのクロスオリジンCookie/CORS問題を回避)。SPAのクライアントサイドルーティング用に`try_files $uri $uri/ /index.html`のフォールバックを設定。IPv4/IPv6両方でlisten(後述の不具合対応)。
- **本番用Compose(`docker-compose.prod.yml`、新規)**: `db`を含まない`api`+`web`のみの構成。`DATABASE_URL` / `FRONTEND_ORIGIN`は`${VAR:?...}`構文で未設定時にエラー終了するようにし、実運用で本番DB接続先の指定漏れを防止。`api`は`ports: []` + `expose: ["8080"]`でホストに公開せず、`web`(Nginx)のみを`WEB_PORT`(既定8080)で公開。`web`は`depends_on: api: condition: service_healthy`でAPIのヘルスチェック完了を待ってから起動。
- **`.dockerignore`の強化**: `backend/.dockerignore` / `frontend/.dockerignore`双方に`.git` / `.env` / `.env.*`を追加(現状害はないが、将来的な取り違え防止の防御的措置)。`backend/.dockerignore`には`*_test.go`も追加し、本番ビルドコンテキストを最小化。
- **README更新**: 「Nginxは今回のMVPスコープ外」という記述が本フェーズで実態と矛盾するようになったため、「本番用コンテナ構成」節に置き換えて`docker-compose.prod.yml`の使い方・設計意図を追記。

### 変更ファイル

- `backend/Dockerfile` — `HEALTHCHECK`追加
- `backend/.dockerignore` — `.git` / `.env*` / `*_test.go`除外を追加
- `frontend/Dockerfile` — マルチステージ化、Nginx配信への全面書き換え
- `frontend/nginx.conf`(新規) — 本番用Nginx設定(同一オリジンプロキシ、キャッシュ、セキュリティヘッダー)
- `frontend/.dockerignore` — `.git` / `.env*`除外を追加
- `docker-compose.prod.yml`(新規) — db非同梱の本番用トポロジー(api非公開 + web公開)
- `README.md` — 本番用コンテナ構成の使い方を追記、スコープ外記述を削除

### 検証結果

- `docker build ./backend`・`docker build ./frontend` → 両方成功。イメージサイズ: backend 23.6MB、frontend(Nginx込み) 50.1MB
- 非root実行確認: backendコンテナは`uid=10001(appuser)`、frontendコンテナは`uid=101(nginx)`で動作することを確認
- 初回検証でDockerのヘルスチェックが「starting」から進まない不具合を発見。原因調査の結果、コンテナ内で`localhost`が`::1`(IPv6)に解決される一方、Nginxは`listen 8080;`のみでIPv4にしかbindしておらず接続拒否されていたことが判明。`nginx.conf`に`listen [::]:8080;`を追加し、両Dockerfileのヘルスチェックを`localhost`から`127.0.0.1`明示指定に変更して解消(修正後、3コンテナとも`healthy`になることを確認)
- **2つ目の不具合**: ローカル検証のため`docker compose -f docker-compose.yml -f docker-compose.prod.yml up`で2ファイルをマージして実行したところ、(1) リポジトリルートの`.env`にある開発用`VITE_API_BASE_URL`(`http://localhost:8090`)が本番ビルドに意図せず焼き込まれ、Nginx経由ではなくAPIへ直接(別ポート)アクセスしていたこと、(2) `docker-compose.yml`側の`api`ポート公開設定がマージによって`docker-compose.prod.yml`側にも引き継がれ、`api`が意図せずホストへ公開されていたことを発見。調査の結果、Docker Composeは`ports`等の一部のリスト型フィールドを「置換」ではなく「マージ(和集合)」する仕様であり、オーバーライド側で`ports: []`と書いても基底ファイルの公開設定を打ち消せないことが判明。対応として、(a) ローカル検証手順を「2ファイルをマージして起動」から「`db`は`docker-compose.yml`単体で、`api`/`web`は`docker-compose.prod.yml`単体で(同一プロジェクト名により同一ネットワークを共有)別々に起動する」方式に変更、(b) 検証時は`VITE_API_BASE_URL=`を明示的に空指定するよう`docker-compose.prod.yml`のコメントを修正。修正後、`docker port levelog-api-1`が空(公開ポートなし)であること、フロントエンドのJSバンドルに`http://localhost:*`のような絶対URLが含まれないこと、ブラウザの全APIリクエストが`web`のポート(Nginx)のみを経由し、Nginxのアクセスログにも`/api/*`が記録されることを実機確認
- 実機検証(修正後、`db`は`docker-compose.yml`単体・`api`/`web`は`docker-compose.prod.yml`単体で起動): 新規登録・ミッション作成(2件)・ミッション一覧表示がすべてNginx経由の単一オリジンで正常動作することをブラウザで確認。同一オリジンのため、ブラウザはCORSプリフライト(OPTIONS)を一切発行しないことを確認(フェーズ2のCORS実装が正しく迂回される設計であることの裏付け)
- 静的アセットに`Cache-Control: public, max-age=31536000, immutable`、`index.html`に`no-cache`が付与されることを確認
- 検証後、全コンテナ・ネットワーク・テスト用イメージ(`levelog-backend-prod:test` / `levelog-frontend-prod:test`)を削除済み。`docker-compose`が生成したビルドキャッシュイメージ(`levelog-api` / `levelog-web`、計約74MB)はローカルのDockerキャッシュとして残置(リポジトリ管理外、削除不要と判断)
- バックエンド: `gofmt -l .`(差分なし)・`go build ./...`・`go vet ./...`・`go test ./...` すべて成功(回帰なし)
- フロントエンド: `npx tsc -b`・`npm run build` すべて成功(回帰なし)

### セキュリティ上の確認

- backend/frontendとも非rootユーザーでコンテナが実行されることを確認済み。
- 本番用Composeでは`api`がホストに一切公開されず、外部から直接到達できないことを実機確認。
- `.env` / `.git`をビルドコンテキストから除外する`.dockerignore`を整備(現状実害はないが将来の事故防止)。
- Nginxに`server_tokens off`を設定し、バージョン情報の露出を抑制。`X-Content-Type-Options` / `X-Frame-Options` / `Referrer-Policy`を付与。
- 検証で使用したDBは全てローカルDocker上の開発用データ(`levelog_db_data`ボリューム)。本番の秘密情報・実データには一切触れていない。
- Git commit・push・クラウド操作・破壊的操作は今回も実施していない。

### 残っている課題

- マイグレーション自動実行の排他制御 or デプロイパイプライン分離(フェーズ13)
- `sessions`テーブルの期限切れレコードのクリーンアップ機構が未実装(将来的に検討)
- HTTPS終端は本フェーズのスコープ外(コンテナ内NginxはHTTP 8080のみ)。TLS証明書管理はフェーズ11(ロードバランサ・DNS・HTTPS)で対応
- 新規追加した`nginx.conf` / `docker-compose.prod.yml`に対する自動テスト(CI上でのビルド・起動確認等)がまだない(フェーズ5・12で検討)
- Terraform基盤以降の未着手フェーズ(フェーズ6〜18)

### 次のフェーズ

フェーズ5「テスト整備」に進み、Goユニットテスト(既存に加えハンドラー層・リポジトリ層)、API統合テスト、XP二重付与防止テスト(既存を確認)、認証・認可テスト、Frontendの型チェック(既存)、Frontendのテスト(テストランナー導入)、production buildテストを実施する。

---

## フェーズ5:テスト整備(完了)

### 調査結果(フェーズ1からの再確認)

- バックエンド: `internal/levelup`・`internal/service`にはフェイクリポジトリを使ったユニットテストが既に存在し、XP整合性・権限・日付境界を検証済みだった。一方、`internal/handler`(HTTPハンドラー)・`internal/middleware`・`internal/httpx`・`internal/config`・`internal/db`・`internal/repository`にはテストが一切なかった。
- フロントエンド: `package.json`にテストランナー(Vitest等)が存在せず、`tsc -b`による型チェックと`vite build`のみが検証手段だった。
- XP二重付与防止は`internal/service`のフェイクリポジトリテストで検証済みだが、実際の排他制御(`SELECT ... FOR UPDATE`、`internal/repository/daily_mission_repo.go`)はインメモリのフェイクでは検証できず、実DBでの並行リクエストに対する堅牢性は未検証だった。

### 実装内容

- **Go統合テスト(新規、`backend/internal/handler/router_integration_test.go`)**: ルーター全体(ミドルウェア+ハンドラー+サービス+リポジトリ)を実際のPostgresに対して`httptest.Server`経由で検証。`TEST_DATABASE_URL`環境変数が未設定の場合は全テストが`t.Skip`されるため、通常の`go test ./...`には一切影響しない設計。内容: ヘルスチェック3種、未知ルートの404、認証フロー(未認証拒否・登録直後のログイン状態・誤パスワード拒否・ログアウト後の失効)、ミッションのライフサイクル(作成→今日のミッション生成→完了でXP付与→二重完了で追加付与なし→取り消しでXP相殺→再取り消しでも変化なし)、**実DB・15並行リクエストによるXP二重付与防止テスト**(`SELECT ... FOR UPDATE`の行ロックが実際に機能し、最終XPが単発分のみになることを検証。フェイクリポジトリのテストでは検証不可能な領域)、クロスユーザー認可(他ユーザーのテンプレート一覧非表示・編集/削除/完了がすべて404になることを検証)。
- **Go単体テスト(新規)**: `internal/config`(env変数の読み込み・デフォルト値・不正値でのエラー7ケース)、`internal/httpx`(JSON応答・apperrorへの変換・タイムアウトの504変換・一般エラーの詳細非露出・JSONデコードの妥当性検証10ケース)、`internal/middleware`(CORS許可/非許可オリジン・プリフライト4ケース、リクエストID生成/再利用/一意性3ケース、タイムアウトのcontext deadline設定/キャンセル2ケース、構造化ログの出力内容とフィールド漏れ検査2ケース)。
- **Frontendテストランナー導入**: `vitest` / `@testing-library/react` / `@testing-library/jest-dom` / `jsdom`を追加。`vite.config.ts`を`vitest/config`の`defineConfig`に切り替え、`test`ブロック(jsdom環境、セットアップファイル)を追加。`package.json`に`test` / `test:watch`スクリプトを追加。`tsconfig.app.json`の`types`に`@testing-library/jest-dom`を追加し、`expect(...).toBeInTheDocument()`等の型を認識できるようにした。
- **Frontendテスト(新規)**: `api/client.test.ts`(fetchをモックし、成功時のデコード・credentials付与・エラーメッセージ抽出・**401時のグローバルunauthorizedHandler呼び出し**・204応答の扱い・リクエストボディのContent-Type付与を検証、10ケース)、`context/AuthContext.test.tsx`(fetchをモックしてclient.ts/endpoints.tsは実物のまま使用し、初回セッションチェック・ログイン・**セッション切れ時の自動ログアウト+`sessionExpired`フラグ**・ログアウト済み状態での401到達時に誤ってフラグが立たないこと・ログアウト・`dismissSessionExpired`を検証、7ケース、フェーズ3で実装した認証切れ処理の直接的な回帰テスト)、`components/ErrorBoundary.test.tsx`(正常時は子要素をそのまま描画、例外発生時はフォールバックUIを描画、2ケース)、`pages/NotFoundPage.test.tsx`(404メッセージとホームへのリンクを検証、1ケース)。
- **README更新**: バックエンドの統合テストの実行方法(`TEST_DATABASE_URL`)とテスト内容、フロントエンドの`npm run test`をテスト方法セクションに追記。

### 変更ファイル

- `backend/internal/handler/router_integration_test.go`(新規) — API統合テスト(認証・ミッション・XP・認可・並行性)
- `backend/internal/config/config_test.go`(新規) — 設定読み込みのユニットテスト
- `backend/internal/httpx/httpx_test.go`(新規) — JSON応答・エラー変換のユニットテスト
- `backend/internal/middleware/cors_test.go` / `request_id_test.go` / `timeout_test.go` / `logging_test.go`(いずれも新規) — 各ミドルウェアのユニットテスト
- `frontend/package.json` / `package-lock.json` — `vitest`等のdevDependencies追加、`test` / `test:watch`スクリプト追加
- `frontend/vite.config.ts` — `vitest/config`への切り替え、`test`ブロック追加
- `frontend/tsconfig.app.json` — `@testing-library/jest-dom`型の追加
- `frontend/src/test/setup.ts`(新規) — Vitestセットアップ(jest-domマッチャー登録)
- `frontend/src/api/client.test.ts` / `frontend/src/context/AuthContext.test.tsx` / `frontend/src/components/ErrorBoundary.test.tsx` / `frontend/src/pages/NotFoundPage.test.tsx`(いずれも新規) — フロントエンドテスト
- `README.md` — テスト方法セクションに統合テスト・Vitestの実行方法を追記

### 検証結果

- バックエンド: `gofmt -l .`(差分なし)・`go build ./...`・`go vet ./...`・`go test ./...`・`go test ./... -race` すべて成功(既存8ケース+新規32ケース=計40ケース)
- バックエンド統合テスト: ホストのネイティブPostgres(既存プロセス、ポート5432)とDockerのポートマッピングが衝突する環境だったため、`POSTGRES_PORT=15432`で開発用DBを起動し`TEST_DATABASE_URL`をそのポートへ向けて実行。`go test ./internal/handler/... -v -race`で全6テスト成功。並行XP二重付与防止テストは3回連続実行(`-race`付き)しても安定して成功し、データ競合も検出されなかった
- フロントエンド: `npx tsc -b`・`npm run lint`(exit 0、警告は既存3件のみ)・`npm run test`(4ファイル20ケース全成功)・`npm run build` すべて成功。ビルド成果物にテストファイルが含まれないこと、バンドルのハッシュが変化していないこと(テスト追加が本番バンドルに一切影響しないこと)を確認
- `docker build ./frontend`で本番Dockerイメージが新規devDependencies追加後も問題なくビルドできることを確認(devDependenciesは`npm ci`時にインストールされるが最終イメージ`nginx`ステージには含まれない)
- 検証に使用したDockerコンテナ・イメージはすべて削除済み

### セキュリティ上の確認

- テストコードに実際の秘密情報は含まれない(統合テストの`TEST_DATABASE_URL`はローカル開発用のダミー認証情報をドキュメント内で例示しているのみ)。
- 統合テストで生成されるテストユーザーのメールアドレスは`it-<timestamp>-<pid>@example.com`形式で自動生成され、実在のメールアドレスは使用していない。
- Git commit・push・クラウド操作・破壊的操作は今回も実施していない。

### 残っている課題

- マイグレーション自動実行の排他制御 or デプロイパイプライン分離(フェーズ13)
- `sessions`テーブルの期限切れレコードのクリーンアップ機構が未実装(将来的に検討)
- `internal/repository`層の直接的な単体テスト(現状は統合テスト経由でカバー)、`internal/db`のマイグレーション処理自体のテストは未整備
- 統合テストはCI環境での自動実行がまだ設定されていない(フェーズ12で`TEST_DATABASE_URL`を用いたPostgresサービスコンテナとして組み込む想定)
- Terraform基盤以降の未着手フェーズ(フェーズ6〜18)

### 次のフェーズ

フェーズ6「staging環境設計」に進み、`staging.levelog.matsu0122.com`の設計、staging/production間の設定分離方針、DB分離方針、Cookie分離方針、Terraformディレクトリ設計、構成図、推定リソース一覧を作成する。

---

## フェーズ6:staging環境設計(完了)

本フェーズは設計フェーズであり、クラウドリソースの作成・Terraform実行・DNS変更・コード変更は一切行っていない。成果物は新規ドキュメント`docs/staging-environment.md`。

### 調査結果

- 現状のコードベース(フェーズ1〜5)は、`FRONTEND_ORIGIN`のカンマ区切り複数オリジン対応(フェーズ2)や設定値の環境変数化により、staging/production分離を前提とした設計に既に対応できる状態だった。追加のコード変更は不要と判断。
- Cookie設定(`COOKIE_DOMAIN`)を素朴に「環境ごとに明示的なサブドメインを設定する」方式にすると、誤って親ドメイン`matsu0122.com`を設定した場合にstaging/production間でセッションCookieが共有されてしまう事故リスクがあることに気づいた。既存実装(`COOKIE_DOMAIN`空欄でhost-onlyクッキーにする)をそのまま両環境で使うほうが安全という設計上の結論に至った(詳細はドキュメント4節)。

### 実装内容

`docs/staging-environment.md`を新規作成し、以下を定義した。

1. **ドメイン設計**: production=`levelog.matsu0122.com`、staging=`staging.levelog.matsu0122.com`
2. **環境分離の基本方針**: アプリサーバ・DB・LB・Terraform stateを環境ごとに完全独立させる方針とその理由
3. **設定分離**: 環境変数ごとのstaging/production値の対応表
4. **Cookie分離**: `COOKIE_DOMAIN`は両環境とも空欄のままとし、host-onlyクッキーによる自動的な分離に頼る方針(親ドメイン設定の禁止を明文化)
5. **DB分離**: 環境ごとに独立したPostgresアプライアンスを用意し、production実データのstagingへの複製は匿名化必須とする方針
6. **Terraformディレクトリ設計**: `modules/`(環境非依存)と`environments/{staging,production}/`(独立state)に分けるレイアウトを設計
7. **構成図**: production・staging双方の概念構成図をMermaidで作成
8. **推定リソース一覧**: スペック目安・台数の暫定表(具体的な金額は記載せず、公式サイトでの見積りを促す注記を追加)
9. **未確定事項の一覧**: state backend方式・staging用LBの要否・DB冗長方式など、後続フェーズで決定すべき事項を明記

### 変更ファイル

- `docs/staging-environment.md`(新規) — staging環境設計書(ドメイン・分離方針・Terraformディレクトリ設計・構成図・リソース一覧)

### 検証結果

コード変更を伴わないフェーズのため、既存の検証コマンドを再実行して回帰がないことのみ確認した。

- バックエンド: `go build ./...` / `go vet ./...` / `go test ./...` すべて成功
- フロントエンド: `npx tsc -b` / `npm run test`(20ケース) / `npm run build` すべて成功

### セキュリティ上の確認

- 本フェーズで作成したドキュメントに実際の秘密情報・実在のドメイン以外の情報は含まれていない(`matsu0122.com`はユーザー指定のドメイン予定として最初の指示に記載されていたものをそのまま使用)。
- Cookie分離の設計において、誤設定(親ドメインへの`COOKIE_DOMAIN`設定)によるセッション漏洩リスクを明文化し、将来の実装・運用時の事故を予防する記述を追加した。
- Git commit・push・クラウド操作・破壊的操作は実施していない。

### 残っている課題

- Terraform state backendの具体的な選定(フェーズ7)
- staging用ロードバランサの要否の最終判断(フェーズ17まで保留)
- DB冗長構成の具体的な方式(フェーズ9)
- 正確なさくらのクラウードのリソース価格の確認(フェーズ7着手前に別途実施)

### 次のフェーズ

フェーズ7「Terraform基盤」に進み、本フェーズで設計した`terraform/`ディレクトリ構成を実際に作成し、provider設定・variables・outputs・staging/production分離・network/app server/database/load balancer/monitoringの各モジュールの雛形・State管理方針を実装する。`terraform fmt` / `terraform validate` / `terraform plan`までを実施し、`terraform apply`は行わない。

---

## フェーズ7:Terraform基盤(完了)

`terraform apply` / `terraform destroy`、さくらのクラウードの有料リソース作成は一切実施していない。実行したのは`terraform fmt` / `terraform validate` / `terraform plan`のみで、後述の通り`plan`は認証情報未設定により意図通り失敗する(=実リソースには一切到達しない)ことを確認している。

### 調査結果

- フェーズ6で設計した`terraform/modules/*` + `terraform/environments/{staging,production}/`のディレクトリ構成をそのまま採用できると判断した。
- さくらのクラウードのTerraformプロバイダ(`sacloud/sakuracloud`)の実際のリソーススキーマは事前知識だけでは正確性に自信が持てなかったため、一時的な検証用ディレクトリで`terraform init`してプロバイダを取得し、`terraform providers schema -json`で実スキーマを取得してから各モジュールを実装した(推測でコードを書いて後で気づかず誤るリスクを避けるため)。この過程で当初の設計から2点修正が必要と判明した: (1) `sakuracloud_database`の`network_interface`ブロックには`packet_filter_id`が存在せず、代わりに`source_ranges`(CIDRリスト)でアクセス制御する設計だった、(2) SSH鍵やパスワード無効化はサーバー本体の`disk_edit_parameter`ブロックで行う(ディスク側ではない)。
- ロードバランサ用の「ルーティング済み(公開)スイッチ」は、アプリサーバに使う`upstream = "shared"`の簡易公開接続とは別物で、さくらのクラウード側のルータ+スイッチ(Internetリソース)の作成が別途必要になる。この具体的な構築はフェーズ11(ロードバランサ・DNS・HTTPS)のスコープであり、本フェーズでは`public_switch_id`等を必須変数(デフォルト値なし)として持たせるに留めた。

### 実装内容

- **provider設定**: 各environment(`versions.tf`)に`required_providers`(`sacloud/sakuracloud ~> 2.25`)と`provider "sakuracloud" { zone = var.sakura_zone }`を設定。認証情報(`SAKURACLOUD_ACCESS_TOKEN` / `SAKURACLOUD_ACCESS_TOKEN_SECRET`)は環境変数から読む設計とし、コード・`.tfvars`には一切書いていない。各モジュールにも`required_providers`を追加(モジュール単体でプロバイダ解決するために必要と判明したため)。
- **network module**: 内部スイッチ(`sakuracloud_switch`、インターネット接続なし)+ アプリサーバ公開NIC用の最小パケットフィルタ(現状ICMPのみ、詳細ルールは今後追加)。内部ネットワークのCIDR・ゲートウェイ・ネットマスクを`cidrhost`/`split`で算出して出力。
- **app_server module**: OSアーカイブの`data`参照、SSH公開鍵(`sakuracloud_ssh_key`)、ブートディスク、サーバー本体(`shared`の公開NIC + 内部スイッチNICの2枚構成、`disk_edit_parameter`でパスワード認証無効化+SSH鍵設定)。`server_count`で台数を可変にし、staging=1台・production=2台をenvironment側から指定。
- **database module**: `sakuracloud_database`(PostgreSQL)。内部スイッチのみに接続し、`source_ranges`で内部CIDR以外からのアクセスを拒否。パスワードは`sensitive = true`かつデフォルト値なしの変数で、`.tfvars`にコミットしない設計を徹底。
- **load_balancer module**: `sakuracloud_load_balancer`。VIP・バックエンドサーバー(動的ブロックでアプリサーバのIPリストを展開)・ヘルスチェックパス(`/health/ready`、フェーズ2で実装したreadinessエンドポイントをそのまま利用)。
- **monitoring module**: `sakuracloud_simple_monitor`による外形監視(`/health/live`をチェック、フェーズ2のlivenessエンドポイントを利用)。Slack通知先はsensitive変数でデフォルト空。
- **staging/production分離**: `environments/staging`(LBなし、DNSから直接アプリサーバ1台。db plan=10g、app server 1vCPU/2GB)と`environments/production`(LB+アプリサーバ2台、db plan=30g、app server 2vCPU/4GB)を別々のroot moduleとして作成。それぞれ独立したstate(現状ローカルbackend)を持つ。
- **State管理方針**: 現時点は両環境ともローカルbackend(暫定)。リモートbackend(さくらのクラウードのS3互換オブジェクトストレージ)への移行手順を`versions.tf`内にコメントアウトした設定例として残し、`terraform/README.md`に移行手順を明文化した。stateファイルは`.gitignore`で除外(`.terraform.lock.hcl`はTerraformの推奨に従いコミット対象のまま)。
- `terraform/README.md`(新規): ディレクトリ構成、認証情報の扱い、実行方法、State管理方針、既知の未確定事項(OSイメージ名・プラン名・公開スイッチ)を記載。

### 変更ファイル

- `terraform/modules/{network,app_server,database,load_balancer,monitoring}/{main,variables,outputs,versions}.tf`(新規、計20ファイル) — 各モジュール実装
- `terraform/environments/{staging,production}/{main,variables,outputs,versions}.tf`(新規、計8ファイル) — 環境別ルートモジュール
- `terraform/environments/{staging,production}/terraform.tfvars`(新規) — 非秘密の環境固有値のみ
- `terraform/environments/{staging,production}/.terraform.lock.hcl`(新規、`terraform init`による自動生成、コミット対象) — プロバイダバージョン固定
- `terraform/.gitignore`(新規) — `.terraform/`・`*.tfstate*`等を除外
- `terraform/README.md`(新規) — 運用手順書

### 検証結果

- `terraform fmt -check -recursive`(`terraform/`直下から実行) → 差分なし(実装中に1回`terraform fmt -recursive`で自動整形)
- `terraform init`(staging/production両方) → 成功。実際のプロバイダ(`sacloud/sakuracloud` v2.36.1)を取得
- `terraform validate`(staging/production両方) → **`Success! The configuration is valid.`**(実プロバイダの実スキーマに対する検証であり、フィールド名・型の誤りがあればここで検出される)
- `terraform plan`:
  - 必須変数(`ssh_public_key` / `db_password`、productionはLB関連変数も)を未指定のまま実行 → 「値が未設定」エラーで正しく停止することを確認(秘密情報や未確定値なしでは`plan`が絶対に先へ進まないことの確認)
  - ダミー値(実際の鍵・パスワードではない、明らかにプレースホルダーと分かる値)をすべての必須変数に`-var`で与えて再実行 → staging/production両方とも`AccessToken is required` / `AccessTokenSecret is required`でエラー終了することを確認。これは本セッションに実際のさくらのクラウード認証情報が(意図的に)存在しないためであり、**実クラウドAPIへの到達・リソース作成が一切発生していないことの直接的な証拠**となる
  - 上記いずれの実行でも`.tfstate`ファイルは生成されていないことを確認済み
- バックエンド(`go build/vet/test`)・フロントエンド(`tsc -b`/`npm run test`)は本フェーズでコード変更していないため再実行のみ、すべて成功

### セキュリティ上の確認

- 実際のさくらのクラウード認証情報・DBパスワード・Slack Webhook URLはコード・`.tfvars`・ドキュメントのいずれにも記載していない。`terraform plan`検証時に使用したダミー値も明らかにプレースホルダーと分かる文字列(`placeholder-not-a-real-password`等)のみ。
- 秘密情報を要求する変数(`db_password`、`monitor_slack_webhook`)はすべて`sensitive = true`を付与し、かつデフォルト値を持たない設計とした(意図せずダミー値のまま`apply`されることを防ぐ)。
- `.gitignore`で`*.tfstate*`を除外し、将来ローカルbackendの状態ファイルが誤ってコミットされることを防止。
- `terraform apply` / `terraform destroy`、さくらのクラウードの有料リソース作成、Git commit・push、破壊的操作は一切実施していない。

### 残っている課題

- Terraform stateのリモートbackendへの実際の移行(バケットのbootstrap作業含む)
- ロードバランサ用の公開スイッチ(Internetリソース)の構築(フェーズ11)
- OSイメージ名(`os_type`)・DB/LBのプラン名など、さくらのクラウードAPI側の最新値の確認(初回`apply`前に必須)
- ネットワークの詳細ルール(SSH接続制限含むパケットフィルタの本実装、フェーズ8)
- DB冗長構成の詳細(フェーズ9)
- 実際の`terraform apply`は、正式な認証情報が用意され、ユーザーから明示的な許可を得るまで実施しない

### 次のフェーズ

フェーズ8「ネットワーク構築準備」に進み、内部スイッチ・パケットフィルタの本実装(SSH接続制限含む)、公開・非公開通信の分離、管理接続方式、AppからDBへの通信ルール、ネットワーク構成図を整備する。

---

## フェーズ8:ネットワーク構築準備(完了)

`terraform apply`・さくらのクラウードの有料リソース作成は本フェーズでも実施していない。

### 調査結果

- フェーズ7時点の`sakuracloud_packet_filter.app_public`はICMPのみを許可するプレースホルダーであり、SSH・HTTP・HTTPSの実際の許可ルールが未実装だった。
- productionのアプリサーバは、ロードバランサ経由ではなく直接インターネットから80/443へ到達できてしまう設計のままだった(LBのヘルスチェックに基づくルーティングをバイパスして直接アクセスできてしまう状態)。
- アプリサーバの内部NIC(DB接続用)への静的IP割り当ての挙動(2枚あるNICのどちらに`disk_edit_parameter`のIP設定が適用されるか)は、フェーズ7時点から未検証のままであり、本フェーズでも実環境なしに断定することを避けた(誤った断定は、意図せずアプリサーバの公開NICに内部IPを設定してしまう等、深刻な事故につながりうるため)。

### 実装内容

- **SSH接続制限**: `terraform/modules/network`に`admin_ssh_cidrs`変数(デフォルトなし、空リストも`validation`ブロックで拒否)を追加し、パケットフィルタにTCP 22の許可ルールを`dynamic`ブロックで生成するようにした。
- **公開・非公開通信の分離**: パケットフィルタをSSH(admin_ssh_cidrsのみ)・HTTP/HTTPS(web_allowed_source_cidrsのみ、既定`0.0.0.0/0`)・ICMPの明示的な許可リストへ置き換え、それ以外は暗黙の最終拒否で遮断する構成にした。production環境のみ、`web_allowed_source_cidrs`をロードバランサの実IP(`var.lb_ip_addresses`、/32ずつ)に絞り込み、アプリサーバへの直接公開アクセスを事実上遮断する設定を追加した(staging環境はロードバランサがないため既定の`0.0.0.0/0`のまま)。
- **管理接続方式**: SSH(鍵認証のみ、パスワード認証は既にフェーズ7で無効化済み)+IP許可リストという方式を採用し、現規模(最大2台)では踏み台サーバーを導入しない方針を`docs/network-design.md`に明文化した。
- **AppからDBへの通信ルール**: フェーズ7で実装済みの`source_ranges = [internal_cidr]`(サブネット単位の制限)を、意図的な設計判断として明文化した。アプリサーバ個々のIP単位への絞り込みは、静的内部アドレッシングの挙動を実環境で検証できるタイミング(フェーズ10近辺)まで見送ることを記録した。
- **ネットワーク構成図**: `docs/network-design.md`に、公開/非公開分離の概念図と、production完成形の詳細構成図(ポート・許可ルールレベル)をMermaidで追加。
- `terraform/environments/{staging,production}`に`admin_ssh_cidrs`変数を追加し、network moduleへ配線。

### 変更ファイル

- `terraform/modules/network/variables.tf` — `admin_ssh_cidrs`(必須)・`web_allowed_source_cidrs`(既定`0.0.0.0/0`)を追加
- `terraform/modules/network/main.tf` — パケットフィルタをSSH/HTTP/HTTPS/ICMPの明示的ルールに置き換え
- `terraform/environments/{staging,production}/variables.tf` — `admin_ssh_cidrs`変数を追加
- `terraform/environments/{staging,production}/main.tf` — network moduleへ`admin_ssh_cidrs`を配線。productionは`web_allowed_source_cidrs`もLBのIPへ絞り込み
- `terraform/environments/{staging,production}/terraform.tfvars` — コメント更新(admin_ssh_cidrsも非秘密だが非コミットである旨を明記)
- `docs/network-design.md`(新規) — ネットワーク設計書(公開/非公開分離・SSH制限・管理接続方式・App-DB通信ルール・構成図)

### 検証結果

- `terraform fmt -check -recursive` → 差分なし
- `terraform validate`(staging/production両方) → 成功(実プロバイダの実スキーマに対する検証)
- `terraform plan`:
  - `admin_ssh_cidrs`を未指定のまま実行 → 「値が未設定」エラーで正しく停止することを確認(SSH許可リストが空・ワイルドカードのまま`apply`されうる余地がないことの確認)
  - ダミー値をすべての必須変数に与えて再実行(staging/production両方) → フェーズ7と同様、`AccessToken is required`で意図通り停止することを確認。production側では`web_allowed_source_cidrs`の`lb_ip_addresses`からの導出(`[for ip in var.lb_ip_addresses : "${ip}/32"]`)を含むローカル評価が正常に完了した上で、実クラウードAPIには到達していないことを確認
  - 実行後も`.tfstate`ファイルは生成されていないことを確認
- バックエンド(`go build/vet/test`)・フロントエンド(`tsc -b`/`npm run test`)は本フェーズでコード変更していないため再実行のみ、すべて成功

### セキュリティ上の確認

- `admin_ssh_cidrs`にデフォルト値を設定しなかったことで、「うっかり空リストや`0.0.0.0/0`のままSSHが全世界に公開される」事故を構造的に防いでいる(`validation`ブロックで空リストも明示的に拒否)。
- 実際の管理者IPアドレスはコード・ドキュメントのいずれにも記載していない(`terraform.tfvars`にも値を入れていない)。
- production環境のアプリサーバは、ロードバランサの実IP以外からの直接アクセスができない設計にした(design上の改善。実際の効果はフェーズ11でLBの実IPが確定し`apply`されて初めて発揮される)。
- `terraform apply`・さくらのクラウードの有料リソース作成、Git commit・push、破壊的操作は一切実施していない。

### 残っている課題

- アプリサーバ内部NICへの静的IP割り当てとDB `source_ranges`の単一ホスト単位への絞り込み(実環境検証が前提、フェーズ10近辺)
- 踏み台サーバー導入要否の再評価(運用開始後)
- デプロイ用ユーザーの具体的な権限設計(フェーズ10)
- ロードバランサ用の公開スイッチ構築・TLS終端の確定(フェーズ11)
- Terraform stateのリモートbackend移行(フェーズ7から継続)

### 次のフェーズ

フェーズ9「PostgreSQL本番構成」に進み、DBアプライアンスの冗長構成、アプリ用ユーザー/マイグレーション用ユーザーの分離、TLS、バックアップ、PITR、監視、復元手順を設計・実装する。

---

## フェーズ9:PostgreSQL本番構成(完了)

`terraform apply`・さくらのクラウードの有料リソース作成・実DBへの変更は本フェーズでも実施していない。

### 調査結果

- フェーズ7時点の`modules/database`は、DBアプライアンスの「デフォルトユーザー」1つ(`username`/`password`)のみを設定しており、これをアプリケーションがそのまま使う設計だった。マイグレーション実行とアプリ実行時接続が同一の(強い権限を持つ)ユーザーを共有しており、最小権限の原則に反する状態だった。
- 冗長構成を実装するにあたり、`sakuracloud_database`リソースの実スキーマを再取得して確認したところ、`replica_user`/`replica_password`属性(レプリケーション接続の認証情報)は存在するが、「このアプライアンスは別のアプライアンスのレプリカである」ことを宣言する属性が存在しないことが判明した。2つ目の`sakuracloud_database`リソースを作ってプライマリと自動的に紐付ける、という操作はTerraformの現行スキーマだけでは宣言的に完結できない。
- PITR(継続バックアップ)は`continuous_backup`ブロックとして存在するが、`database_version`の明示指定とNFSサーバーアドレスが前提条件であることが実スキーマから判明した。

### 実装内容

- **アプリ用ユーザー/マイグレーション用ユーザーの分離**: `cyrilgdn/postgresql`プロバイダを新規導入し、DBアプライアンスへ直接SQL接続して`postgresql_role`(アプリ用ロール)・`postgresql_grant`(テーブル/シーケンスへのDML権限のみ)・`postgresql_default_privileges`(将来のマイグレーションで追加されるテーブル/シーケンスにも自動適用)を宣言的に管理するようにした。`modules/database`の変数を`username`/`password`から`admin_username`/`admin_password`(マイグレーション用、DDL権限あり)+`app_username`/`app_password`(アプリ用、DML権限のみ)に分離。
- **冗長構成**: プライマリ側の`replica_user`/`replica_password`を変数経由で設定可能にした(既定は未設定)。実際のレプリカアプライアンスの作成・紐付けはTerraformだけでは完結しないため、手動操作が必要であることを`docs/database-design.md`に明記した(誤って「自動で冗長化される」と誤解されないようにするため)。
- **TLS**: `postgresql`プロバイダの接続に`sslmode = "require"`を既定で設定。アプリケーションの`DATABASE_URL`についても、本番では`sslmode=require`以上が必要である旨を`.env.example`に追記した。
- **PITR**: `continuous_backup`ブロックを変数(`enable_continuous_backup`、既定`false`)経由でオプトイン設定できるようにした。NFSストレージのアドレスが未確定のため、staging/production共に現状は無効のまま。
- **監視**: `monitoring_suite`ブロックを有効化(既定`true`)。
- **復元手順**: `docs/database-design.md`に、日次バックアップ・PITRからの復元手順(暫定版)と復元後の確認チェックリストを記載した。

### 変更ファイル

- `terraform/modules/database/{main,variables,outputs,versions}.tf` — アプリ用/マイグレーション用ユーザー分離、`postgresql`プロバイダ導入、冗長構成・PITR・監視の変数追加
- `terraform/environments/{staging,production}/{main,variables,versions}.tf` — 変数名変更(`db_username`/`db_password` → `db_admin_username`/`db_admin_password`+`db_app_username`/`db_app_password`)、`postgresql`プロバイダのrequired_providers追加
- `terraform/environments/{staging,production}/terraform.tfvars` — 変数名変更に追従
- `terraform/README.md` — `postgresql`プロバイダの内部ネットワーク到達性要件、更新済みのnetwork/databaseドキュメントへのリンクを追記
- `.env.example` — 本番の`DATABASE_URL`にsslmode要件のコメントを追記
- `docs/database-design.md`(新規) — PostgreSQL本番構成の設計書(ユーザー分離・冗長構成・TLS・バックアップ・PITR・監視・復元手順)

### 検証結果

- `terraform fmt -check -recursive` → 差分なし
- `terraform validate`(staging/production両方) → 成功(`postgresql`プロバイダを含む実スキーマに対する検証)
- `terraform plan`(ダミー値使用、staging/production両方):
  - `postgresql_role` / `postgresql_grant` × 2 / `postgresql_default_privileges` × 2 の**計5リソースの作成計画がエラーなく生成される**ことを確認(新規作成のみの計画は`postgresql`プロバイダ側のDB接続を必要とせず、ローカルで完結することを確認できた)
  - その後、想定通り`sakuracloud`プロバイダ側で`AccessToken is required`により停止することを確認(実クラウードAPIには到達していない)
  - `.tfstate`ファイルが生成されていないことを確認
- バックエンド(`go build/vet/test`)・フロントエンド(`tsc -b`/`npm run test`)は本フェーズでコード変更していないため再実行のみ、すべて成功

### セキュリティ上の確認

- アプリケーション実行時に使うDB権限を、スキーマ変更ができない最小権限(DMLのみ)に分離した。SQLインジェクション等でアプリが侵害された場合の被害範囲を、データの読み書きに限定する設計とした。
- 実際のDBパスワード(admin/app双方)・レプリケーションパスワードはコード・ドキュメントのいずれにも記載していない。すべて`sensitive = true`かつデフォルト値なし。
- TLS接続(`sslmode=require`)を`postgresql`プロバイダ・アプリケーション双方の既定方針として明記した。
- `terraform apply`・さくらのクラウードの有料リソース作成、実DBへの変更、Git commit・push、破壊的操作は一切実施していない。

### 残っている課題

- レプリカアプライアンスの実際の作成・紐付け手順の確立(フェーズ17より前に実環境で検証)
- PITR用NFSストレージの準備(フェーズ15)
- `postgresql`プロバイダによる`apply`の実行場所の確定(フェーズ12・13のCI/CD設計と合わせて決定)
- 実際のデータベース名の確認(`postgres`と仮定しているが初回`apply`後に確認要)
- TLSの`verify-full`への強化(CA証明書の入手確認後)
- アプリサーバ内部NICの静的IP割り当て(フェーズ8から継続、フェーズ10近辺)
- ロードバランサ用の公開スイッチ構築・TLS終端の確定(フェーズ11)

### 次のフェーズ

フェーズ10「アプリサーバ構築」に進み、App Server 1・2の構築(Terraformで既に雛形は作成済み)、Docker導入、デプロイユーザーの作成、OSセキュリティ設定、Nginx、ログ転送、監視エージェント、ヘルスチェックを実装する。

---

## フェーズ10:アプリサーバ構築(完了)

`terraform apply`・さくらのクラウードの有料リソース作成は本フェーズでも実施していない。

### 調査結果

- フェーズ7〜9時点の`modules/app_server`は、サーバー本体(ディスク・NIC・SSH鍵によるアクセス)のみを構築しており、OSレベルのセットアップ(Docker導入・デプロイユーザー・セキュリティ強化)が未実装だった。
- さくらのクラウードには「起動スクリプト」機構(`sakuracloud_note`、`class = "shell"`をディスクの`disk_edit_parameter.note_ids`から参照)があり、これが初回起動時のプロビジョニングの標準的な手段であることを、実プロバイダスキーマから確認した。
- 「Nginx」というフェーズ10のチェックリスト項目は、フェーズ4で既にコンテナ内(`nginxinc/nginx-unprivileged`)に実装済みであり、ホストOS上に別途Nginxを追加インストールする必要はないと判断した(二重運用を避けるため)。

### 実装内容

- **起動スクリプト**(`terraform/modules/app_server/templates/startup.sh.tftpl`、新規): OSパッケージ更新・`unattended-upgrades`による自動セキュリティ更新・`ufw`によるホストファイアウォール(22/80/443のみ、さくらのクラウードのパケットフィルタと同じ許可リストを多層防御として設定)・`fail2ban`・SSHのroot直接ログイン禁止(`PermitRootLogin no`)を実装。
- **デプロイ用ユーザー**: 起動スクリプト内で`deploy`ユーザー(変数`deploy_user`で変更可)を作成し、SSH公開鍵を設定、`docker`グループのみに追加(sudoは付与しない)。
- **Docker導入**: Docker Engine + Docker Composeプラグインを起動スクリプトでインストールし、`docker.service`を有効化(サーバー再起動後もコンテナが自動復旧する)。ログドライバに`max-size`/`max-file`を設定し、ログによるディスク枯渇を防止。
- **監視エージェント**: `node_exporter`をsystemdサービスとして導入(`127.0.0.1:9100`のみでリッスン、外部非公開)。実際の収集方法・転送先の選定はフェーズ14に委ねることを明記。
- **ヘルスチェック**: 新規実装はないが、フェーズ2(アプリ)・フェーズ4(コンテナ)・本フェーズ(OS/systemd)・フェーズ7(LB/外形監視モジュール)がどう組み合わさって全体のヘルスチェックを構成するかを`docs/app-server-design.md`に整理した。
- `terraform/modules/app_server/main.tf`に`sakuracloud_note`リソースを追加し、`templatefile()`でデプロイユーザー名・SSH公開鍵を埋め込んだ上で、`disk_edit_parameter.note_ids`から参照するよう配線。

### 変更ファイル

- `terraform/modules/app_server/templates/startup.sh.tftpl`(新規) — 起動スクリプト本体
- `terraform/modules/app_server/main.tf` — `sakuracloud_note`リソース追加、`note_ids`配線
- `terraform/modules/app_server/variables.tf` — `deploy_user`変数追加
- `docs/app-server-design.md`(新規) — アプリサーバ構築の設計書(スコープ境界・起動スクリプト内容・デプロイ用ユーザー権限・ヘルスチェック構成の整理)

### 検証結果

- **起動スクリプトのテンプレート展開**: `templatefile()`関数を単独のスクラッチ構成で実行し、シェルスクリプト自体の`${VAR}`展開(`$${NODE_EXPORTER_VERSION}`のようなエスケープが必要な箇所)とTerraformの`${...}`補間が衝突しないことを実際にレンダリングして確認した(推測で書かず、実際に評価させて確認)。
- レンダリング結果(113行)を`bash -n`(構文チェックのみ、実行はしない)にかけ、シェル構文エラーがないことを確認した。
- `terraform fmt -check -recursive` → 差分なし
- `terraform validate`(staging/production両方) → 成功
- `terraform plan`(ダミー値使用、staging/production両方) → 引き続き`postgresql`関連5リソースの計画は正常に生成され、`sakuracloud`プロバイダ側で`AccessToken is required`により意図通り停止することを確認。`.tfstate`は生成されていない
- バックエンド(`go build/vet/test`)・フロントエンド(`tsc -b`/`npm run test`)は本フェーズでコード変更していないため再実行のみ、すべて成功

### セキュリティ上の確認

- デプロイ用ユーザーにはsudo権限を付与せず、`docker`グループのみとした(必要最小限の権限)。
- SSHのroot直接ログインを完全に禁止(パスワード認証無効化(フェーズ7)に加えた多層防御)。
- ホストファイアウォール(`ufw`)をさくらのクラウードのパケットフィルタ(フェーズ8)と同じ許可リストで設定し、クラウード側の設定ミスに対する多層防御とした。
- `node_exporter`は`127.0.0.1`のみでリッスンし、外部から到達不可能な設定にした。
- 起動スクリプトにSSH公開鍵は埋め込むが、秘密鍵・パスワード等の秘密情報は一切含まれていないことを確認済み。
- `terraform apply`・さくらのクラウードの有料リソース作成、Git commit・push、破壊的操作は一切実施していない。

### 残っている課題

- ログ集約先・監視スタックの選定(フェーズ14)
- node_exporterの実際のスクレイプ方法の確定(フェーズ14)
- 緊急時のroot操作手順(コンソール機能)の運用手順書への記載(フェーズ18)
- アプリサーバ内部NICの静的IP割り当て(フェーズ8から継続、実環境検証待ち)
- ロードバランサ用の公開スイッチ構築・TLS終端の確定(フェーズ11)
- Terraform stateのリモートbackend移行(フェーズ7から継続)

### 次のフェーズ

フェーズ11「ロードバランサ・DNS・HTTPS」に進み、アプリサーバ2台のロードバランサへの登録、`/health/ready`によるヘルスチェックの実配線、TLS証明書、HTTPからHTTPSへの転送、`levelog.matsu0122.com`のDNS設計、片系停止試験の計画を実装する。

---

## フェーズ11:ロードバランサ・DNS・HTTPS(完了)

`terraform apply`・さくらのクラウードの有料リソース作成・DNSレコードの変更は本フェーズでも一切実施していない。

### 調査結果

- フェーズ7時点の`modules/load_balancer`は、公開スイッチ・IP・VIPをすべて外部変数として要求しており、production環境の`terraform.tfvars`にも値を入れられない状態(未確定)だった。
- 実プロバイダスキーマ(`sakuracloud_internet`、通称「Switch+Router」)を確認した結果、この公開スイッチ・グローバルIP群は**Terraformで直接プロビジョニングできる**ことが判明した(`switch_id`・`ip_addresses`・`gateway`・`netmask`は作成後にすべて自動的に払い出される計算値)。
- さくらのクラウードのロードバランサはL4(TCP)パススルー型であり、TLS終端機能を持たないことを設計判断の前提として明記した(各アプリサーバのNginxでTLSを終端する設計とした)。
- ACMEのHTTP-01チャレンジは、2台のバックエンドのどちらが検証リクエストを受けるか制御できないため、L4パススルー型ロードバランサ配下では不安定になりうるという問題を特定し、DNS-01チャレンジを推奨方式として記録した。

### 実装内容

- **ロードバランサの公開IP自動払い出し**: `modules/load_balancer`に`sakuracloud_internet`リソースを追加し、`public_switch_id` / `ip_addresses` / `netmask` / `gateway` / `vip_address`をすべて自動導出するように変更。外部から必要な入力は`vrid`(任意の一意な整数)のみに削減した。
- **依存関係の循環の解消**: ロードバランサの実IPが判明するタイミングと、アプリサーバのパケットフィルタ設定(`network`モジュール)のタイミングの間に生じる循環依存を、`lb_known_ip_addresses`変数による意図的な2段階ブートストラップ(初回は`0.0.0.0/0`で`apply`→実IPを確認→変数を設定して再`apply`)で解消した。
- **ポート80/443双方のVIP**: `vip_ports`変数(既定`[80, 443]`)を追加し、`dynamic`ブロックで両方のVIPを生成。ヘルスチェックプロトコルはポートに応じて`http`/`https`を自動選択。
- **HTTPからHTTPSへの転送**: `frontend/nginx.conf`を2つの`server`ブロック(8080=平文HTTP、8443=TLS終端)に分割。`/health/`と`/healthz`はリダイレクトから除外し、ロードバランサ・外形監視のヘルスチェックが平文のままでも200を受け取れるようにした。
- **TLS終端**: ポート8443で`/etc/nginx/tls/`配下の証明書を読み込む設定を追加。共通ロケーション定義を`nginx-locations.conf`に切り出し、両ブロックから`include`。
- **証明書取得方針**: アプリサーバの起動スクリプト(フェーズ10)に`certbot`のインストールと証明書配置用ディレクトリ(`/opt/levelog/tls`)の作成を追加。実際の発行コマンド(DNS APIの認証情報を要する)は自動実行せず、手動/別途プロビジョニング手順とすることを明記した。
- **DNS設計**: `levelog.matsu0122.com` → ロードバランサVIP、`staging.levelog.matsu0122.com` → staging アプリサーバIPのA レコード設計を文書化。実際のレコード変更は行っていない(ユーザーの明示的な許可が必要な操作のため)。ゾーンの現在の管理場所が未確認であることも明記した。
- **片系停止試験**: 実施はフェーズ17に委ね、本フェーズでは試験手順・合格基準を計画として文書化した。

### 変更ファイル

- `terraform/modules/load_balancer/{main,variables,outputs}.tf` — `sakuracloud_internet`追加、公開IP/VIP自動導出、複数ポートVIP対応
- `terraform/environments/production/{main,variables,terraform.tfvars}` — LB関連の外部変数を削除し`lb_known_ip_addresses`による2段階ブートストラップへ変更
- `frontend/nginx.conf` — HTTP→HTTPSリダイレクト+TLS終端の2ブロック構成に全面書き換え
- `frontend/nginx-locations.conf`(新規) — 両ブロック共通のロケーション定義
- `frontend/Dockerfile` — `EXPOSE 8443`追加、`HEALTHCHECK`を`/healthz`に変更
- `docker-compose.prod.yml` — `TLS_CERT_DIR`ボリュームマウント追加、ポート公開を`80:8080`/`443:8443`に変更
- `terraform/modules/app_server/templates/startup.sh.tftpl` — `certbot`インストール、`/opt/levelog/tls`ディレクトリ作成を追加
- `terraform/README.md` — LB自動払い出し・TLS設計への参照を追記
- `README.md` — `docker-compose.prod.yml`の`TLS_CERT_DIR`必須化を反映
- `docs/tls-design.md`(新規) — TLS終端位置・LB配線・証明書方針・DNS設計・片系停止試験計画
- `frontend/nginx-locations.conf` — 実機再検証(下記)で発見した`add_header`継承欠落の修正(`location /assets/`・`location = /index.html`にセキュリティヘッダーを再宣言)

### 検証結果

- `terraform fmt -check -recursive` → 差分なし
- `terraform validate`(staging/production両方) → 成功
- `terraform plan`(ダミー値使用、production) → **必須変数が`lb_vrid`のみに削減された状態**で計画が正常に生成され、想定通り`sakuracloud`プロバイダ側で`AccessToken is required`により停止することを確認。`.tfstate`は生成されていない
- **Nginx設定の実機検証**(Dockerビルド+自己署名証明書で実施):
  - 初回ビルドで`nginx-locations.conf`を`conf.d/`配下に置いたところ、ベースイメージが`conf.d/*.conf`をhttpコンテキストで自動`include`するため`"location" directive is not allowed here`で起動失敗することを発見。`/etc/nginx/snippets/`へ配置場所を変更して解決
  - 平文HTTP(8080)へのアクセスが`https://`へ301リダイレクトされることを確認
  - `/health/live`(8080、平文)がリダイレクトされず200を返すことを確認
  - `/healthz`(Nginx自身)が200を返すことを確認。当初`add_header Content-Type`でヘッダが重複する不具合を発見し、`default_type`に変更して解決
  - HTTPS(8443、自己署名証明書)でトップページが正常に配信されることを確認
  - `COOKIE_SECURE=true`の状態でHTTPS経由の新規登録が成功し、`Set-Cookie`に`Secure`属性が付与されることを確認(TLS終端後もCookie発行が正しく機能することの確認)
  - コンテナの`HEALTHCHECK`が`/healthz`を使って`healthy`と判定されることを確認
- **セキュリティヘッダーの実機再検証(追加で発見した不具合)**: 上記検証後、`curl`でレスポンスヘッダーをパスごとに比較したところ、`X-Content-Type-Options` / `X-Frame-Options` / `Referrer-Policy`が`/`・`/index.html`・`/assets/*`のレスポンスに付与されていないことを発見した(`/api/*`・`/health/*`には正しく付与されていた)。原因はnginxの`add_header`継承仕様(現在のコンテキストで1つでも`add_header`を宣言すると親コンテキストからの継承が完全に打ち切られる)で、`location /assets/`・`location = /index.html`が独自に`Cache-Control`を`add_header`していたため、共通で宣言していた3つのセキュリティヘッダーが継承されていなかった。さらに`location /`の`try_files`フォールバックはinternal redirectで`/index.html`に対して再度location解決を行うため、SPAのほぼ全HTML応答に影響する不具合だった。`location /assets/`・`location = /index.html`双方に3つのセキュリティヘッダーを明示的に再宣言する形で修正し、修正後は`/`・`/index.html`・`/assets/*`・`/api/*`のすべてで3ヘッダーが付与されることを`curl`で再確認した(詳細は`docs/tls-design.md`5.1節)。
- バックエンド(`go build/vet/test`)・フロントエンド(`tsc -b`/`lint`/`npm run test`/`npm run build`)すべて成功

### セキュリティ上の確認

- ヘルスチェック用パス以外は常にHTTPSへリダイレクトされる設定にした。
- 証明書の秘密鍵はイメージに焼き込まず、実行時にボリュームマウントする設計にした(`TLS_CERT_DIR`必須変数)。
- DNS APIの認証情報を起動スクリプトに埋め込むことを避け、証明書発行を手動/別プロビジョニング手順とした。
- 検証で使用した証明書は自己署名かつ1日限りの有効期限の使い捨てで、検証後に削除済み。
- DNSレコードの変更・さくらのクラウードの有料リソース作成・`terraform apply`・Git commit・push・破壊的操作は一切実施していない。

### 残っている課題

- `matsu0122.com`ゾーンの現在の管理場所の確認(DNS変更前に必須)
- DNS-01チャレンジ用certbotプラグインの選定
- 証明書更新自動化の実装
- ロードバランサの`assigned_ip_addresses`実際の並び順の確認(初回`apply`後)
- 片系停止試験の実施(フェーズ17)
- Terraform stateのリモートbackend移行(フェーズ7から継続)
- アプリサーバ内部NICの静的IP割り当て(フェーズ8から継続)

### 次のフェーズ

フェーズ12「CI」に進み、Pull Requestごとに実行するGo format/vet/test/脆弱性チェック、Frontend lint/typecheck/test/build、Terraform fmt/validate/plan、秘密情報混入チェックのGitHub Actionsワークフローを実装する。

---

## フェーズ12:CI(完了)

さくらのクラウードの有料リソース作成・`terraform apply`・実際のGitHub Actions実行(pushによるトリガー)は本フェーズでも一切行っていない。ワークフロー自体はローカルで`actionlint`・各コマンドの単体実行により検証した。

### 調査結果

- `.github/`ディレクトリ自体が存在せず、CIは一切未整備だった。
- フェーズ5で作成した統合テスト(`backend/internal/handler/router_integration_test.go`)は`TEST_DATABASE_URL`未設定時に自動的に`t.Skip`される設計であり、通常の`go test ./...`には影響しないが、実DBに対する検証(XP二重付与防止・認可等)をCIで確実に実行するには、Postgresサービスコンテナと`TEST_DATABASE_URL`の設定が別途必要と判明した。`TestMain`(同ファイル)がマイグレーション適用を含め自己完結しているため、CI側は接続文字列を渡すだけでよいことを確認した。
- フェーズ4で指摘されていた「`nginx.conf` / `docker-compose.prod.yml`に対する自動テスト(CI上でのビルド・起動確認等)がまだない」という残課題が未解消のままだった。
- Terraformの`validate`は(フェーズ7〜11で確認済みの通り)さくらのクラウード認証情報なしで完結するため、CI上でクラウード認証情報を一切扱わずに実行できると判断した。

### 実装内容

`.github/workflows/ci.yml`を新規作成し、`main`向けPull Request・pushをトリガーに以下のジョブを並列実行する構成にした(`concurrency`で同一ブランチの古い実行を自動キャンセル)。

- **secret-scan**: `gitleaks`(Dockerイメージ、`zricethezav/gitleaks:latest`)でリポジトリ全体(`fetch-depth: 0`でコミット履歴を含めて取得)を走査し、秘密情報の混入を検出する。
- **backend**: `gofmt -l .`の差分チェック(差分があれば`::error::`で失敗)、`go vet ./...`、`go build ./...`、`go test ./... -race`(フェイクリポジトリによるユニットテストのみ)。
- **backend-integration**: `postgres:16-alpine`のサービスコンテナ(ヘルスチェック付き)を使い、`TEST_DATABASE_URL`を設定した上で`go test ./internal/handler/... -v -race`を実行し、実DBに対する統合テスト(認証・ミッションのライフサイクル・XP二重付与防止・認可)をCI上で常時検証する構成にした(フェーズ5からの残課題を解消)。
- **frontend**: `npm ci`後、`tsc -b`(型チェック)・`npm run lint`(oxlint)・`npm run test -- --run`(Vitest)・`npm run build`。
- **docker-build**: `backend/Dockerfile` / `frontend/Dockerfile`を`docker build`し、イメージが壊れずビルドできることを確認する(レジストリへのpush・認証は行わない)。フェーズ4の残課題(コンテナ構成の自動テストがない)を解消。
- **terraform-fmt**: `terraform fmt -check -recursive`。
- **terraform-validate**: staging/production環境それぞれで`terraform init` → `terraform validate`(`strategy.matrix`で並列化)。さくらのクラウード認証情報は一切与えず、静的な構文・スキーマ検証のみに限定した(ワークフロー中に明示コメントで「ここでクラウード認証情報を扱わない」ことを明記)。

`terraform plan`はワークフローに含めなかった。理由: `plan`の実行にはさくらのクラウード認証情報(`SAKURACLOUD_ACCESS_TOKEN`等)をCI Secretsとして登録する必要があり、これはリポジトリへの書き込み権限を持つ者に対して実質的にクラウードリソースを操作できる権限を与えることになる。現時点ではそこまでの権限をCIに付与する判断を下しておらず、`validate`(認証情報なしで完結する静的検証)までに意図的に留めた。CI/CDでの`plan`/`apply`の扱いはフェーズ13(CD)で改めて設計する。

`README.md`に「CI」節を新規追加し、上記ジョブの一覧と目的を記載した。

### 変更ファイル

- `.github/workflows/ci.yml`(新規) — 上記7ジョブのGitHub Actionsワークフロー
- `README.md` — 「CI」節を新規追加

### 検証結果

- **ワークフローYAMLの静的検証**: `ruby -ryaml`でYAML構文が正しいことを確認。`actionlint`(Dockerイメージ`rhysd/actionlint`、shellcheck統合込み)を実行し、警告・エラーともに0件であることを確認(アクションのバージョン指定・式の構文・シェルスクリプト部分を含めた検証)。
- **各ジョブが実行するコマンドの単体実行**(GitHub Actions自体は実行していないため、同じコマンドをこのセッションのローカル環境で個別に実行して検証):
  - `gofmt -l .`(差分なし)・`go vet ./...`・`go build ./...`・`go test ./... -race` → すべて成功
  - `go test ./internal/handler/... -v -race`(`TEST_DATABASE_URL`をローカルの開発用Postgresへ向けて実行) → 全6テスト成功(フェーズ5と同様の実行方法)
  - `npx tsc -b`・`npm run lint`・`npm run test -- --run`(20ケース)・`npm run build` → すべて成功
  - `docker build ./backend`・`docker build ./frontend` → 両方成功
  - `terraform fmt -check -recursive`(差分なし)、`terraform init` + `terraform validate`(staging/production両方) → 成功
  - `docker run --rm -v "$PWD:/repo" zricethezav/gitleaks:latest detect --source=/repo -v --redact` → `no leaks found`で正常終了(exit 0)を確認。ワークフロー内のコマンドがそのまま動作することの確認であり、実際のGitHub Actions実行環境での動作は次回以降のPRで初めて検証される
- 実際のGitHub Actions上での実行(pushまたはPull Requestのトリガー)は本フェーズでは行っていない。ワークフローファイルをコミット・pushしていないため。

### セキュリティ上の確認

- ワークフロー全体を通じて、さくらのクラウードの認証情報・DBパスワード(本番)・その他の実秘密情報は一切登場しない。`backend-integration`ジョブの`TEST_DATABASE_URL`はCI実行のたびに使い捨てで起動されるサービスコンテナのダミー認証情報であり、実運用の秘密情報ではない。
- `terraform-validate`ジョブは意図的に`plan`/`apply`を含めず、クラウード認証情報をCI Secretsに登録する必要がない設計にとどめた(理由は「実装内容」節に記載)。
- `secret-scan`ジョブにより、今後のPull Requestで秘密情報が誤ってコミットされた場合に検出できる体制を整えた。
- `docker-build`ジョブはイメージのビルドのみを行い、レジストリへのpush・認証情報の使用は一切ない。
- Git commit・push、さくらのクラウードの有料リソース作成、`terraform apply`、破壊的操作は一切実施していない。

### 残っている課題

- ワークフローファイル自体がまだGitにコミット・pushされていないため、実際のGitHub Actions実行(Pull Request作成時の動作)がまだ検証されていない。次回コミット・push後、最初のPull Requestで実際の動作を確認する必要がある。
- `terraform plan`のCI組み込み(クラウード認証情報のCI Secrets登録方針を含む)はフェーズ13(CD)で設計する。
- Dependabot等による依存パッケージの脆弱性自動チェック・更新は未導入(セキュリティ確認より広い範囲のため、フェーズ16で検討)。
- `matsu0122.com`ゾーンの管理場所確認、DNS-01チャレンジ用certbotプラグイン選定、証明書更新自動化(フェーズ11から継続)
- 片系停止試験の実施(フェーズ17)
- Terraform stateのリモートbackend移行(フェーズ7から継続)
- アプリサーバ内部NICの静的IP割り当て(フェーズ8から継続)

### 次のフェーズ

フェーズ13「CD」に進み、`main`へのマージをトリガーにした本番/staging環境へのデプロイパイプライン(Dockerイメージのビルド・レジストリへのpush、アプリサーバへのデプロイ、マイグレーション実行の排他制御、ロールバック手順)を設計・実装する。

---

## フェーズ13:CD(完了)

さくらのクラウードの有料リソース作成・`terraform apply`・実際のGitHub Actions実行・実SSHデプロイは本フェーズでも一切行っていない。アプリサーバ自体がまだ構築されていない(`terraform apply`未実施)ため、実デプロイ先が存在しない。本フェーズで検証できたのはワークフロー/スクリプトの静的検証と、ローカルで再現可能な範囲(マイグレーション排他制御・Terraformテンプレートのレンダリング・Docker Compose構文)に限られる。

### 調査結果

- フェーズ1から継続していた課題「起動時自動マイグレーションが複数インスタンス同時起動時に競合しうる」について、実際に競合するかを`backend/internal/db/migrate_test.go`で検証したところ、**8並行での初回マイグレーションが`CREATE TABLE IF NOT EXISTS schema_migrations`の段階で`duplicate key value violates unique constraint "pg_type_typname_nsp_index"`により確実に失敗する**ことを実測で確認した(想定していた「`schema_migrations`への重複INSERT」よりも早い、カタログレベルでの競合だった)。仮説ではなく実測で再現できたため、修正の必要性が明確になった。
- コンテナオーケストレーション基盤(Kubernetes等)を導入していない現状規模(アプリサーバ最大2台)では、SSH経由での`docker compose`操作によるデプロイが妥当と判断した。
- レジストリの選定にあたり、さくらのクラウードにもコンテナレジストリ相当のサービスはあるが、追加の契約・Terraform管理外の設定が必要になるため見送り、リポジトリと同じGitHubアカウントで完結し`secrets.GITHUB_TOKEN`のみで認証できるGitHub Container Registry(GHCR)を採用した。
- `terraform/modules/app_server`の起動スクリプトには、デプロイ時に使う`/opt/levelog/docker-compose.prod.yml`・`.env`を配置する仕組みがまだなかった(`/opt/levelog`ディレクトリ自体はフェーズ10で作成済み)。

### 実装内容

- **マイグレーション実行の排他制御**: `backend/internal/db/migrate.go`を変更し、`Migrate()`全体(`schema_migrations`テーブルのCREATE TABLE含む)をPostgresのセッションレベルアドバイザリロック(`pg_advisory_lock` / `pg_advisory_unlock`)で排他化した。専用コネクションを1本取得して処理全体で使い回すことで、ロックとマイグレーション適用が同一セッション上で行われることを保証。ロックはセッションスコープのため、コネクション切断時(クラッシュ等)は自動解放され、ロックが永続する心配がない。デプロイパイプライン分離ではなくアプリケーションコード側の排他制御を選んだ理由は`docs/deployment-design.md`4節に記載。
- **マイグレーションのテスト**: `backend/internal/db/migrate_test.go`(新規)。`TestMigrate_ConcurrentInstancesDoNotRace`(8並行インスタンスでの同時初回マイグレーションを再現、使い捨てのスクラッチDBを`CREATE DATABASE`/`DROP DATABASE`で都度作成・破棄)、`TestMigrate_SecondRunIsNoOp`(再起動時の再実行が冪等であることを確認)。
- **CIワークフロー(`.github/workflows/ci.yml`)**: フェーズ12で実装済み、本フェーズでの変更なし。
- **CDワークフロー(`.github/workflows/cd.yml`、新規)**: `resolve`(デプロイ対象の環境・イメージタグを決定) → `build-and-push`(GHCRへbackend/frontend両イメージをビルド・push、ロールバック時はスキップ) → `deploy-staging`または`deploy-production`。
  - トリガーは2種類: `workflow_run`(CIが`main`で成功) → 常にstagingへ自動デプロイ。`workflow_dispatch`(手動) → `environment`(staging/production)と`image_tag`(省略時は現在のHEAD、指定時はロールバック)を選択可能。
  - production環境は`strategy.matrix` + `max-parallel: 1`で2台のアプリサーバを**1台ずつ順番に**デプロイするローリング方式とした(同時に両方を落とさない)。
  - GitHub Environmentの`production`にリポジトリ設定でRequired reviewersを設定することを想定した設計(Terraform/YAMLからは設定不可、手動作業として`docs/deployment-design.md`に明記)。
- **デプロイスクリプト(`.github/scripts/deploy-host.sh`、新規)**: 対象ホスト1台に対し、`docker-compose.prod.yml`をSCPで同期 → SSHで`GITHUB_TOKEN`を使い`docker login ghcr.io`(このジョブ実行中のみ有効なトークンをその場で使用、サーバーに永続化しない) → `/opt/levelog/deploy.sh`を`IMAGE_TAG`付きで実行 → `docker logout`。
- **アプリサーバ側デプロイスクリプト**: `terraform/modules/app_server/templates/startup.sh.tftpl`を拡張し、`/opt/levelog/deploy.sh`(`docker compose pull && up -d`を実行し、`/health/ready`を最大60秒ポーリングして確認)を起動時に配置するようにした。
- **`docker-compose.prod.yml`**: `api`/`web`両サービスに`image: ghcr.io/matsu0122-png/levelog-{backend,frontend}:${IMAGE_TAG:-latest}`を追加。`build:`ブロックは維持し、フェーズ4・11で確立した`--build`付きのローカル検証手順との互換性を保った。本番デプロイでは`--build`なしの`pull` → `up -d`のみを使う。
- **`docs/deployment-design.md`(新規)**: 全体構成図、イメージのビルド・push方針、SSHデプロイ方式、`.env`の扱い(CIは作成・変更しない、初期構築時の手動作業とする設計判断とその理由)、マイグレーション排他制御の設計、必要なGitHub Secrets/Variables一覧、未確定事項をまとめた。
- `README.md`に「CD」節を追加。

### 変更ファイル

- `backend/internal/db/migrate.go` — `pg_advisory_lock`によるマイグレーション排他制御
- `backend/internal/db/migrate_test.go`(新規) — 並行マイグレーションのレース再現テスト・冪等性テスト
- `.github/workflows/cd.yml`(新規) — CDワークフロー
- `.github/scripts/deploy-host.sh`(新規) — 1ホストへのSSHデプロイスクリプト
- `terraform/modules/app_server/templates/startup.sh.tftpl` — `/opt/levelog/deploy.sh`の配置を追加
- `docker-compose.prod.yml` — `image:`フィールド追加(GHCRからのpullに対応)
- `docs/deployment-design.md`(新規) — CD設計書
- `README.md` — 「CD」節を追加

### 検証結果

- **マイグレーション排他制御(実DBで検証)**: 修正前のコードで`TestMigrate_ConcurrentInstancesDoNotRace`を3回連続実行し、**毎回**`duplicate key value violates unique constraint "pg_type_typname_nsp_index"`で失敗することを確認(バグの実在を先に証明)。アドバイザリロックを実装後、同テストを`-race`付きで5回連続実行しすべて成功、`TestMigrate_SecondRunIsNoOp`も含めすべて成功することを確認。ローカルの開発用Postgres(`POSTGRES_PORT=15432`、フェーズ5と同じ回避策)に対して実施。
- **バックエンド全体の回帰確認**: `gofmt -l .`(差分なし)・`go build ./...`・`go vet ./...`・`go test ./... -race`・`go test ./internal/handler/... -v -race`(既存の統合テスト6件)すべて成功。
- **CDワークフローの静的検証**: `ruby -ryaml`でYAML構文を確認。`actionlint`(shellcheck統合込み)で`cd.yml`・`ci.yml`双方を検証し、警告・エラー0件(初回は`SC2129`スタイル警告が出たため`resolve`ジョブの複数`echo >> $GITHUB_OUTPUT`を`{ } >> `の1回のリダイレクトへ整理して解消)。
- **`deploy-host.sh`の静的検証**: `bash -n`で構文確認。`shellcheck`(Dockerイメージ`koalaman/shellcheck`)を実行し、意図的なローカル展開ヒアドキュメント(`SC2087`)以外は警告なし(該当箇所は理由をコメントで明記の上`shellcheck disable`)。
- **`startup.sh.tftpl`のレンダリング検証**(フェーズ10と同じ手法): スクラッチ構成で`templatefile()`を実際に評価し、`/opt/levelog/deploy.sh`部分が正しくレンダリングされることを確認。**この過程で実装ミスを1件発見**: シェルの`$(seq 1 30)`をTerraformの`${`エスケープ規則を誤って適用し`$$(seq 1 30)`と二重エスケープしてしまい、レンダリング後も`$$(seq 1 30)`のまま(コマンド置換として機能しない)になっていた。Terraformのエスケープ対象は`${`に続く場合のみで、`$(`単体はエスケープ不要と判明し、`$(seq 1 30)`に修正して再レンダリングし正しく`$(seq 1 30)`になることを確認した。外側の起動スクリプト・`deploy.sh`本体それぞれを`bash -n`で個別に構文確認済み。
- **`docker-compose.prod.yml`の検証**: `docker compose -f docker-compose.prod.yml config --quiet`で構文確認。`image:`追加後も`--build`付きのローカルビルドが引き続き動作すること(`docker compose ... build api web`が成功し、`ghcr.io/matsu0122-png/levelog-backend:latest`としてローカルにタグ付けされること)を確認。検証で作成したイメージは削除済み。
- **Terraform**: `terraform fmt -check -recursive`(差分なし)、`terraform validate`(staging/production両方)成功、`terraform plan`(ダミー値使用、production)でも`sakuracloud`プロバイダ側の`AccessToken is required`で想定通り停止、`.tfstate`未生成を確認。
- フロントエンド: `npx tsc -b`・`npm run lint`・`npm run test -- --run`(20ケース)・`npm run build`すべて成功(本フェーズでフロントエンドのコード変更はなし、回帰確認のみ)。

### セキュリティ上の確認

- CDワークフロー全体を通じて、さくらのクラウードの認証情報・本番DBパスワードはコード・ワークフローファイルのいずれにも登場しない。
- `DEPLOY_SSH_KEY`・アプリサーバのホスト名(`*_APP_SERVER_HOST*`)はすべてGitHub Secrets経由の想定とし、本フェーズでは実際の値を一切設定・使用していない。
- GHCRへのpush・アプリサーバ側での`docker login`は`secrets.GITHUB_TOKEN`(そのジョブ実行中のみ有効な短命トークン)のみを使用し、長期間有効な認証情報をアプリサーバに永続化しない設計とした(`docker logout`を`trap`で保証)。
- `.env`(DBパスワード等を含む)はCIが作成・変更しない設計とし、GitHub Secretsから本番DB認証情報がサーバーへ流れる経路自体を作らないことで、CI実行権限の侵害が直接本番DBの侵害に繋がるリスクを避けた。
- productionへのデプロイは`workflow_dispatch`による手動実行のみとし、GitHub Environment保護ルール(Required reviewers、未設定・手動設定が必要)による人の承認ゲートを設計に組み込んだ。
- さくらのクラウードの有料リソース作成・`terraform apply`・実際のSSHデプロイ・Git commit・push、破壊的操作は一切実施していない。

### 残っている課題

- 実際のアプリサーバがまだ存在しないため、GitHub Secrets(`DEPLOY_SSH_KEY`等)を設定できず、本ワークフローはまだ一度も実行できていない。アプリサーバ構築(フェーズ17近辺)後、最初のstagingデプロイで実際の動作を確認する必要がある
- GitHub Environment `production`のRequired reviewers設定(リポジトリ設定画面での手動作業、未実施)
- GHCRパッケージの保持ポリシー(古いイメージの自動削除)未設定(フェーズ14または18で検討)
- ロールバック演習の実施(フェーズ17)
- `matsu0122.com`ゾーンの管理場所確認、DNS-01チャレンジ用certbotプラグイン選定、証明書更新自動化(フェーズ11から継続)
- Terraform stateのリモートbackend移行(フェーズ7から継続)
- アプリサーバ内部NICの静的IP割り当て(フェーズ8から継続)

### 次のフェーズ

フェーズ14「監視・ログ・通知」に進み、アプリケーションログ・アクセスログの集約先選定、node_exporter(フェーズ10)のスクレイプ方法確定、外形監視(フェーズ7の`monitoring`モジュール)のアラート通知先、エラー率・レイテンシ等のアプリケーションメトリクス、障害時の通知経路(Slack等)を設計・実装する。

---

## フェーズ14:監視・ログ・通知(完了)

さくらのクラウードの有料リソース作成・`terraform apply`・Grafana Cloudアカウントの作成・実サーバーへのエージェント導入は本フェーズでも一切行っていない。本フェーズで検証できたのは、Goコードの単体・統合テスト、`docker-compose.prod.yml`のローカルビルド・起動確認、Terraformテンプレートのレンダリング結果、そして実際のGrafana Alloyバイナリ(Dockerイメージ`grafana/alloy:v1.19.2`、Docker Hubから取得)による設定ファイルの`validate`である。

### 調査結果

- `docs/app-server-design.md`(フェーズ10)・`docs/production-roadmap.md`フェーズ13の「次のフェーズ」節が本フェーズのスコープを明示していた: ログ集約先選定、node_exporterのスクレイプ方法確定、外形監視のアラート通知先、アプリケーションメトリクス、障害通知経路。
- `terraform/modules/monitoring`(フェーズ7で実装済み)はさくらのクラウードの外形監視(`sakuracloud_simple_monitor`、Slack通知対応済み)だが、`terraform/environments/production/main.tf`にのみ配線されており、staging環境には配線されていなかった(調査の結果判明したギャップ)。
- バックエンドの依存は`pgx`と`golang.org/x/crypto`のみで、`client_golang`等のPrometheus公式クライアントライブラリは未導入だった。
- `docker-compose.prod.yml`は`api`コンテナのポートを一切ホストへ公開していない(`ports: []`、`expose`のみ)ため、ホスト上で稼働する監視エージェントがアプリの`/metrics`へ到達する経路がそもそも存在しなかった。
- Grafana Alloy(Docker Hub公式イメージ)を実際に`docker pull` / `docker run ... validate`でこのセッション内から検証できることを確認した(ネットワークアクセスあり)。これにより、設定ファイルの構文・コンポーネント配線を「たぶん合っている」ではなく実際のバイナリで検証できた。この過程で、`env()`関数(Alloy設定言語の標準ライブラリ)が非推奨であり、**`validate`コマンドがこの非推奨警告を実質的なエラー(`validation failed`、詳細メッセージなし)として扱う**ことを実測で発見した。後継の`sys.env()`に置き換えたところ検証が通った。ドキュメントの警告表示だけでは気づけない実装依存の挙動であり、実際にバイナリで検証した意義があった。

### 実装内容

**アプリケーションメトリクス(バックエンド)**

- `backend/internal/metrics`(新規): Prometheusテキスト形式(`https://prometheus.io/docs/instrumenting/exposition_formats/`)を手書きで出力する自前の`Registry`。`levelog_http_requests_total{method,route,status}`(カウンター)と`levelog_http_request_duration_seconds{method,route}`(ヒストグラム、`_bucket`/`_sum`/`_count`)の2メトリクスのみを持つ。`client_golang`を採用しなかった理由: 必要なメトリクスが2種類のみで、依存関係を増やすほどの価値がないと判断(`docs/monitoring-design.md`5節)。
- `backend/internal/middleware/metrics.go`(新規): `mux.Handler(r)`の戻り値(登録済みパターン文字列、例: `GET /api/missions/{id}`)をラベルに使うミドルウェア。生のURLパスを使うとID分だけラベルの組み合わせが増えるカーディナリティ爆発を避けるため、`mux`を直接ラップ(他のミドルウェアより内側)して正しいパターンを解決できるようにした。
- `backend/internal/middleware/status_recorder.go`(新規): `Logging`ミドルウェアに元々あった`statusRecorder`を`StatusRecorder`として切り出し、`Metrics`ミドルウェアと共有(2箇所目の利用が出た時点でのリファクタ、事前の抽象化ではない)。`logging.go`を追従修正。
- `backend/internal/handler/router.go`: `NewRouter`が`(http.Handler, *metrics.Registry)`を返すよう変更。`/metrics`はメインルーターには含めず、呼び出し元(`cmd/api/main.go`)が別サーバーとしてマウントする設計とした(次項)。
- `backend/cmd/api/main.go`: メインのAPIサーバー(`cfg.Port`、既定`8080`)とは別に、`/metrics`のみを持つ専用HTTPサーバーを`cfg.MetricsPort`(既定`9090`)で起動。グレースフルシャットダウンも両方に対応。メトリクスサーバーの起動失敗はアプリ本体を道連れにしない(ログのみ、非fatal)。
- `backend/internal/config/config.go`: `MetricsPort`設定(環境変数`METRICS_PORT`、既定`9090`)を追加。
- `docker-compose.prod.yml`: `api`サービスに`METRICS_PORT: 9090`環境変数と、`127.0.0.1:9090:9090`のポート公開を追加(アプリ本体のポート8080は引き続き非公開のまま)。node_exporterと同じ「ループバックのみ」の姿勢。

**Grafana Alloy(監視エージェント、Terraform)**

- `terraform/modules/app_server/templates/startup.sh.tftpl`: node_exporterブロックの直後に、Grafana Alloy(Docker Hub公式イメージ`grafana/alloy:v1.19.2`、バージョン固定)を`docker run --network host`で起動するsystemdユニット(`levelog-alloy.service`)を追加。`--network host`を使う理由: Dockerソケット経由のコンテナログ検出(`discovery.docker`)と、ループバックのみにバインドされたnode_exporter(`127.0.0.1:9100`)・アプリの`/metrics`(`127.0.0.1:9090`)への到達を同時に満たす必要があるため。副作用としてAlloy自身のHTTP UI/APIサーバーも全インターフェースへバインドされてしまう点は`--server.http.listen-addr=127.0.0.1:12345`で明示的に閉じた。
- Alloy設定ファイル(`/etc/levelog/alloy-config.alloy`)はスクリプトが静的に書き出す(秘密情報を含まない)。実際のGrafana CloudのURL・認証情報はすべて`sys.env(...)`で実行時に環境変数から読む設計とし、`/etc/levelog/monitoring.env`(このスクリプトは空ファイルを`touch`するのみ)へサーバー初期構築時に手動で設定する運用とした。`docker-compose.prod.yml`の`.env`と同じ理由(`docs/deployment-design.md`5節): CIやTerraformの実行権限が、そのまま監視基盤の認証情報の窃取につながる経路を作らないため。
- `terraform/modules/app_server/variables.tf`: 新規変数`environment_name`(非秘密、`"staging"`/`"production"`)を追加。Alloy設定の`external_labels`/`labels`に埋め込み、1つのGrafana Cloudアカウントで両環境を区別する。
- `terraform/environments/staging/main.tf` / `production/main.tf`: `app_server`モジュール呼び出しに`environment_name`を配線。
- `terraform/environments/staging/main.tf`(新規): `module "monitoring"`をstaging環境にも新規配線(調査結果で判明したギャップの解消)。`terraform/environments/staging/variables.tf`に`monitor_target`(既定`staging.levelog.matsu0122.com`)・`monitor_slack_webhook`(秘密情報、デフォルトなし)を追加。

**ドキュメント**

- `docs/monitoring-design.md`(新規): 本フェーズの設計書。全体構成図、ログ/メトリクス集約先としてGrafana Cloud無料枠を選んだ理由、node_exporterのスクレイプ方法、Alloyの導入方式、アプリケーションメトリクスの実装方針、アラート設計案、必要な環境変数一覧、残課題をまとめた。
- `docs/app-server-design.md`: 7節の「未確定・今後の判断が必要な事項」のうち、本フェーズで解消した2項目(ログ集約先・監視スタックの選定、node_exporterのスクレイプ方法)に取り消し線を付け、解決内容と`docs/monitoring-design.md`への参照を追記(過去フェーズの記録として本文は残しつつ、後続フェーズでの解消を追記する形式)。
- `README.md`: 「監視・ログ」節を新規追加。

### 変更ファイル

- `backend/internal/metrics/metrics.go`(新規) / `metrics_test.go`(新規)
- `backend/internal/middleware/metrics.go`(新規) / `metrics_test.go`(新規)
- `backend/internal/middleware/status_recorder.go`(新規) — `Logging`から切り出し
- `backend/internal/middleware/logging.go` — `StatusRecorder`使用に追従
- `backend/internal/handler/router.go` — `NewRouter`の戻り値変更、`Metrics`ミドルウェア配線
- `backend/internal/handler/router_integration_test.go` — 呼び出し箇所の追従、`TestMetrics_RecordsRequestsByRoutePattern`追加
- `backend/internal/config/config.go` / `config_test.go` — `MetricsPort`追加
- `backend/cmd/api/main.go` — 専用メトリクスサーバーの起動・シャットダウン
- `docker-compose.prod.yml` — `METRICS_PORT`環境変数・ループバック限定のポート公開
- `terraform/modules/app_server/templates/startup.sh.tftpl` — Grafana Alloy導入ブロック追加
- `terraform/modules/app_server/main.tf` / `variables.tf` — `environment_name`変数追加・配線
- `terraform/environments/staging/main.tf` / `variables.tf` — `environment_name`配線、`module "monitoring"`新規追加
- `terraform/environments/production/main.tf` — `environment_name`配線
- `docs/monitoring-design.md`(新規)
- `docs/app-server-design.md` — 残課題節の更新
- `README.md` — 「監視・ログ」節を追加

### 検証結果

- **バックエンド回帰確認**: `gofmt -l .`(差分なし)・`go build ./...`・`go vet ./...`・`go test ./... -race`(全パッケージ成功)。
- **バックエンド統合テスト**(`TEST_DATABASE_URL`をローカルの開発用Postgres、`POSTGRES_PORT=15432`、フェーズ5と同じ回避策、へ向けて実行): 既存6件 + 新規`TestMetrics_RecordsRequestsByRoutePattern`を含む全7件が`-race`付きで成功。
- **メトリクスエンドポイントの実動作確認**: `go run ./cmd/api`をローカルで実際に起動し、`GET /health/live`(メインポート)と`GET /metrics`(専用ポート)の両方が意図通り応答し、`/metrics`のレスポンスに`levelog_http_requests_total{method="GET",route="GET /health/live",status="200"} 1`が実際に記録されていることを確認した。
- **`docker-compose.prod.yml`の実動作確認**: `docker compose -f docker-compose.prod.yml build api`でイメージをビルドし、`up -d api`で実際に起動(開発用`db`コンテナに接続、マイグレーション成功)。`curl http://127.0.0.1:9090/metrics`が応答すること、`curl http://127.0.0.1:8080/health/live`(アプリ本体のポート)が**到達不可**であること(意図通り非公開)を確認した。検証後、コンテナ・ローカルビルドイメージは削除済み。
- **Grafana Alloy設定ファイルの実バイナリ検証**: Docker Hubから`grafana/alloy:v1.19.2`を取得し、`alloy validate`を実行。当初`env()`関数の非推奨警告が実質的なエラー(`validation failed`)として扱われることを発見し、`sys.env()`へ置き換えて解消(調査結果参照)。**Terraformの`templatefile()`で実際にレンダリングした`startup.sh.tftpl`から、Alloy設定ファイル部分だけを抽出し、そのままの内容で`alloy validate`が成功する**ことを確認した(レンダリング結果そのものを検証、手書きの再現コードではない)。検証で取得したDockerイメージは削除済み。
- **`startup.sh.tftpl`のレンダリング検証**(フェーズ10・13と同じ手法): スクラッチ構成で`templatefile()`を実際に評価し、`${environment_name}`が`"production"`に正しく置換されること、`bash -n`で構文エラーがないことを確認した。
- **`startup.sh.tftpl`の静的検証**: レンダリング後のスクリプト全体に対して`shellcheck`(Dockerイメージ`koalaman/shellcheck`)を実行。本フェーズで追加したAlloyブロックに起因する新規の警告は0件(既存の警告2件は本フェーズ以前からのもので変化なし)。
- **Terraform**: `terraform fmt -check -recursive`(差分なし)、`terraform validate`(staging/production両方)成功、`terraform plan`(ダミー値使用、staging/production両方)で`sakuracloud`プロバイダ側の`AccessToken is required`により想定通り停止することを確認、`.tfstate`が生成されていないことを確認した。
- フロントエンド: 本フェーズでフロントエンドのコード変更はなし。

### セキュリティ上の確認

- Grafana Cloudの認証情報(URL・APIキー)はTerraformのコード・状態・起動スクリプトのいずれにも一切登場しない。`sys.env(...)`で実行時に`/etc/levelog/monitoring.env`(このフェーズでは空ファイルを`touch`するのみ)から読む設計とし、実際の値の設定はサーバー初期構築時の手動作業とした(`docs/deployment-design.md`の`.env`と同じ方針)。
- アプリの`/metrics`エンドポイントは認証を持たないが、(1)メインのアプリポートとは別ポートに分離し、(2)`docker-compose.prod.yml`で`127.0.0.1`のみに公開しているため、サーバー外部からは到達不能。Nginx(`frontend/nginx-locations.conf`)も`/api/`・`/health/`以外はバックエンドへプロキシしないため、仮にこの分離がなくても外部到達経路はなかった(多層防御)。
- Grafana Alloyは`--network host`で稼働するため、アプリのメトリクスポート・node_exporterと同様にホストのループバックへ到達できるが、Alloy自身のHTTP UI/APIサーバーも`--server.http.listen-addr=127.0.0.1:12345`でループバックのみに明示的に制限した。
- Alloyコンテナは`/var/run/docker.sock`を読み取り専用(`:ro`)でマウントしており、書き込み・コンテナ操作はできない(ログ検出のための読み取りのみ)。
- staging環境に新規追加した外形監視(`module "monitoring"`)の`monitor_slack_webhook`変数はデフォルト値なしの`sensitive = true`とし、production環境の既存パターンをそのまま踏襲した。
- さくらのクラウードの有料リソース作成、`terraform apply`、Grafana Cloudアカウントの作成、Git commit・push、破壊的操作は一切実施していない。

### 残っている課題

- Grafana Cloudアカウントの作成・APIキー発行(手動作業、未実施)。アプリサーバ自体も未構築のため、`/etc/levelog/monitoring.env`への実際の値設定はアプリサーバ構築後になる。
- Grafana Cloud Alertingのアラートルール自体の作成(`docs/monitoring-design.md`6節に条件案を記載したが、実装〈Terraform `grafana`プロバイダ導入かGrafana UI手動設定か〉は未着手)。
- Nginx(`web`コンテナ)自体のメトリクス(リクエスト数・接続数等)は本フェーズの収集対象に含めていない(`docs/monitoring-design.md`7節)。
- Grafana Alloyの`docker run`イメージバージョン固定(`v1.19.2`)の更新ポリシー未定義。
- GHCRパッケージの保持ポリシー(フェーズ13から継続)
- `matsu0122.com`ゾーンの管理場所確認、DNS-01チャレンジ用certbotプラグイン選定、証明書更新自動化(フェーズ11から継続)
- 片系停止試験の実施(フェーズ17)
- Terraform stateのリモートbackend移行(フェーズ7から継続)
- アプリサーバ内部NICの静的IP割り当て(フェーズ8から継続)

### 次のフェーズ

フェーズ15「バックアップと復元」に進み、PostgreSQL(フェーズ9)のバックアップ方式の実運用確認、バックアップからの復元手順の検証、アプリサーバ・設定ファイル(Terraform管理外のものを含む)のバックアップ方針を設計・実装する。

---

## フェーズ15:バックアップと復元(完了)

さくらのクラウードの有料リソース作成・`terraform apply`は本フェーズでも一切行っていない。一方、PostgreSQLのバックアップ・復元の一連の流れ自体は、ローカルの開発用Postgresに対して`pg_dump`/`pg_restore`で実際にリハーサルした(検証結果参照)。

### 調査結果

- `docs/database-design.md`(フェーズ9)は、PITR(継続バックアップ)に「NFSサーバーのアドレスを事前に用意する」という外部前提条件が残っており、これが未解決のままだった。本フェーズで`terraform providers schema`とさくらのクラウードプロバイダの公式ドキュメント(GitHub `sacloud/terraform-provider-sakuracloud`)を実際に確認したところ、**`sakuracloud_nfs`(NFSアプライアンス自体を宣言的に作成できるリソース)がプロバイダに存在する**ことが分かった。これにより「NFSを事前に用意する」という手動の前提条件自体を取り除けることが判明した。
- 同じ調査で、`sakuracloud_auto_backup`(任意のディスクに対する週次スナップショットバックアップ)、`sakuracloud_database_read_replica`(読み取り専用レプリカ)という2つのリソースの存在も確認した。前者はアプリサーバのboot diskバックアップにそのまま使える(本フェーズで採用、後述)。後者は`docs/database-design.md`3節の「冗長構成のプライマリ↔スタンバイの紐付けを宣言的にできない」という課題に関連しそうだが、読み取り専用でありフェイルオーバー用途に使えるかは未調査のため、残課題として記録するに留めた。
- `docker-compose.prod.yml`の`api`サービスは`db`サービスを含まない設計(本番DBは別管理)だが、ローカルの`docker-compose.yml`の`db`(開発用Postgres)はLevelogと同じマイグレーション・スキーマを持つため、さくらのクラウードの実アプライアンスがなくても「バックアップ→データ消失→復元→アプリケーションからの疎通確認」という手順そのものは実機同等の内容で検証できると判断した。

### 実装内容

**PostgreSQL PITR用NFSの自己プロビジョニング**

- `terraform/modules/database/main.tf`: `sakuracloud_nfs.pitr`を追加(`enable_continuous_backup`が有効かつ`continuous_backup_nfs_connect`未指定の場合のみ作成)。DBアプライアンスと同じ内部スイッチのみに接続する設計とし、`connect`文字列(`nfs://<IP>/export`)は`local.continuous_backup_connect`で自動導出するようにした。
- `terraform/modules/database/variables.tf`: `continuous_backup_nfs_ip_address` / `continuous_backup_nfs_plan` / `continuous_backup_nfs_size_gb`を新規追加。`continuous_backup_nfs_connect`の説明を「明示指定時は自己プロビジョニングをスキップし、既存/共有NFSを指す」よう更新。
- `terraform/environments/production/main.tf`: `enable_continuous_backup = true`・`database_version = "16"`(要実機確認の暫定値、`os_type`と同種の注記を付記)・`continuous_backup_nfs_ip_address = cidrhost(module.network.internal_cidr, 12)`を追加し、production環境でPITRを既定有効化した。staging環境は変更なし(単一・非冗長ノードのため日次バックアップで十分、という既存判断を踏襲)。

**アプリサーバのディスクバックアップ(新規)**

- `terraform/modules/app_server/main.tf`: `sakuracloud_auto_backup.boot`を追加。各アプリサーバのboot diskを対象に週次スナップショットを取得する。
- `terraform/modules/app_server/variables.tf`: `auto_backup_weekdays`(既定`["sun"]`)・`auto_backup_max_generations`(既定`2`)を新規追加。
- 対象をディスク全体としたのは、`.env`・`monitoring.env`・TLS秘密鍵といった「Git・Terraformいずれの管理下にもなく他にコピーが存在しないファイル」をまとめて保護する目的で、新たな専用バックアップの仕組み(暗号化・転送先・スケジューラの設計)を追加で運用するより、既存の宣言的リソースをそのまま使う方がこのプロジェクトの一貫した判断(Grafana Cloud・GHCR等と同様、自前運用対象を増やさない)に沿うと判断したため。

**ドキュメント**

- `docs/backup-restore-design.md`(新規): 本フェーズの設計書。バックアップ対象の棚卸し、PITR用NFS自己プロビジョニングの設計、リハーサル結果、アプリサーバのディスクバックアップ設計、必要な変数一覧、残課題をまとめた。
- `docs/database-design.md`: 6節(PITR)・8節(復元runbook、8.2節PITR復元手順を新規記載)・9〜10節(残課題)を本フェーズの内容に合わせて更新。
- `README.md`: 「バックアップと復元」節を新規追加。

### 変更ファイル

- `terraform/modules/database/main.tf` / `variables.tf` — `sakuracloud_nfs.pitr`自己プロビジョニング
- `terraform/environments/production/main.tf` — PITR有効化
- `terraform/modules/app_server/main.tf` / `variables.tf` — `sakuracloud_auto_backup.boot`
- `docs/backup-restore-design.md`(新規)
- `docs/database-design.md` — バックアップ関連節の更新
- `README.md` — 「バックアップと復元」節を追加

### 検証結果

- **PostgreSQLバックアップ・復元の実機リハーサル**(ローカルの開発用Postgres、`POSTGRES_PORT=15432`、フェーズ5と同じ回避策): `go run ./cmd/api`を実際に起動してマイグレーションを適用 →実際のAPI経由(`POST /api/auth/register`・`POST /api/missions`)でテストユーザー・ミッションを作成 → `pg_dump -Fc`でバックアップ取得(usersが41件、mission_templatesが26件だった時点、セッション内の既存テストデータ込み) → `pg_terminate_backend` → `DROP DATABASE` → `CREATE DATABASE`でデータを完全に消去し`\dt`で0テーブルになったことを確認 → `pg_restore --no-owner`で復元 → 行数が41件/26件で完全一致することを確認 → 手順で登録した特定のユーザー・ミッション(`backup-drill@example.com`)が実際に復元されていることを確認 → `schema_migrations`テーブルの整合性を確認 → **復元後のDBに対して実際にバックエンドを起動**し、`/health/ready`が200を返すこと、当該ユーザーで実際に`POST /api/auth/login`できること、`GET /api/me`が正しいメールアドレスを返すことを確認。検証に使ったバックアップファイルはリハーサル後に削除済み。
- **Terraform**: `terraform fmt -check -recursive`(差分なし)、`terraform validate`(staging/production両方)成功、`terraform plan`(ダミー値使用、staging/production両方)で`sakuracloud`プロバイダ側の`AccessToken is required`により想定通り停止することを確認(新規追加した`sakuracloud_nfs`・`sakuracloud_auto_backup`を含め、新規のエラー・警告が発生しないことを確認)、`.tfstate`が生成されていないことを確認した。
- **バックエンド回帰確認**: `gofmt -l .`(差分なし)・`go build ./...`・`go vet ./...`・`go test ./... -race`すべて成功(本フェーズでのコード変更なし、リハーサルで使った`go run`のみ)。
- フロントエンド: 本フェーズでフロントエンドのコード変更はなし。

### セキュリティ上の確認

- 自己プロビジョニングした`sakuracloud_nfs`はDBアプライアンスと同じく内部スイッチのみに接続し、公開NICを持たない。
- `sakuracloud_auto_backup`はディスクのスナップショットを取得するだけで、新たな認証情報・ネットワーク経路を追加しない。
- 本フェーズで追加した変数に秘密情報はない(`database_version`・IPアドレス・スケジュール設定のみ)。
- ローカルのリハーサルで使用したテストデータ(メールアドレス等)は開発用Postgres上でのみ扱い、バックアップファイル自体もリハーサル後に削除済み。
- さくらのクラウードの有料リソース作成、`terraform apply`、Git commit・push、破壊的操作は一切実施していない(ローカル開発用Postgresに対する`DROP DATABASE`は、リハーサルの一部として意図的に実行したものであり、対象は本フェーズ専用に起動した使い捨ての開発用コンテナ)。

### 残っている課題

- アプライアンス自体の復元操作(さくらのクラウードのコントロールパネル/API)の実機リハーサル(フェーズ17)
- Terraform stateのリモートbackend移行(フェーズ7から継続。本フェーズのスコープには含めなかった — 移行自体が先に必要な別の課題のため)
- `database_version = "16"`の実際のカタログ確認
- PITR復元時、DBアプライアンスが新規アプライアンスとして起こされた場合の`continuous_backup`再設定手順の確定
- `sakuracloud_database_read_replica`がフェイルオーバー用途の冗長構成に使えるかの調査(未着手)
- アプリサーバのディスクスナップショットからの実際の復元手順の確定(フェーズ17)
- `matsu0122.com`ゾーンの管理場所確認、DNS-01チャレンジ用certbotプラグイン選定、証明書更新自動化(フェーズ11から継続)
- アプリサーバ内部NICの静的IP割り当て(フェーズ8から継続)

### 次のフェーズ

フェーズ16「セキュリティ確認」に進み、ここまでの全フェーズを通したセキュリティ設計の棚卸し(認証・認可、秘密情報管理、ネットワーク境界、依存パッケージの脆弱性、コンテナイメージ等)と、必要な追加対策を実施する。

---

## フェーズ16:セキュリティ確認(完了)

さくらのクラウードの有料リソース作成・`terraform apply`は本フェーズでも一切行っていない。一方、依存パッケージ・コンテナイメージのスキャンは実際のツール(`govulncheck`・`trivy`・`npm audit`)で、認証フロー(Cookie・レート制限)は実際のnginx+TLS+APIスタックに対する`curl`で検証した。詳細・全項目は`docs/security-review.md`にまとめた。本節はサマリ。

### 調査結果

- サブエージェントに認証・認可・秘密情報管理・ネットワーク境界の4領域を実コード(ドキュメントではなく)から棚卸しさせ、事実ベースの報告を得た。主な発見: ログイン・登録エンドポイントにレート制限が一切なかったこと、`terraform.tfvars`が`.gitignore`されていなかったこと(内容自体は非秘密だが取りこぼし)、ホストの`ufw`ルールがパケットフィルタと独立に「どこからでも許可」のブランケットルールになっていたこと。
- `govulncheck`を実際に実行し、`github.com/jackc/pgx/v5`にコードから到達可能な**SQLインジェクション脆弱性(GO-2026-5004)**が実在することを発見した(アプリ側のクエリの書き方では回避不可能な、ドライバ自体のバグ)。
- `trivy`でコンテナイメージをスキャンしたところ、脆弱性以前に**両Dockerfileのベースイメージが既にEOL(Docker Hub上でフローティングタグ自体が削除済み)**になっていることが判明した(`golang:1.23-alpine`・`alpine:3.20`・`node:20-alpine`・`nginxinc/nginx-unprivileged:1.27-alpine`)。フェーズを重ねる中で一度もベースイメージのタグを見直していなかった運用上の見落とし。

### 実装内容

**認証・認可の強化**

- Cookieの`SameSite`を`Lax`から`Strict`へ変更(同一オリジンのみのSPAであり、`Lax`が許す挙動を必要とするフローが存在しないため)。
- ログイン・登録エンドポイントへプロセスローカルのレート制限を新規実装(`backend/internal/middleware/rate_limit.go`、クライアントIP単位のトークンバケット、`golang.org/x/time/rate`使用)。

**秘密情報管理**

- `terraform/.gitignore`を`*.auto.tfvars`から`*.tfvars`(より広いパターン)へ変更。既存の`terraform.tfvars`は`terraform.tfvars.example`(追跡対象テンプレート)へリネームし、`.env`/`.env.example`と同じ運用に揃えた。

**ネットワーク境界**

- `terraform/modules/app_server`の起動スクリプトが生成する`ufw`ルールを、パケットフィルタと同じ`admin_ssh_cidrs`/`web_allowed_source_cidrs`変数からCIDR単位で生成するよう変更(ブランケット許可の解消)。production環境は両モジュール呼び出しが同じ`local`を参照するようにし、将来的な乖離自体を構造的に防いだ。
- フロントエンドNginxに`Content-Security-Policy`(`default-src 'self'`)・`Strict-Transport-Security`ヘッダーを新規追加。

**依存パッケージの脆弱性**

- `github.com/jackc/pgx/v5` v5.7.2→v5.9.2(SQLインジェクション修正)、`golang.org/x/crypto` v0.31.0→v0.57.0、`golang.org/x/text` v0.21.0→v0.42.0へ更新。
- Go本体を`1.23`→`1.26`(`go.mod`・`backend/Dockerfile`・CIの`GO_VERSION`)へ更新し、stdlib側の既知脆弱性(`crypto/tls`・`net/http`・`encoding/xml`・`encoding/asn1`)を解消。

**コンテナイメージ**

- `backend/Dockerfile`: `golang:1.23-alpine`→`golang:1.26-alpine`、`alpine:3.20`→`alpine:3.23`。
- `frontend/Dockerfile`: `node:20-alpine`→`node:24-alpine`、`nginxinc/nginx-unprivileged:1.27-alpine`→`1.30-alpine`。

**CIへの継続的スキャンの組み込み**

- `.github/workflows/ci.yml`に`govulncheck`(backendジョブ)・`npm audit --audit-level=high`(frontendジョブ)・`trivy`イメージスキャン(docker-buildジョブ、HIGH/CRITICAL、`ignore-unfixed: true`)を追加し、一度きりの手動スキャンで終わらせず以後のPull Requestで継続的に検証されるようにした。`GO_VERSION`/`NODE_VERSION`も実際のDockerfileと揃えて更新。

### 変更ファイル

`docs/security-review.md`7節に全ファイル一覧を記載。主なもの: `backend/internal/handler/auth_handler.go`、`backend/internal/middleware/rate_limit.go`(新規)、`backend/internal/apperror/apperror.go`、`backend/go.mod`/`go.sum`、`backend/Dockerfile`、`frontend/Dockerfile`、`frontend/nginx-locations.conf`、`terraform/.gitignore`、`terraform/environments/{staging,production}/terraform.tfvars.example`(リネーム)、`terraform/modules/app_server/*`、`terraform/environments/{staging,production}/main.tf`、`.github/workflows/ci.yml`、`docs/security-review.md`(新規)、`README.md`。

### 検証結果

- バックエンド・フロントエンドの回帰確認(build/vet/test/lint/tsc)すべて成功(詳細は`docs/security-review.md`8節)。
- `govulncheck`: 修正前コードから到達可能な脆弱性6件→修正後0件(stdlib分は実際にCIが使う`go1.26.8`で確認)。`npm audit`: 0件。
- `trivy`: 修正前はEOLベースイメージの警告、修正後backend/frontend両方でHIGH/CRITICAL 0件。
- レート制限: 単体テストに加え、実際のNginx+TLS+APIスタックへ6回連続ログインを送り1〜5回目401・6回目429を実機確認。
- CSP: ビルド後のバンドル内容を全数確認し外部参照が皆無であることを根拠に採用。実ブラウザでのコンソールエラー確認は、Chromeの自己署名証明書インタースティシャルの自動操作制限により未実施(残課題)。
- Terraform: `fmt`/`validate`/`plan`(両環境、新規のエラー・警告なし)、`.gitignore`修正を`git check-ignore`で確認、`ufw`ルールのレンダリング結果を`shellcheck`で確認(新規警告0件)。

### セキュリティ上の確認

本フェーズの成果物そのものがセキュリティ確認であるため、詳細は`docs/security-review.md`各節を参照。さくらのクラウードの有料リソース作成、`terraform apply`、Git commit・push、破壊的操作は一切実施していない。

### 残っている課題

`docs/security-review.md`9節に記載。主なもの: セッション管理(30日固定)の見直し、レート制限の共有ストア化(現状はプロセスローカル)、CSPの実ブラウザ確認、CIスキャンの実際のGitHub Actions実行確認(ワークフロー未push)。フェーズ7・8・11・13・14・15からの既存の継続課題は変わらず未解消。

### 次のフェーズ

フェーズ17「障害試験・負荷試験」に進み、片系停止試験、DBフェイルオーバー/リストア演習、ロールバック演習、負荷試験(ボトルネック調査を含む)を実施する。

---

## フェーズ17:障害試験・負荷試験(完了)

さくらのクラウードの有料リソース作成・`terraform apply`は本フェーズでも一切行っていない。アプリサーバ・DBアプライアンス・ロードバランサいずれも実在しないため、実機での障害試験はまだ実施できない。本フェーズでは、ローカルのDocker環境に本番構成へできる限り近いシミュレーションを組み、実際のツール(`hey`によるHTTP負荷生成、実際のPostgres・Nginx・アプリケーションコード)で検証した。詳細・実機との違い・残課題は`docs/load-test-results.md`にまとめた。本節はサマリ。

### 調査結果

- さくらのクラウードLBの能動ヘルスチェックはローカルで再現できないため、オープンソース版Nginxの受動ヘルスチェック(`proxy_next_upstream`)で片系停止試験を代替した。
- DBフェイルオーバー(レプリカへの昇格)は実機・レプリカ双方が存在せず試験不能なため、「DBが完全に消える/戻るという障害にアプリケーションがどう振る舞うか」に絞って検証した(リストア演習自体はフェーズ15で実施済み)。
- 実際のCD(GitHub Actions)は未実行のため、ロールバック演習は`deploy.sh`の実際のロジック(`pull`以外)をローカルで再現する形で実施した。
- **負荷試験で、`GET /api/missions`が同時接続40〜50でスループットが105 req/sまで崩壊する重大なボトルネックを発見した。** 原因は`TemplateRepo.ListByUser`が外側クエリの`*sql.Rows`を開いたまま内側クエリ(スケジュール曜日取得)をループ内で発行しており、コネクションプール(既定20)が同時接続増加時に自己枯渇していたため。

### 実装内容

**片系停止試験**: 2つの`api`コンテナ+Nginx(受動ヘルスチェック)をローカルに構築し、継続的な負荷生成中に片方を`docker kill`。20万件超のリクエストで失敗0件、片系再起動後の復帰も確認。

**DB障害試験**: 実行中のアプリケーションに対しDBコンテナを`docker stop`/`docker start`。`/health/ready`の503↔200遷移、`/health/live`が影響を受けないこと、アプリケーションプロセスがクラッシュしないこと、DB復旧後の自動再接続(アプリ再起動不要)を確認。

**ロールバック演習**: 現行コード(v1)をデプロイ → 意図的な退行(ログインハンドラが常に500を返す、演習専用のスクラッチコピーのみに注入、実リポジトリ未変更)を仕込んだv2をデプロイ → ヘルスチェックは合格するが実際には壊れていることを確認 → `IMAGE_TAG`をv1へ戻して再デプロイし復旧を確認。

**負荷試験・ボトルネック修正**:

- `backend/internal/repository/template_repo.go`の`ListByUser`を、外側クエリを閉じてから全テンプレート分のスケジュール曜日を1回のクエリ(`= ANY($1)`、pgxドライバのネイティブ配列パラメータ)でまとめて取得しグルーピングする方式へ修正。1+Nクエリが常に2クエリになり、コネクション保持の重なりも解消。
- 修正後、同一条件(同時接続50)で67,011リクエスト・失敗0・p99 12.4msまで回復(修正前は105 req/s・大半504)。
- `backend/internal/handler/router_integration_test.go`に`TestListMissions_EachTemplateGetsItsOwnScheduleDays`(グルーピングの正しさ)・`TestListMissions_Concurrent`(同時接続100での回帰防止、修正前コードで実際に失敗することを確認済み)を追加。
- 他の認証済みエンドポイント(`/api/missions/today`・`/api/stats`・`/api/missions/history`)をスポットチェックし、同様の問題がないことを確認。

### 変更ファイル

- `backend/internal/repository/template_repo.go` — コネクションプール枯渇バグの修正
- `backend/internal/handler/router_integration_test.go` — 回帰テスト2件追加
- `docs/load-test-results.md`(新規)
- `README.md` — 「障害試験・負荷試験」節を追加

### 検証結果

`docs/load-test-results.md`6節に詳細。バックエンド回帰確認(`gofmt`/`go build`/`go vet`/`go test -race`/統合テスト全9件)すべて成功。修正前のコードへ一時的に戻して新規テストが実際に失敗すること、修正を戻すと成功することを確認済み(フェーズ13の migrate レース検証と同じ徹底度)。すべての検証用Dockerコンテナ・イメージ・スクラッチファイルは削除済み。

### セキュリティ上の確認

本フェーズで発見・修正したバグは性能上の問題であり認可・認証には影響しない。ロールバック演習の「意図的な退行」はスクラッチ領域のみで扱い、実リポジトリには一切含まれていない。さくらのクラウードの有料リソース作成、`terraform apply`、Git commit・push、破壊的操作は一切実施していない。

### 残っている課題

`docs/load-test-results.md`8節に記載。主なもの: 実際のさくらのクラウードLB・DBアプライアンス・CDパイプラインでの試験(実インフラ構築後)、ヘルスチェックだけでは検知できない退行への対策(デプロイ後スモークテスト等、フェーズ18で検討)、Grafana Cloud側のアラートが同様の性能崩壊を検知できるかの確認(アカウント未作成のため未検証)。

### 次のフェーズ

フェーズ18「運用手順書と公開判定」に進み、これまでの全フェーズの成果を踏まえた運用手順書の整備と、本番公開の可否判定を行う。

---

## フェーズ18:運用手順書と公開判定(完了)

さくらのクラウードの有料リソース作成・`terraform apply`は本フェーズでも一切行っていない。本フェーズは主にフェーズ1〜17の成果を統合するドキュメント作成フェーズだが、その過程で1件、実装済みの修正を行った(調査結果参照)。

### 調査結果

- 全17フェーズの「残っている課題」節を横断的に収集・整理した(`docs/production-roadmap.md`各フェーズ)。多くは「実際のさくらのクラウードリソースが存在しないと検証・実施できない」性質のものに収斂することが分かった(Terraform stateのリモートbackend移行、DB冗長構成、TLS証明書自動化、GitHub Secretsの設定等)。
- **`sessions`テーブルの期限切れレコードをクリーンアップする仕組みが、フェーズ2で課題として指摘されたまま、フェーズ3〜5の「残っている課題」に3回引き継がれた後、フェーズ6以降は一度も言及されなくなっていた** ことを発見した。実際にコードを確認したところ、指摘通り該当する仕組みは一切実装されておらず、`sessions`テーブルは無制限に増え続ける設計のままだった。「トラッキングリストから消えた」だけで「解決した」わけではない項目が実在することを示す具体例であり、本フェーズで実装した(実装内容参照)。

### 実装内容

**`sessions`テーブルの自動クリーンアップ(新規)**

- `backend/internal/repository/session_repo.go`: `DeleteExpired(ctx, now)`を追加(`DELETE FROM sessions WHERE expires_at < $1`)。
- `backend/internal/service/interfaces.go` / `auth_service.go`: `SessionRepository`インターフェースに`DeleteExpired`を追加し、`AuthService.CleanupExpiredSessions(ctx)`を新設。
- `backend/internal/service/session_cleanup.go`(新規): `RunSessionCleanupLoop`が起動直後に1回、以後1時間ごとにクリーンアップを実行するバックグラウンドループ。`cmd/api/main.go`のシャットダウンcontextに連動し、SIGTERM等で自然に停止する。
- 環境変数化はしなかった(DBプール設定等と異なり、クリーンアップ間隔に環境依存の「正解値」がないため)。

**運用ドキュメントの新規作成**

- `docs/operations-runbook.md`(新規): システム構成図、初回構築手順(ゼロから公開までの手順を、これまでの17フェーズの手動対応事項を集約して時系列で整理)、日常デプロイ、ロールバック、障害対応(アプリサーバ障害・DB障害・デプロイ後の不具合・秘密情報漏洩)、監視・アラート対応、定期メンテナンス、既知の制約一覧、緊急時のroot操作。
- `docs/go-live-readiness.md`(新規): 公開判定書。判定結果(条件付きGO)、完了していることの要約、公開前必須の手動対応チェックリスト、公開直後に対応すべき事項、長期的な改善事項、判定の限界(実インフラでの未検証部分)を明記。

**README更新**: 「運用手順書と公開判定」節を新規追加。フェーズ1以前から残っていた「今後の拡張候補」節の記述(レート制限・HTTPS終端が未実装であるかのような古い記述)を、実際には実装済みであることが分かるよう修正。

### 変更ファイル

- `backend/internal/repository/session_repo.go` — `DeleteExpired`追加
- `backend/internal/service/interfaces.go` / `auth_service.go` — `SessionRepository`拡張、`CleanupExpiredSessions`追加
- `backend/internal/service/session_cleanup.go`(新規) / `session_cleanup_test.go`(新規)
- `backend/internal/service/auth_service_test.go`(新規) — `AuthService`初の単体テスト(クリーンアップロジック)
- `backend/internal/service/fakes_test.go` — `SessionRepository`用フェイク追加
- `backend/internal/repository/session_repo_test.go`(新規) — 実DBでの`DeleteExpired`検証
- `backend/cmd/api/main.go` — クリーンアップループの起動配線
- `docs/operations-runbook.md`(新規)
- `docs/go-live-readiness.md`(新規)
- `README.md` — 「運用手順書と公開判定」節を追加、古い記述の修正

### 検証結果

- バックエンド回帰確認: `gofmt -l .`(差分なし)・`go build ./...`・`go vet ./...`・`go test ./... -race`(新規テスト5件を含め全パッケージ成功)・統合テスト(実DB、`-race`、全9件)すべて成功。
- 新規テスト: フェイクリポジトリによる`CleanupExpiredSessions`のユニットテスト(期限切れのみ削除・期限切れなしはno-op)、バックグラウンドループの起動直後の実行と`ctx`キャンセル時の停止、**実Postgresに対する`DeleteExpired`の実行**(期限切れセッションのみ削除され有効なセッションは残ることを確認)。
- 実際にアプリケーションを起動し、クリーンアップループがエラーなく起動すること(ログにエラーが出ないこと)を確認。

### セキュリティ上の確認

`DeleteExpired`は`expires_at`のみで絞り込む単純な削除であり、認可・所有者チェックは関係しない(セッションは既にトークンハッシュでのみ照合される設計)。本フェーズで新たな秘密情報の取り扱いは発生していない。さくらのクラウードの有料リソース作成、`terraform apply`、Git commit・push、破壊的操作は一切実施していない。

### 残っている課題

`docs/go-live-readiness.md`2〜4節、`docs/operations-runbook.md`8節に集約した。要約すると、残る課題はすべて「実際のさくらのクラウードリソースの存在」を前提とするものであり、コードレベルでこれ以上先に進められる項目はない。

### 次のフェーズ

なし。ロードマップの全18フェーズが完了した。次の一歩は、`docs/go-live-readiness.md`2節のチェックリストに沿って、ユーザー自身が実際のさくらのクラウードリソースを構築することである。

## フェーズ19:公開前の残課題のうちコードで解消できるもの(完了)

フェーズ18の`docs/go-live-readiness.md`・`docs/operations-runbook.md`8節に残っていた課題のうち、実在の認証情報・費用・ドメイン操作を伴わずに進められる3件を実装した。

### 実装内容

1. **デプロイ後の自動スモークテスト**(`docs/load-test-results.md`3節の課題): `.github/scripts/smoke-test.sh`を新規作成し、`deploy-host.sh`が`/health/ready`合格後に各サーバー上で実行する。データを書き込まない確認だけで構成したので、productionでも安全に実行できる。
2. **TLS証明書の自動更新**(`docs/tls-design.md`4節の課題): certbot公式の`certbot-dns-sakuracloud`(Ubuntu 24.04の`python3-certbot-dns-sakuracloud`)に確定。`certbot.timer`の有効化と、証明書を`/opt/levelog/tls/`へ差し替えてNginxをリロードするデプロイフックを、起動スクリプトに追加した。外形監視に証明書の残り日数チェック(`sslcertificate`、14日未満で通知)も追加した。
3. **Grafana Cloudのアラートルールのコード化**(`docs/monitoring-design.md`6節の課題): `monitoring/alerts/levelog.rules.yml`(7ルール)と`promtool`の単体テストを追加し、CIに`monitoring-rules`ジョブを追加した。
4. **調査中に見つけた既存の不具合の修正**: Alloyの設定に、サーバーを区別するラベルがなかった(全サーバーが`127.0.0.1`をスクレイプするため`instance`が同じになり、production 2台の系列が衝突する)。`host = constants.hostname`をメトリクス・ログの両方に追加した。
5. CIに`shellcheck`ジョブを追加した(これまでローカルでのみ実行していた)。

### 検証結果

- スモークテスト: 本番構成(`docker-compose.prod.yml`、自己署名証明書)をローカルで起動し、全12項目が合格することを確認した。さらにフェーズ17と同じ「ログインが常に500を返す」退行を仕込んだビルドでは、`/health/ready`が200のままでもスモークテストが失敗(終了コード1)することを確認した。検証中に、接続失敗時にステータスが`000000`と表示される不具合を見つけて修正した。
- 証明書更新フック: 同じ環境で、別の証明書を`RENEWED_LINEAGE`としてフックを実行し、コンテナを再起動せずに配信される証明書が切り替わること、鍵がuid 101・0400で置かれてもNginxが読めることを確認した。
- Alloy: 修正後の設定を`grafana/alloy:v1.19.2`で実際に起動し、全7コンポーネントがhealthyであることを確認した。
- アラートルール: `promtool check rules`・`promtool test rules`(9テストケース、すべて合格)・`mimirtool rules check`が成功。閾値を変えた版・`or vector(0)`を外した版でテストが失敗することも確認した。
- Terraform: `terraform fmt -check`が成功。`sacloud/sakuracloud`プロバイダ(v2.36.1)をソースからビルドし、変更した`monitoring`・`app_server`モジュールで`terraform validate`が成功した(この環境からはTerraformレジストリに接続できないため)。`remaining_days = 0`で範囲外エラーになることから、スキーマが実際に検査されていることも確認した。起動スクリプトのテンプレートを`templatefile()`でレンダリングし、`bash -n`・`shellcheck`で確認した(警告は既存のnode_exporter部分の1件のみ)。
- `actionlint`・`shellcheck`(デプロイ・スモークテスト・証明書フックのスクリプト)が成功。バックエンドの`go test ./...`が成功。
- 検証環境ではIPv6が無効なため、Nginxの`listen [::]`を検証用のコピーでだけ外して起動した(本番のUbuntuでは問題にならない。リポジトリのファイルは変更していない)。

### セキュリティ上の確認

さくらのクラウードのAPIキー(証明書発行用)は`/etc/letsencrypt/sakuracloud.ini`(0600)に手動で置く方針とし、起動スクリプト・Terraform・CIのいずれにも持たせない。スモークテストはデータを書き込まず、ログインの確認には予約済みTLD `.invalid`のアドレスを使うので、実在のアカウントに当たることはない。Grafana CloudのAPIキーとSlackのWebhook URLはリポジトリに置かない(`mimirtool`の引数とGrafana CloudのUIで直接扱う)。`terraform apply`・DNS変更・クラウドリソースの作成は、このフェーズでも実施していない。

### 残っている課題

`docs/go-live-readiness.md`2節の手動チェックリストは変わらない(アカウント・APIキー・SSH鍵・`terraform apply`・秘密情報の投入・DNS・GitHub Secrets)。これに加えて、証明書の初回発行(`docs/operations-runbook.md`2.3節)とアラートルールのGrafana Cloudへの登録(`docs/monitoring-design.md`6.1節)が、アカウント作成後に1回ずつ必要。証明書の自動発行は、`matsu0122.com`ゾーンがさくらのクラウードDNSで管理されていることが前提。

---

ここでロードマップの全フェーズが完了しました。実際の公開作業は`docs/go-live-readiness.md`と`docs/operations-runbook.md`を参照してください。
