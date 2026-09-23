# アプリサーバ構築

最終更新: 2026-09-23(フェーズ10)

このドキュメントは、Levelogのさくらのクラウードアプリサーバ(App Server 1/2)のOS構築方針を定義する。実装は`terraform/modules/app_server`(フェーズ7〜9で作成済みの骨格に、本フェーズでSakura Cloudの起動スクリプト機構を追加)。クラウドリソースの作成・`terraform apply`は本フェーズでも一切行っていない。

## 1. 全体方針: このフェーズが用意するもの / 用意しないもの

| 用意するもの(本フェーズ) | 用意しないもの(他フェーズ) |
| --- | --- |
| OS・Docker・デプロイ用ユーザーのセットアップ(起動スクリプト) | アプリケーション自体のデプロイ(`docker-compose.prod.yml`の配置・起動)→ フェーズ13(CD) |
| 監視エージェント(node_exporter)のインストール | 実際のメトリクス収集先・アラート設計 → フェーズ14 |
| ログローテーション(Dockerのjson-fileログ) | 外部への集約ログ転送先の選定・設定 → フェーズ14 |
| ヘルスチェックの土台(後述) | ヘルスチェックしきい値のチューニング → フェーズ14・17 |

**「Nginx」について**: フェーズ4で、フロントエンドの本番用Dockerfileは既に`nginxinc/nginx-unprivileged`ベースのコンテナとして実装済みであり、`docker-compose.prod.yml`の`web`コンテナがNginx+Reactを、`api`コンテナがGoを実行する。本フェーズでは**ホストOS上に別途Nginxをインストールしない**(コンテナ内のNginxと二重になり、複雑さが増すだけで利点がないため)。「各アプリサーバでNginx、React、Goを実行」という最終目標は、このコンテナ構成で満たされる。

## 2. 起動スクリプト(Sakura Cloudのnote機構)

さくらのクラウードのサーバーリソース(`sakuracloud_server`)は、ディスクの`disk_edit_parameter.note_ids`を通じて、初回起動時に1回だけ実行される「スタートアップスクリプト」(`sakuracloud_note`、`class = "shell"`)を指定できる。本フェーズで`terraform/modules/app_server/templates/startup.sh.tftpl`を作成し、`templatefile()`関数でデプロイユーザー名・SSH公開鍵を埋め込んだ上で`sakuracloud_note`として登録し、全アプリサーバの`disk_edit_parameter`から参照するようにした。

スクリプトの内容(概要):

1. OSパッケージの更新、`unattended-upgrades`によるセキュリティアップデートの自動化
2. ホストファイアウォール(`ufw`)の設定(22/80/443のみ許可 — さくらのクラウードのパケットフィルタ(フェーズ8)と同じ許可リストを、OSレベルでも多重に強制する多層防御)
3. `fail2ban`の有効化(SSHブルートフォース対策)
4. SSHのroot直接ログインを完全に禁止(パスワード認証は`disk_edit_parameter.disable_pw_auth`で既に無効化済み。本スクリプトはさらに`PermitRootLogin no`を設定する)
5. デプロイ用ユーザー(既定`deploy`)の作成。SSH公開鍵を設定し、`docker`グループにのみ追加する(sudo権限は付与しない — 必要な操作はDockerコンテナの操作に限定されるため)
6. Docker Engine + Docker Composeプラグインのインストール、`docker.service`の有効化(再起動後も自動起動し、`docker-compose.prod.yml`の`restart: unless-stopped`と組み合わさって、サーバー再起動後もコンテナが自動復旧する)
7. Dockerのログドライバに`max-size: 10m` / `max-file: 5`を設定(暴走したコンテナのログでディスクが枯渇する事故を防ぐ。実際の集約ログ転送先の選定はフェーズ14)
8. `node_exporter`(Prometheus用のOSメトリクスエクスポータ)を`127.0.0.1:9100`のみでリッスンするsystemdサービスとしてインストール(外部からは到達不可。実際にどう収集するか(SSHトンネル経由か、別途収集エージェントを追加するか)はフェーズ14の監視スタック選定に合わせて決定する)

このテンプレートは、実際に`templatefile()`でレンダリングした上で`bash -n`による構文チェックを実施済み(本フェーズの検証結果を参照)。

## 3. デプロイ用ユーザーの権限設計

| 項目 | 方針 |
| --- | --- |
| ユーザー名 | `deploy`(変数`deploy_user`で変更可) |
| ログイン方式 | SSH鍵認証のみ(`terraform`の`ssh_public_key`変数の鍵を`authorized_keys`に設定) |
| 権限 | `docker`グループのみ。`sudo`は付与しない |
| 用途 | フェーズ13のCDパイプラインが、このユーザーでSSHし、`docker compose -f docker-compose.prod.yml pull && up -d`のようなコマンドを実行する想定 |

rootでの直接ログインは(パスワード認証無効化に加え)`PermitRootLogin no`で完全に禁止しているため、緊急時のroot操作が必要な場合はさくらのクラウードのコントロールパネルの「コンソール」機能(シリアルコンソール相当)を使う想定とする。

## 4. ヘルスチェックの構成(まとめ)

本フェーズ単体で新たに実装したものはないが、既存フェーズの実装がどう組み合わさって「ヘルスチェック」を構成するかを整理する。

| レイヤー | 仕組み | 実装フェーズ |
| --- | --- | --- |
| アプリケーション | `/health/live`(生存確認) / `/health/ready`(DB疎通込み) | フェーズ2 |
| コンテナ | 各Dockerfileの`HEALTHCHECK`命令 | フェーズ4 |
| コンテナオーケストレーション | `docker-compose.prod.yml`の`depends_on: condition: service_healthy`、`restart: unless-stopped` | フェーズ4 |
| OS | `docker.service`のsystemd自動起動(再起動後もコンテナ群が自動復旧) | 本フェーズ |
| ロードバランサ | `/health/ready`への定期アクセスでバックエンドの生死を判定 | フェーズ7(モジュール実装)、フェーズ11(実配線) |
| 外形監視 | `/health/live`への外部からの定期アクセス | フェーズ7(モジュール実装)、フェーズ14(実運用) |

## 5. ログ転送・監視エージェントの現状スコープ

- 本フェーズで用意したのは「ログを溢れさせない」(ローテーション)と「メトリクスを取得できる状態にする」(node_exporterの設置)までであり、**実際の転送先・収集先の選定と設定はフェーズ14**で行う。理由: 監視・ログ基盤(Prometheus/Grafana、外部SaaS、さくらのクラウードのログサービス等)の選定は、本フェーズの主題(サーバー自体の構築)とは独立した意思決定であり、先に決め打ちすると手戻りのリスクがあるため。

## 6. 本フェーズでの実装内容(Terraform)まとめ

- `terraform/modules/app_server/templates/startup.sh.tftpl`(新規): 起動スクリプト本体
- `terraform/modules/app_server/main.tf`: `sakuracloud_note`リソースを追加し、`templatefile()`でレンダリング。`disk_edit_parameter.note_ids`から参照するよう配線
- `terraform/modules/app_server/variables.tf`: `deploy_user`変数(既定`deploy`)を追加

## 7. 未確定・今後の判断が必要な事項

- ~~ログ集約先・監視スタックの選定~~ → フェーズ14で決定・実装(`docs/monitoring-design.md`)。Grafana Alloyを本フェーズの起動スクリプトに追記する形で追加している(`terraform/modules/app_server/templates/startup.sh.tftpl`のnode_exporterブロック直後)。
- ~~node_exporterの実際のスクレイプ方法~~ → フェーズ14で決定(同上、SSHトンネルではなく各サーバー上のAlloyがアウトバウンドでGrafana Cloudへpush)
- 緊急時のroot操作手順(コンソール機能の利用手順)を運用手順書(フェーズ18)に記載
- アプリケーション自体のデプロイ手順の詳細(フェーズ13)
