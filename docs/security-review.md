# セキュリティ確認

最終更新: 2026-09-23(フェーズ16)

このドキュメントは、フェーズ1〜15を通して積み上げてきたセキュリティ設計の棚卸しと、本フェーズで発見・修正した項目をまとめる。対象は認証・認可、秘密情報管理、ネットワーク境界、依存パッケージの脆弱性、コンテナイメージの5領域。

**実際のさくらのクラウードの有料リソース作成・`terraform apply`は本フェーズでも一切行っていない。** 一方、依存パッケージ・コンテナイメージのスキャン、認証フローの実機検証(実際のnginx+TLS+APIスタックに対する`curl`)は、いずれも実際のツール(`govulncheck`・`trivy`・`npm audit`・実ブラウザ)で行った。

## 1. 認証・認可

### 現状(棚卸し)

- パスワードハッシュ: bcrypt(`bcrypt.DefaultCost`)。
- セッショントークン: `crypto/rand`による32バイトのランダム値を16進エンコード(64文字)。サーバーはトークンそのものではなく`sha256`ハッシュのみを保存する。
- セッション有効期限: 30日固定、アクセスの都度の延長(スライディング)や再発行は行っていない。
- ログアウト: サーバー側でセッションレコードを実際に削除しており、Cookieを消すだけの見せかけではない。
- 認可: すべてのハンドラがリクエストボディ/パスのユーザーIDを信用せず、`RequireAuth`ミドルウェアがセッションから解決した認証済みユーザーIDのみを使う。全リポジトリのクエリが`WHERE id = $1 AND user_id = $2`のように所有者で必ず絞り込む設計になっている。`TestCrossUserAuthorization`(統合テスト)が実際のHTTP API経由で、他ユーザーのリソースへの操作が404になること・不正操作後も自分のリソースは正常に操作できることを検証している。

### 本フェーズで変更した項目

- **Cookieの`SameSite`を`Lax`から`Strict`へ変更**(`backend/internal/handler/auth_handler.go`)。このアプリはSPA+同一オリジンAPIプロキシ(`frontend/nginx-locations.conf`)のみで、Cookieを必要とするクロスサイトのナビゲーションフローが存在しないため、`Lax`が許していた「トップレベルのクロスサイトGETナビゲーションでもCookieを送る」という挙動を維持する理由がなく、より強いCSRF対策になる`Strict`に変更した。
- **ログイン・登録エンドポイントへのレート制限を新規追加**(`backend/internal/middleware/rate_limit.go`)。クライアントIP(nginxが設定する`X-Forwarded-For`の先頭値、ヘッダーがなければ`RemoteAddr`)ごとのトークンバケット。ログインは5回/6秒間隔で補充(ブルートフォース対策として厳しめ)、登録は10回/10秒間隔(スパム登録対策)。ブルートフォース・スパム登録への対策が一切なかったギャップを埋めた。
  - production環境(アプリサーバ2台)では各インスタンスが独立してこの制限を持つため、2台に分散されたリクエストに対する実効上限は単一インスタンスの約2倍になる。共有ストア(Redis等)を新たに運用してまで厳密な統一上限にする価値は現状の規模では低いと判断し、プロセスローカルな実装にとどめた。
  - アイドルになったエントリ(10分間アクセスがないIP)は5分おきに掃除しており、プロセスメモリが無制限に増え続けることはない。
  - **実機検証**: 3節参照。

### 変更しなかった項目(意図的な判断)

- セッション有効期限30日・スライディング延長なしは、本フェーズでは変更していない。ユーザー体験(頻繁な再ログインを求めない)とのトレードオフであり、単純な「短くする」判断が常に正しいとは限らないため、次にセッション管理を見直す機会(フェーズ18の運用手順書、または実際の悪用兆候が見つかった場合)まで現状維持とする。
- 2要素認証・CAPTCHAは導入していない(MVPの規模に対して過剰と判断)。

## 2. 秘密情報管理

### 現状(棚卸し)

- `.env`(ルート/backend/frontend)は`.gitignore`で除外済み、追跡されていないことを確認した。`.env.example`はプレースホルダのみ。
- DBパスワード・Slack Webhook等、Terraformの秘密情報系変数はすべて`sensitive = true`かつデフォルト値なし。
- GHCR認証は`secrets.GITHUB_TOKEN`(ジョブ実行中のみ有効)、デプロイ用SSH鍵は`secrets.DEPLOY_SSH_KEY`(デフォルトなし)。
- Grafana Cloud認証情報はサーバー上の`/etc/levelog/monitoring.env`(Terraform/CIが作成しない、手動投入)。
- TLS秘密鍵は`TLS_CERT_DIR`経由でマウントするのみで、リポジトリには一切存在しない。

### 本フェーズで発見・修正した項目

- **`terraform/environments/{staging,production}/terraform.tfvars`が`.gitignore`されていなかった。** `terraform/.gitignore`は`*.auto.tfvars`のみを除外しており、プレーンな`terraform.tfvars`はパターンに一致していなかった(`docs/staging-environment.md`フェーズ6時点の設計意図では`*.tfvars`を広く除外する想定だったが、フェーズ7の実装が`*.auto.tfvars`に留まっていた取りこぼし)。現在の内容は非秘密情報のみで実害はなかったが、将来誰かが一時的に秘密値を書き込んでから`git add -A`した場合に検知する仕組みが何もなかった。
  - 修正: `terraform/.gitignore`のパターンを`*.tfvars`(`*.auto.tfvars`を包含するより広いパターン)へ変更。
  - 既存の`terraform.tfvars`(非秘密値のみ)は`terraform.tfvars.example`(追跡対象のテンプレート)へリネームし、`.env`/`.env.example`と同じパターンに揃えた。実際に使う場合は`terraform.tfvars`へコピー(`.gitignore`済み)するか、`-var`/`TF_VAR_*`で直接値を渡す。
  - `terraform/README.md`・`docs/staging-environment.md`の該当記述を更新。

## 3. ネットワーク境界

### 現状(棚卸し)

3層構成: さくらのクラウードのパケットフィルタ(`terraform/modules/network`)、ホストの`ufw`(起動スクリプト)、アプリケーションレベルのルーティング(Nginx)。DBアプライアンス・PITR用NFS(フェーズ15)は非ルーティングの内部スイッチのみに接続し、そもそもインターネットから到達不能。`api`コンテナはホストにポート公開されておらず(`/metrics`のみループバック公開、フェーズ14)、node_exporter・Grafana AlloyのUIもループバックのみ。

### 本フェーズで発見・修正した項目

- **ホストの`ufw`ルールとさくらのクラウードのパケットフィルタが、別々のファイル・別々の言語で独立に保守されており、両者が食い違いうる状態だった。** 具体的には、`ufw allow 22/tcp`のように送信元を一切制限しないブランケット許可になっており、パケットフィルタ側で`admin_ssh_cidrs`/`web_allowed_source_cidrs`をどれだけ絞り込んでも、ufw側は「どこからでもそのポートへ到達可能」なままだった(パケットフィルタが正しく機能していれば実害はないが、多層防御の趣旨に反する)。
  - 修正: `terraform/modules/app_server`の起動スクリプトが生成する`ufw`ルールを、`admin_ssh_cidrs`/`web_allowed_source_cidrs`(パケットフィルタと同じ変数)からCIDR単位で生成するよう変更(`ufw allow from <CIDR> to any port <PORT> proto tcp`)。production環境では、この値をネットワークモジュール呼び出しと同じ`local`から取得するようにし、2箇所が別々の式を持って将来ズレる可能性自体を排除した。
  - 検証: `templatefile()`で実際にレンダリングし、意図した`ufw allow from`行が生成されることを確認。`shellcheck`で新規の警告が発生していないことを確認(4節参照)。

### フロントエンドのHTTPセキュリティヘッダーを追加

既存のセキュリティヘッダー(`X-Content-Type-Options`・`X-Frame-Options`・`Referrer-Policy`)に加え、本フェーズで以下を追加した(`frontend/nginx-locations.conf`)。

- **`Content-Security-Policy: default-src 'self'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'; object-src 'none'`**。ビルド後の`dist/index.html`・バンドルされたJS/CSSを実際に確認し、インラインスクリプト/スタイルが一切なく、外部オリジンへの参照(フォント・CDN・解析ツール等)も一切ないこと(バンドル中に見つかった`http(s)://`文字列はすべてReact/React Routerのエラーメッセージ文字列やXML名前空間URIであり、実際に読み込まれるリソースではないことを確認済み)を根拠に、最も厳しい`default-src 'self'`を採用した。`VITE_API_BASE_URL`をクロスオリジンに設定する(`docker-compose.prod.yml`がドキュメント化している非既定の使い方)場合は、このポリシーにも`connect-src`の追加が必要になる旨をコメントに明記した。
- **`Strict-Transport-Security: max-age=63072000`**(`includeSubDomains`・`preload`は付けない — `matsu0122.com`の他のサブドメインがHTTPS対応済みとは限らないため、また`preload`は事実上取り消せない登録のため)。

**実機検証**: `docker-compose.prod.yml`相当の構成(`web`+`api`+開発用DB、自己署名TLS証明書)をローカルで実際に起動し、`curl -k https://localhost:.../ `で上記ヘッダーがすべて実際に返ることを確認した。同じスタックに対し、実際のAPI経由でユーザー登録→Cookie発行→`GET /api/me`が正しく機能すること、およびログインエンドポイントへ6回連続でリクエストを送ると1〜5回目は401(パスワード誤り)、6回目は429(レート制限)を返すことを、Nginx経由(`X-Forwarded-For`が実際に付与される経路)で確認した — レート制限がGoの単体テストの中だけでなく、実際のリバースプロキシ構成でも機能することの裏付け。CSPについては、Chromeの自己署名証明書インタースティシャル(セキュリティ警告)ページがChrome DevTools Protocolでの自動操作をブロックする仕様上、実ブラウザでのコンソールエラー確認までは行えなかった(既知の制約として記録)。上記の静的解析(バンドル内容の完全な確認)で十分な根拠が得られたと判断した。

## 4. 依存パッケージの脆弱性

`govulncheck`(Go公式のツール)を実際にインストールして`./...`に対して実行した。

### 発見内容(修正前)

コードから実際に到達可能な脆弱性が6件見つかった。

| 脆弱性 | モジュール/stdlib | 内容 |
| --- | --- | --- |
| GO-2026-5004 | `github.com/jackc/pgx/v5` v5.7.2 | **ダラー引用文字列リテラルとのプレースホルダ混同によるSQLインジェクション**。`v5.9.2`で修正 |
| GO-2026-5970 | `golang.org/x/text` v0.21.0 | 不正な入力による無限ループ(DoS)。`db.Open`経由で到達 |
| GO-2026-6090/6089/6088/5972 | Go標準ライブラリ(`crypto/tls`・`net/http`・`encoding/xml`・`encoding/asn1`) | いずれも`go1.26.6`で修正済み |

`github.com/jackc/pgx/v5`のSQLインジェクションは、本プロジェクトのすべてのクエリがプレースホルダ(`$1`等)を使っている(生のSQL文字列結合をしていないこと自体は別途確認済み、後述)としても、**ドライバ自体のプレースホルダ解釈にバグがある**という種類の脆弱性であり、アプリケーションコード側の書き方では回避できない。優先度高で修正した。

### 修正内容

- `github.com/jackc/pgx/v5` `v5.7.2` → `v5.9.2`
- `golang.org/x/crypto` `v0.31.0` → `v0.57.0`(直接到達する脆弱性はなかったが、bcrypt以外のサブパッケージ(ssh等、本プロジェクトは使用していない)に多数のCVEが蓄積していたため、実害がなくても最新化した)
- `golang.org/x/text` `v0.21.0` → `v0.42.0`(間接依存)
- Go本体: `go.mod`の`go`ディレクティブを`1.23`から`1.26.0`(`go mod tidy`がpgxの要求に従い自動的に引き上げた)、`backend/Dockerfile`のビルドイメージを`golang:1.23-alpine`から`golang:1.26-alpine`、CIの`GO_VERSION`を`1.23`から`1.26`へ変更(理由は5節)。
- 修正後、`govulncheck`をコードから実際に到達可能な脆弱性ゼロまで確認(stdlib分は、CIが実際に取得する最新の`1.26.x`パッチ(`go1.26.8`)で動作確認し、`0`件になることを確認済み — ローカル開発環境の`go`バイナリ自体は`1.26.5`のままだったため、別途`go1.26.8`をダウンロードして検証した)。

### npm audit(フロントエンド)

`npm audit`を実行し、**0件**であることを確認した(追加の修正は不要)。

### CIへの組み込み(新規)

一度の手動スキャンで終わらせず、以後のPull Requestで継続的に検証されるよう、`.github/workflows/ci.yml`に以下を追加した。

- `backend`ジョブ: `govulncheck`を追加。
- `frontend`ジョブ: `npm audit --audit-level=high`を追加。

## 5. コンテナイメージ

`trivy`(Docker Hubから取得した公式イメージ)で、実際にビルドしたbackend/frontendイメージをスキャンした。

### 発見内容(修正前)

脆弱性そのものより先に、**両イメージのベースOS/ランタイムが既にEOL(サポート終了)になっていた**ことが判明した。

| Dockerfile | 変更前 | 問題 |
| --- | --- | --- |
| `backend/Dockerfile`(ビルドステージ) | `golang:1.23-alpine` | Docker Hubから`1.23-alpine`系のフローティングタグ自体が既に削除されており(`1.25`/`1.26`/`1.27`のみ現存)、`docker pull`しても以後セキュリティ更新が一切届かない状態だった |
| `backend/Dockerfile`(実行ステージ) | `alpine:3.20` | `trivy`が明示的に`"This OS version is no longer supported"`と警告 |
| `frontend/Dockerfile`(ビルドステージ) | `node:20-alpine` | Node 20のフローティング`-alpine`系タグも同様に現存しない(`26`のみ) |
| `frontend/Dockerfile`(実行ステージ) | `nginxinc/nginx-unprivileged:1.27-alpine` | `1.27`系タグが現存しない(`1.30`/`1.31`のみ) |

これは`docker-compose.prod.yml`を使う実際のビルド(フェーズ4・11・13で繰り返しローカル検証してきた)が、知らないうちに更新の届かない古いベースイメージの上で行われ続けていたことを意味する。フェーズを重ねるごとに一度もベースイメージのタグを見直していなかった、という運用上の見落としであり、依存パッケージのバージョンと同様に定期的な確認が必要な対象だと分かった。

### 修正内容

- `backend/Dockerfile`: `golang:1.23-alpine` → `golang:1.26-alpine`、`alpine:3.20` → `alpine:3.23`
- `frontend/Dockerfile`: `node:20-alpine` → `node:24-alpine`(現行LTS)、`nginxinc/nginx-unprivileged:1.27-alpine` → `1.30-alpine`
- 両イメージを実際に再ビルドし、`trivy image --severity HIGH,CRITICAL`で再スキャン。EOL警告が消え、**HIGH/CRITICAL 0件**(Goバイナリ自体のスキャン含む)であることを確認した。

### CIへの組み込み(新規)

`.github/workflows/ci.yml`の`docker-build`ジョブに、ビルド済みイメージへの`trivy`スキャン(HIGH/CRITICAL、`exit-code: 1`で失敗させる。`ignore-unfixed: true`— 上流に修正がまだない脆弱性でPRを恒久的にブロックしても誰も対応できないため)を追加した。レジストリへの認証・pushは一切行わず、ローカルでビルドしたイメージをスキャンするのみ。

## 6. その他の確認(grepベース)

以下は個別の追加対策は不要と判断したが、確認した内容を記録する。

- `backend/internal/repository`内の`fmt.Sprintf`は、プレースホルダの位置番号(`$1`等)と固定のカラム名リストの組み立てにのみ使われており、実際の値は常にバインド引数として渡されている(文字列結合によるSQLインジェクションの経路ではない)。
- `frontend/src`に`dangerouslySetInnerHTML`・`eval(`の使用なし。
- backend/frontendのソースコード(テストファイルの明らかなダミー値を除く)にハードコードされた認証情報・APIキーなし。

## 7. 変更ファイル一覧

- `backend/internal/handler/auth_handler.go` — Cookie `SameSite` を `Strict` へ
- `backend/internal/middleware/rate_limit.go`(新規)/ `rate_limit_test.go`(新規)
- `backend/internal/apperror/apperror.go` — `TooManyRequests`追加
- `backend/internal/handler/router.go` — レート制限ミドルウェアの配線
- `backend/go.mod` / `go.sum` — 依存パッケージ更新、`golang.org/x/time`追加
- `backend/Dockerfile` — ベースイメージ更新
- `frontend/Dockerfile` — ベースイメージ更新
- `frontend/nginx-locations.conf` — CSP・HSTSヘッダー追加
- `terraform/.gitignore` — `*.tfvars`パターンへ拡大
- `terraform/environments/{staging,production}/terraform.tfvars.example`(リネーム、新規)
- `terraform/modules/app_server/main.tf` / `variables.tf` / `templates/startup.sh.tftpl` — ufw CIDRスコープ化
- `terraform/environments/{staging,production}/main.tf` — ufw用変数の配線(productionは`local`で一元化)
- `terraform/README.md` / `docs/staging-environment.md` — tfvars運用の記述更新
- `.github/workflows/ci.yml` — `govulncheck`・`npm audit`・`trivy`スキャンを追加、`GO_VERSION`/`NODE_VERSION`更新

## 8. 検証結果まとめ

- **バックエンド回帰確認**: `gofmt -l .`(差分なし)・`go build ./...`・`go vet ./...`・`go test ./... -race`・統合テスト(実DB、`-race`)すべて成功。
- **フロントエンド回帰確認**: `npx tsc -b`・`npm run lint`・`npm run test -- --run`(20ケース)・`npm run build`すべて成功。
- **`govulncheck`**: 修正前は到達可能な脆弱性6件、修正後0件(stdlib分は実際にCIが使う`go1.26.8`で確認)。
- **`npm audit`**: 0件。
- **`trivy`(コンテナイメージ)**: 修正前はEOLベースイメージによる警告あり、修正後backend/frontend両方でHIGH/CRITICAL 0件。
- **レート制限**: 単体テスト(`rate_limit_test.go`)に加え、実際のNginx+TLS+APIスタックに対して6回連続ログインを送り、1〜5回目401・6回目429を実機で確認。
- **Terraform**: `terraform fmt -check -recursive`(差分なし)、`terraform validate`(staging/production両方)成功、`terraform plan`(ダミー値、両環境)で新規のエラー・警告なく`AccessToken is required`で想定通り停止することを確認、`.tfstate`が生成されていないことを確認。
- **`ufw`ルールのレンダリング**: `templatefile()`で実際にレンダリングし、意図したCIDRスコープの`ufw allow from`行が生成されることを確認。`shellcheck`で新規警告0件。
- **`.gitignore`修正**: `git check-ignore`で、`terraform.tfvars`が新たに無視されること、`terraform.tfvars.example`は無視されないことを実際に確認。
- **`actionlint`**: `.github/workflows/ci.yml`の全変更後、警告・エラー0件。

## 9. 残っている課題

- セッション管理(30日固定・スライディング延長なし)の見直しは本フェーズでは行っていない(1節)。
- レート制限がプロセスローカルであること(共有ストアなし)は既知のトレードオフとして受容した(1節)。production環境の2台構成での実効的な挙動は、フェーズ17の負荷試験で実際に観測することが望ましい。
- CSPの実ブラウザでのコンソールエラー確認は、Chromeの自己署名証明書インタースティシャルの自動操作制限により行えなかった(3節)。実際のドメイン・実証明書が用意できるフェーズ(本番/staging環境構築後)で改めて確認するのが望ましい。
- Grafana Alloyの`docker run`イメージバージョン固定(`v1.19.2`、フェーズ14)・GHCRパッケージの保持ポリシー(フェーズ13から継続)・`matsu0122.com`ゾーンの管理場所確認(フェーズ11から継続)・Terraform stateのリモートbackend移行(フェーズ7から継続)・アプリサーバ内部NICの静的IP割り当て(フェーズ8から継続)は、いずれも本フェーズのスコープ外として引き続き未解消。
- `sakuracloud_database_read_replica`がフェイルオーバー用途の冗長構成に使えるかの調査(フェーズ15から継続)。
- 依存パッケージ・コンテナイメージのCIスキャン(本フェーズで追加)は、実際にGitHub Actions上で走ってグリーンになることはまだ確認できていない(ワークフローファイル自体を未コミット・未push のため、フェーズ12からの既知の制約と同じ)。
