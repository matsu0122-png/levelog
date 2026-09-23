# Terraform

Levelogのさくらのクラウードインフラを、staging/production完全分離のTerraformで管理する。設計の背景・分離方針は [`../docs/staging-environment.md`](../docs/staging-environment.md) を参照。

## ディレクトリ構成

```text
terraform/
├── modules/            # 環境非依存の再利用可能なモジュール
│   ├── network/          # 内部スイッチ・パケットフィルタ
│   ├── app_server/        # アプリサーバ(Nginx+React+Go実行)
│   ├── database/          # PostgreSQLアプライアンス
│   ├── load_balancer/     # ロードバランサ(production用)
│   └── monitoring/        # 外形監視
└── environments/
    ├── staging/          # 独立したstate。LBなし、サーバ1台
    └── production/       # 独立したstate。LB+サーバ2台
```

`environments/{staging,production}`はそれぞれ別のTerraform rootモジュールであり、別々のstateを持つ。一方に対する`plan`/`apply`が他方に影響することは構造上あり得ない。

## 認証情報

さくらのクラウードの認証情報は環境変数から読み込む。**`.tfvars`やコード内には一切書かない。**

```bash
export SAKURACLOUD_ACCESS_TOKEN="..."
export SAKURACLOUD_ACCESS_TOKEN_SECRET="..."
```

DBパスワード(`db_admin_password` / `db_app_password`)やSlack Webhook URL等の秘密情報も同様に、`TF_VAR_*`のような環境変数、またはCI/CDのSecrets機能経由で注入する。各`environments/*/terraform.tfvars.example`は非秘密情報のみを記載したテンプレートで、これをコピーした`terraform.tfvars`(`.gitignore`済み、フェーズ16で追加)を使うか、同じ値を`-var`/`TF_VAR_*`で直接渡す。

`modules/database`は`postgresql`プロバイダ(`cyrilgdn/postgresql`)でDBロールを直接管理しており、**この部分の`apply`はDBアプライアンスの内部ネットワークへ到達できる場所から実行する必要がある**(DBは内部ネットワークからしか到達できない設計のため)。詳細は[`../docs/database-design.md`](../docs/database-design.md)を参照。

## 実行方法(このフェーズで許可されている範囲)

```bash
cd environments/staging   # または environments/production
terraform init
terraform fmt -check -recursive   # ../ から実行する場合は -recursive
terraform validate
terraform plan
```

`terraform apply` / `terraform destroy`は、明示的な許可を得るまで実行しない。

## State管理方針

現時点では各環境ともローカルbackend(`terraform.tfstate`が作業ディレクトリ直下に作られる、デフォルトの挙動)を使用している。これは「リモートbackend用のオブジェクトストレージバケット自体をどう作るか」という鶏と卵の問題があるため、意図的な暫定措置である。

将来的には、さくらのクラウードのオブジェクトストレージ(S3互換)をバックエンドとして使う想定で、`environments/{staging,production}/versions.tf`内に`backend "s3" { ... }`の設定例をコメントアウトで残してある。移行手順(概要):

1. バケットをTerraform管理外で一度だけ手動作成する(bootstrap)。
2. `versions.tf`のコメントアウトを解除し、`terraform init -migrate-state`でローカルstateをリモートへ移行する。
3. 移行後は各開発者・CIが同じstateを参照できることを確認する。

state(バックエンド)にはリソースの属性値がしばしば平文で含まれるため、**ローカルbackend使用中の`.tfstate`はGit管理対象外**(`.gitignore`参照)であり、共同作業が必要になった時点でリモートbackendへの移行を最優先で行う。

## 注意事項

- `modules/app_server`の`os_type`、`modules/database`の`plan`、`modules/load_balancer`の`plan`等、さくらのクラウードAPI側のプラン名・イメージ名に依存する値は暫定値である。実際の`apply`前に、さくらのクラウードコントロールパネル/APIで最新の値を確認すること。
- `modules/load_balancer`は、公開用(ルーティング済み)スイッチ(`sakuracloud_internet`)を自身で作成し、公開IP・VIPを自動的に払い出された値から導出する。`environments/production`のネットワークパケットフィルタ側は、ロードバランサの実IPをこの払い出し後にしか知り得ないため、意図的な2段階ブートストラップ(`lb_known_ip_addresses`変数)になっている。詳細は[`../docs/tls-design.md`](../docs/tls-design.md)を参照。
- `modules/network`のパケットフィルタは、SSH(`admin_ssh_cidrs`)・HTTP/HTTPS(`web_allowed_source_cidrs`)・ICMPの許可ルールを実装済み(詳細は[`../docs/network-design.md`](../docs/network-design.md))。
- `modules/database`のアプリ用ユーザー分離・冗長構成・PITR・TLS・復元手順は[`../docs/database-design.md`](../docs/database-design.md)を参照。DBアプライアンスのレプリカ(冗長構成)の実際の紐付けは、プロバイダのスキーマ上Terraformだけでは完結せず手動操作が必要な点に注意。
- TLS終端はロードバランサではなく各アプリサーバのNginxで行う設計。証明書取得・DNS設計・片系停止試験の計画は[`../docs/tls-design.md`](../docs/tls-design.md)を参照。
