# PostgreSQL本番構成

最終更新: 2026-09-23(フェーズ9作成、フェーズ15でバックアップ関連節を更新)

このドキュメントは、Levelogの本番PostgreSQLの構成方針(冗長構成・ユーザー分離・TLS・バックアップ・PITR・監視・復元手順)を定義する。実装は`terraform/modules/database`(フェーズ7で作成、本フェーズで大幅拡張)。クラウドリソースの作成・`terraform apply`・実DBへの変更は本フェーズでも一切行っていない。

## 1. DBアプライアンス概要

- さくらのクラウードのデータベースアプライアンス(`sakuracloud_database`)を使用し、PostgreSQLをマネージドサービスとして運用する。
- 内部スイッチにのみ接続し(フェーズ7・8で実装済み)、`source_ranges`で内部CIDR以外からのアクセスを拒否する。この構成は本フェーズで変更していない。
- プラン(`10g`/`30g`/`90g`/`240g`/`500g`/`1t`)は、実際のプロバイダスキーマ(`terraform providers schema`)から確認した値であり、staging=`10g`、production=`30g`を既定値としている。

## 2. アプリ用ユーザー / マイグレーション用ユーザーの分離

さくらのクラウードのDBアプライアンス自体は、`sakuracloud_database`リソース1つにつき「デフォルトユーザー」を1つしか持てない(スキーマ上、複数ユーザーを直接作成する属性は存在しない)。そこで、以下の2段構成を採用する。

| ユーザー | 役割 | 権限 | 作成方法 |
| --- | --- | --- | --- |
| `admin_username`(既定`levelog_migrate`) | マイグレーション専用。アプリは直接使用しない | DBアプライアンスのデフォルトユーザーとしての全権限(DDL含む) | `sakuracloud_database`リソースの`username`/`password`属性(フェーズ7から) |
| `app_username`(既定`levelog_app`) | アプリケーションの実行時接続用 | `public`スキーマに対するDML権限のみ(`SELECT`/`INSERT`/`UPDATE`/`DELETE`。`CREATE`/`ALTER`/`DROP`は不可) | Terraformの`postgresql`プロバイダ(`cyrilgdn/postgresql`)で`postgresql_role` + `postgresql_grant` + `postgresql_default_privileges`を作成 |

`postgresql_default_privileges`により、`admin_username`(マイグレーション実行者)が将来作成するテーブル・シーケンスにも自動的に`app_username`のDML権限が付与されるようにしている(マイグレーション追加のたびに手動でGRANTし直す必要がない)。

**設計意図**: アプリケーションが万一SQLインジェクション等で侵害されても、`app_username`にはスキーマ変更・テーブル削除の権限がないため、被害をデータの読み書きに限定できる(最小権限の原則)。

### 運用上の重要な注意: `postgresql`プロバイダはDBへの到達性を必要とする

`postgresql_role`等はTerraformの`postgresql`プロバイダを通じて、DBアプライアンスの内部IPへ直接SQL接続して作成される。DBは内部ネットワークからしか到達できない設計(フェーズ7・8)のため、**`terraform apply`をこの部分について実行する端末/CIランナーは、DBの内部ネットワークに到達できる必要がある。** 開発者の自宅PCから直接`apply`することはできない。想定される対応:

- CD(フェーズ13)のデプロイパイプラインを、アプリサーバ上(内部ネットワークに到達可能)で実行する、または内部ネットワークへの一時的なアクセス経路(踏み台等)を用意する。
- 初回構築時のみ、運用者のIPを一時的に`source_ranges`へ追加して`apply`し、完了後に取り除く運用も選択肢としてありうる(ただし恒久的な穴を開けない運用ルールが必要)。

具体的な方式はフェーズ12・13(CI/CD)で確定する。

## 3. 冗長構成

`sakuracloud_database`リソースは`replica_user`/`replica_password`という属性を公開しており、これはプライマリ側が「レプリカからの接続を受け付けるための認証情報」を意味する。**しかし、プロバイダのスキーマには「このアプライアンスは別のアプライアンスのレプリカである」ということを宣言する属性が存在しない。** つまり、2つ目の`sakuracloud_database`リソースを作ってプライマリと紐付ける、という操作をTerraformだけで宣言的に完結させることはできない(本フェーズで実際にプロバイダのスキーマを取得して確認した事実であり、推測ではない)。

このため、本フェーズでは以下の設計とした。

- `terraform/modules/database`は、プライマリ側に`replica_user`/`replica_password`を設定できるようにした(変数`replica_user`/`replica_password`、既定値`null`＝未設定)。
- **レプリカ(スタンバイ)アプライアンス自体の作成と、プライマリへの紐付けは、さくらのクラウードのコントロールパネルまたはAPI(`usacloud`等)を用いた手動(またはTerraform外のスクリプトによる)操作が必要**であることを明記する。
- 将来的にTerraformプロバイダ側がレプリカのリンクを宣言的にサポートするようになった場合、またはPostgreSQLのストリーミングレプリケーションを自前で構築する方式(2つの独立したアプライアンスの上で`pg_basebackup`等を用いる)を採用する場合は、本ドキュメントと`terraform/modules/database`を更新する。

**現時点の結論**: 冗長構成の「プライマリ側の準備」(レプリケーションユーザーの払い出し)まではTerraformで宣言的に管理できるが、「実際に2台構成にして同期させる」部分は本フェーズのスコープ外とし、フェーズ17(障害試験)より前に、実環境で手順を確立してから実施する。

## 4. TLS

- `postgresql`プロバイダ(ロール管理用の接続)は`sslmode = "require"`を既定にした(暗号化はするが証明書検証はしない)。DBアプライアンスのCA証明書を確実に取得できることが確認できたら`verify-full`へ強化する。
- アプリケーション(Go backend)の`DATABASE_URL`も同様に、本番では`sslmode=require`以上を指定する必要がある。`.env.example`にその旨のコメントを追記した(開発環境の既定値`sslmode=disable`はローカルDocker Compose用のまま変更していない)。

## 5. バックアップ

- 日次バックアップ(`backup`ブロック、フェーズ7で実装済み)は変更なし。時刻・曜日を変数化済み。

## 6. PITR(継続バックアップ)

- `sakuracloud_database`は`continuous_backup`ブロック(NFSサーバーへの継続バックアップ)を公開しており、これが実質的なPITR機能にあたる。制約(プロバイダのスキーマ記載を確認済み): `database_version`を明示的に指定した場合のみ設定可能。NFSサーバーのアドレス(`connect`、例: `nfs://192.0.2.1/export`)が別途必要。
- 本フェーズ(9)時点では、このNFSサーバーの準備がTerraform外の前提条件だった(「NFS/バックアップ用ストレージの準備」を別途行い、そのアドレスを`continuous_backup_nfs_connect`変数に手動で設定する想定)。**フェーズ15で、さくらのクラウードプロバイダが`sakuracloud_nfs`リソース(NFSアプライアンス自体を宣言的に作成できる)を実際に公開していることを確認した**ため、この前提条件は解消した。`terraform/modules/database`が`continuous_backup_nfs_connect`未指定時に自動的に`sakuracloud_nfs`を自己プロビジョニングし、そのIPアドレスから`connect`文字列を導出するよう変更している(詳細は`docs/backup-restore-design.md`)。
- production環境は本フェーズからPITRを既定で有効化した(`enable_continuous_backup = true`、`database_version = "16"`、`terraform/environments/production/main.tf`)。staging環境は単一・非冗長ノードであり日次バックアップで十分と判断し、引き続き無効のままとした(本フェーズでの変更なし)。

## 7. 監視

- `sakuracloud_database`の`monitoring_suite`ブロック(さくらのクラウードのMonitoring Suiteへの信号送信)を有効化した(既定`true`)。外形監視(フェーズ7の`modules/monitoring`)とは別に、アプライアンス自体のメトリクス監視の入口として機能する想定。詳細なアラート設計(CPU/メモリ/DB容量/接続数のしきい値等)はフェーズ14(監視・ログ・通知)で行う。

## 8. 復元手順(runbook)

このアプライアンス自体(`sakuracloud_database`)の実際の復元操作(バックアップ世代からの新規アプライアンス起こし)は、さくらのクラウードのコントロールパネル/APIの操作であり、本プロジェクトではまだ実際のアプライアンスが存在しない(`terraform apply`未実施)ため実機での検証はできていない。一方、**SQLレベルでの手順(バックアップ取得→データ消失→復元→整合性確認という一連の流れがLevelogのスキーマ・マイグレーション・アプリケーションコードに対して正しく機能すること)は、フェーズ15でローカルの開発用Postgres(`docker-compose.yml`の`db`)に対して`pg_dump`/`pg_restore`で実際にリハーサル済み**(検証結果は`docs/backup-restore-design.md`3節)。さくらのクラウードのアプライアンス復元も「新しいPostgresインスタンスへ、取得済みのバックアップからデータを流し込む」という点では同じ形であるため、以下の手順のうち3〜5(アプリケーション側から見た復元後の確認手順)は今回のリハーサルで実際に踏んだ手順と一致する。1〜2(アプライアンス自体の復元操作)はさくらのクラウード固有の操作であり、実機でのリハーサルはフェーズ17(障害試験)で行う。

### 8.1 日次バックアップからの復元

1. さくらのクラウードのコントロールパネルまたはAPIで、復元対象のバックアップ世代を確認する。
2. 復元は新規アプライアンスとして作成される(既存アプライアンスを上書きしない)想定のため、復元後のアプライアンスの内部IPを確認し、必要に応じて`terraform.tfvars`または変数を更新する。
3. アプリケーション側の`DATABASE_URL`を復元後のアプライアンスへ向け直す(フェーズ13のデプロイ手順に従う)。
4. `schema_migrations`テーブルの内容を確認し、バックアップ取得時点と現在のマイグレーション状態に差分がないか確認する。差分がある場合は、不足しているマイグレーションを再適用する前にデータ整合性を確認する。**フェーズ15のリハーサルでこの手順を実際に実行し、`SELECT count(*) FROM schema_migrations;`が復元前後で一致することを確認した。**
5. アプリケーションを起動し、`/health/ready`が`200`を返すこと、主要な画面が正常に動作することを確認してから切り戻しを完了する。**フェーズ15のリハーサルでは、復元後のDBに対して実際にアプリケーションを起動し、`/health/ready`が`200`を返すこと、復元前に登録したユーザーで実際にログインでき`/api/me`が正しいデータを返すことまで確認した。**

### 8.2 PITR(継続バックアップ)からの復元

フェーズ15でPITR自体をproduction環境に有効化した(6節)。復元手順は日次バックアップからの復元(8.1)と基本的に同じだが、次の点が異なる。

1. さくらのクラウードのコントロールパネルまたはAPIで、復元したい**時刻**(バックアップ世代ではなく、任意のタイムスタンプ)を指定する。NFSに継続的に蓄積されたWAL相当のデータから、指定時刻の状態が復元される想定(さくらのクラウード側の実装詳細はドキュメント上明確でないため、実機検証時に確認する)。
2. 復元後の新規アプライアンスの内部IPを確認し、`DATABASE_URL`を向け直す(8.1の2〜5と同様)。
3. **PITR用NFSアプライアンス自体は`terraform/modules/database`の一部として管理されている**(`sakuracloud_nfs.pitr`)ため、DBアプライアンスを復元してもNFSアプライアンス自体は別リソースとして存続する。DB側を新規アプライアンスとして復元した場合、その新アプライアンスに対して改めて`continuous_backup`を設定し直す(同じNFSを指すよう`terraform apply`し直す)必要がある可能性がある。
4. 実際の復元操作(さくらのクラウードAPI/コントロールパネル)のリハーサルはフェーズ17で行う。

### 8.3 復元後の確認チェックリスト(共通)

- `SELECT count(*) FROM schema_migrations;`等でマイグレーション適用状況を確認する。
- アプリ用ユーザー(`app_username`)の権限が復元後のDBにも正しく存在するか確認する(復元方式によっては、権限設定(本フェーズで追加した`postgresql_role`等)を復元後に再適用(`terraform apply`)する必要がある可能性がある)。
- セッション(`sessions`テーブル)は復元時点のものに巻き戻るため、全ユーザーが再ログインを求められる可能性がある(既知の影響として運用手順書(フェーズ18)に記載する)。

## 9. 本フェーズ(9)での実装内容(Terraform)まとめ

- `terraform/modules/database`の変数を`username`/`password`から`admin_username`/`admin_password`(マイグレーション用)+`app_username`/`app_password`(アプリ用)へ分離。
- `postgresql`プロバイダ(`cyrilgdn/postgresql`)を追加し、`postgresql_role` / `postgresql_grant`(テーブル・シーケンス) / `postgresql_default_privileges`(将来のテーブル・シーケンス)でアプリ用ユーザーの権限を宣言的に管理。
- `replica_user` / `replica_password`(プライマリ側のレプリケーション受け入れ設定)、`continuous_backup`(PITR、NFS前提)、`monitoring_suite`を変数経由で設定可能にした(いずれも既定では無効/未設定)。
- `database_name`(既定`postgres`)、`postgresql_sslmode`(既定`require`)を追加。
- `terraform/environments/{staging,production}`の変数・tfvars・main.tfを上記に合わせて更新。

**フェーズ15での追加変更**は`docs/backup-restore-design.md`にまとめた(`sakuracloud_nfs`の自己プロビジョニング、production環境でのPITR有効化)。

## 10. 未確定・今後の判断が必要な事項

- レプリカアプライアンスの実際の作成・紐付け手順(手動操作、フェーズ17より前に実環境で確立)。**フェーズ15の調査で、`sakuracloud_database_read_replica`リソース(読み取り専用レプリカを宣言的に作成できる)がプロバイダに存在することを確認した。** ただしこれは読み取り専用レプリカであり、本節が指す「フェイルオーバー可能な冗長構成(プライマリ昇格)」とは異なる可能性が高く、そのままでは3節の課題を解決しない。実際にフェイルオーバー用途に使えるかは未調査(読み取りスケーリング目的なら有用)。
- ~~PITR用NFSストレージの準備・実際のアドレス確認~~ → フェーズ15で解消(`sakuracloud_nfs`を自己プロビジョニング、`docs/backup-restore-design.md`)
- `postgresql`プロバイダによる`apply`をどこから実行するか(フェーズ12・13のCI/CD設計と合わせて決定)
- DBアプライアンスの実際のデータベース名(`postgres`と仮定しているが、初回`apply`後に実際のアプライアンスで確認する必要がある)
- `verify-full`へのTLS強化(CA証明書の入手方法確認後)
