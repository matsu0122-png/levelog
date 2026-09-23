# バックアップと復元 設計

最終更新: 2026-09-23(フェーズ15)

このドキュメントは、Levelogの本番/staging環境における「PostgreSQLのバックアップ方式の実運用確認」「バックアップからの復元手順の検証」「アプリサーバ・設定ファイル(Terraform管理外のものを含む)のバックアップ方針」を扱う。PostgreSQL自体の詳細(ユーザー分離・TLS等)は`docs/database-design.md`を参照し、本ドキュメントはバックアップ・復元に関わる部分(本フェーズでの変更・新規決定)に絞る。

**実際のさくらのクラウードの有料リソース作成・`terraform apply`は本フェーズでも一切行っていない。** 一方、PostgreSQLの「バックアップ取得→データ消失→復元」という一連の流れ自体は、ローカルの開発用Postgres(`docker-compose.yml`の`db`、Levelogと同じマイグレーション・スキーマ)に対して`pg_dump`/`pg_restore`で実際にリハーサルした(3節)。さくらのクラウードのアプライアンス固有の操作(バックアップ世代の選択、新規アプライアンスとしての復元)自体はフェーズ17(障害試験)で実機リハーサルする。

## 1. バックアップ対象の棚卸し

| 対象 | 現状 | 復元不能になると何を失うか | 本フェーズでの扱い |
| --- | --- | --- | --- |
| PostgreSQLデータ(users/missions/xp等) | 日次バックアップ(フェーズ7〜9)+ production環境はPITR(本フェーズで有効化、2節) | ユーザーデータ全体 | 実装・リハーサル(2・3節) |
| アプリサーバのboot disk上の、Terraform管理外のファイル(`.env`・`monitoring.env`・TLS秘密鍵) | 手動作成のみ、他に一切コピーが存在しない | 各サーバーを1台ずつ作り直す手間(DBパスワード等の再入力、TLS証明書の再発行) | 週次ディスクスナップショットで保護(4節、新規) |
| アプリケーションコード・Terraformコード・Dockerイメージ定義 | Git(GitHub)+ GHCR(ビルド済みイメージ) | なし(すべてバージョン管理下) | 対象外(既にバックアップ済みと言える) |
| Terraform state | ローカルの`terraform.tfstate`(リモートbackend未移行、フェーズ7から継続の残課題) | 「何がどう作成されているか」の記録。実リソースは残るが、Terraformでの追跡・以後の`apply`が困難になる | **本フェーズのスコープ外**(6節に残課題として記載。バックアップ云々より前に、まずリモートbackendへの移行自体が必要な問題のため) |
| Grafana Cloud上のメトリクス・ログ | Grafana Cloud側の保持ポリシーに依存(無料枠の保持期間) | 過去の監視データ(復旧には影響しない、事後分析用) | 対象外(Grafana Cloud側の責務、`docs/monitoring-design.md`) |

## 2. PostgreSQL PITR用NFSの自己プロビジョニング(フェーズ9からの継続課題の解消)

フェーズ9では、PITR(`continuous_backup`ブロック)が「NFSサーバーのアドレスを事前に用意し、変数へ手動設定する」という外部前提条件を必要としており、そのNFSサーバー自体をどう用意するかが未確定のまま残っていた(`docs/database-design.md`10節)。

本フェーズで、さくらのクラウードのTerraformプロバイダ(`sacloud/sakuracloud`)が`sakuracloud_nfs`リソース(NFSアプライアンス自体を宣言的に作成できる)を実際に公開していることを、`terraform providers schema`とプロバイダの公式ドキュメント(`https://github.com/sacloud/terraform-provider-sakuracloud`)の両方で確認した。これにより、「事前にNFSサーバーを用意する」という手動の前提条件そのものを取り除けることが分かった。

- `terraform/modules/database`に`sakuracloud_nfs.pitr`(内部スイッチのみ接続、DBアプライアンスと同じネットワーク姿勢)を追加。`enable_continuous_backup = true`かつ`continuous_backup_nfs_connect`が未指定の場合にのみ作成される(`count`)。
- `connect`文字列(`nfs://<IP>/export`)は、自己プロビジョニングしたNFSのIPアドレスから`local.continuous_backup_connect`で自動的に導出する。外部の既存NFSを使いたい場合は`continuous_backup_nfs_connect`を明示的に指定すれば、自己プロビジョニングはスキップされる(どちらの運用も選べる設計)。
- `continuous_backup`には`database_version`の明示指定が必須(プロバイダのスキーマ制約)。production環境で`database_version = "16"`を設定した。**この値は実際のさくらのクラウードのカタログと突き合わせて確認したものではなく、一般的に流通しているPostgreSQLのメジャーバージョンからの仮置きである。初回の実`apply`前に、さくらのクラウードのコントロールパネル/APIで実際に選択可能な値を確認すること**(`os_type`変数に既にある同種の注記と同じ性質の暫定値)。
- production環境で`enable_continuous_backup = true`を既定にした(`terraform/environments/production/main.tf`)。staging環境は単一・非冗長ノードであり、日次バックアップで十分と判断し変更していない(`docs/database-design.md`5節の既存判断を踏襲)。

## 3. バックアップ・復元のリハーサル(ローカル、実施済み)

さくらのクラウードの実アプライアンスがまだ存在しない(`terraform apply`未実施)ため、アプライアンス自体の復元操作(バックアップ世代の選択、新規アプライアンスとしての起こし)は実機で検証できない。しかし、「バックアップを取得し、データを完全に失い、復元し、アプリケーションが正しく動くことを確認する」という一連の流れ自体は、同じPostgreSQLエンジン・同じマイグレーション・同じLevelogのバックエンドコードに対して、ローカルの開発用Postgres(`docker-compose.yml`の`db`)で実際に検証できる。本フェーズで以下の手順を実施した。

1. 開発用Postgresを起動し、Levelogのバックエンド(`go run ./cmd/api`)を実際に起動してマイグレーションを適用。
2. **実際のAPI経由で**(直接SQLではなく)テストユーザーを登録し、ミッションテンプレートを1件作成(`POST /api/auth/register`・`POST /api/missions`)。
3. `pg_dump -Fc`でカスタム形式のバックアップを取得。
4. `pg_terminate_backend` → `DROP DATABASE` → `CREATE DATABASE`で、データベースを完全に空の状態に戻す(意図的な「災害」の再現)。`\dt`で全テーブルが消えていることを確認。
5. `pg_restore --no-owner`でバックアップから復元。
6. 復元後のテーブルの行数(`users`・`mission_templates`)がバックアップ取得前と完全に一致することを確認。
7. 手順2で登録した特定のユーザー・ミッションが実際に復元されていることを確認(`SELECT ... WHERE email = 'backup-drill@example.com'`等)。
8. `schema_migrations`テーブルの行数を確認(`docs/database-design.md`8.1手順4に対応)。
9. **復元後のDBに対して実際にLevelogのバックエンドを起動**し、`/health/ready`が`200`を返すこと、手順2で登録したユーザーで実際に`POST /api/auth/login`できること、`GET /api/me`が正しいメールアドレスを返すことを確認(`docs/database-design.md`8.1手順5に対応)。

すべて成功した。検証に使ったバックアップファイル・データはリハーサル後に削除済みで、開発環境に痕跡は残していない。

この結果は、`docs/database-design.md`8.1節の復元runbookの手順3・4・5(アプリケーション側から見た確認手順)が実際に機能することの裏付けとなる。8.2節(PITR復元)については、アプライアンス自体の復元操作の部分はさくらのクラウード固有でありフェーズ17に持ち越すが、「復元後にアプリケーションから見て正しく動くこと」を確認する部分は本節と同じ考え方が適用できる。

## 4. アプリサーバのディスクバックアップ(新規)

`docs/app-server-design.md`(フェーズ10)は「ログを溢れさせない」「メトリクスを取得できる状態にする」までを扱い、ディスク自体のバックアップは対象外としていた。本フェーズで、以下の理由から`sakuracloud_auto_backup`(ディスク単位の週次スナップショット)を各アプリサーバのboot diskに追加した。

- アプリサーバのboot disk上には、Git・Terraformいずれの管理下にもない、他にコピーが存在しないファイルが存在する: `/opt/levelog/.env`(`docs/deployment-design.md`5節)、`/etc/levelog/monitoring.env`(`docs/monitoring-design.md`4節)、TLS秘密鍵(`docs/tls-design.md`)。これらはすべて「CI/Terraformの実行権限が本番の秘密情報に直結しないように」という意図的な設計判断で、手動投入のみとした結果として生まれた対象である。サーバーを失うと、単にサーバーを1台作り直すだけでなく、これらすべてを手動で再投入し直す作業が発生する。
- `terraform/modules/app_server`に`sakuracloud_auto_backup.boot`(各サーバーのboot diskに対応、`weekdays`・`max_backup_num`を変数化、既定は毎週日曜・2世代保持)を追加した。
- **ディスク全体のスナップショットであり、対象を上記の数ファイルだけに絞った設計ではない。** より的を絞ったバックアップ(例: その数ファイルだけをさくらのクラウードのオブジェクトストレージ等へ定期的にコピーする専用の仕組み)も検討したが、`sakuracloud_auto_backup`は既に存在する宣言的リソースをそのまま使うだけで済み、新たな仕組み(暗号化・転送先・スケジューラ)を設計・運用する必要がない。このプロジェクトで既に繰り返し採用してきた判断(Grafana Cloud、GHCR等、自前で運用対象を増やさない)と一貫させた。
- 復元は新しいディスクをスナップショット世代から作成する形になり(既存ディスクの上書きではない)、それをサーバーに差し替えるか、新規サーバーとして起こす運用になる想定。実際の復元操作はフェーズ17で実機リハーサルする。

## 5. 必要な変数まとめ(Terraform)

| 変数 | モジュール | 既定値 | 備考 |
| --- | --- | --- | --- |
| `enable_continuous_backup` | `modules/database` | `false`(productionでは`true`に上書き) | |
| `database_version` | `modules/database` | `null` | production環境で`"16"`を明示(2節の注記参照) |
| `continuous_backup_nfs_ip_address` | `modules/database` | `null` | production環境で`cidrhost(module.network.internal_cidr, 12)`を設定 |
| `continuous_backup_nfs_connect` | `modules/database` | `null` | 明示指定時は自己プロビジョニングをスキップ |
| `continuous_backup_nfs_plan` / `continuous_backup_nfs_size_gb` | `modules/database` | `"hdd"` / `100` | |
| `auto_backup_weekdays` | `modules/app_server` | `["sun"]` | |
| `auto_backup_max_generations` | `modules/app_server` | `2` | プロバイダ側の許容範囲は1〜10 |

秘密情報(DBパスワード等)は本フェーズで新たに追加していない。

## 6. 残っている課題

- **アプライアンス自体の復元操作の実機リハーサル**(さくらのクラウードのコントロールパネル/API、フェーズ17)。本フェーズで検証できたのはSQLレベルの手順のみ(3節)。
- **Terraform stateのリモートbackend移行**(フェーズ7から継続)。ローカル`terraform.tfstate`自体のバックアップ・保護は、まずリモートbackendへ移行してから設計するのが筋が良いと判断し、本フェーズでは着手していない。
- **`database_version = "16"`の実際のカタログ確認**(2節)。
- **PITR復元時、DBアプライアンスが新規アプライアンスとして起こされた場合の`continuous_backup`再設定の実際の手順**(`docs/database-design.md`8.2節3項)。
- **レプリカアプライアンスの実際の作成・紐付け手順**(`docs/database-design.md`から継続。`sakuracloud_database_read_replica`という読み取り専用レプリカ用のリソースの存在を本フェーズの調査で確認したが、フェイルオーバー可能な冗長構成の課題を直接解決するものではなさそうで、詳細は未調査)。
- **アプリサーバのディスクスナップショットからの実際の復元手順**(新規ディスク作成→サーバーへの差し替え、またはサーバー新規作成のどちらを標準手順とするか)は、フェーズ17の実機リハーサルで確定する。
- `matsu0122.com`ゾーンの管理場所確認、DNS-01チャレンジ用certbotプラグイン選定、証明書更新自動化(フェーズ11から継続)
- アプリサーバ内部NICの静的IP割り当て(フェーズ8から継続)
