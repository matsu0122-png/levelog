# Self-provisioned NFS target for PITR-style continuous backup. Phase 9
# left this as an external prerequisite ("provision an NFS server, then
# supply its address"); phase 14/15 confirmed sakuracloud_nfs is a real,
# declaratively-managed resource, so this module can provision its own
# instead of requiring a pre-existing one — see docs/backup-restore-design.md
# for the decision. Only created when continuous backup is on and no
# external NFS address was explicitly supplied (var.continuous_backup_nfs_connect
# lets a caller opt out of self-provisioning and point at a shared/existing
# NFS appliance instead). Same network posture as the database itself:
# internal switch only, no public NIC.
resource "sakuracloud_nfs" "pitr" {
  count = var.enable_continuous_backup && var.continuous_backup_nfs_connect == null ? 1 : 0

  name = "${var.name_prefix}-db-pitr-nfs"
  plan = var.continuous_backup_nfs_plan
  size = var.continuous_backup_nfs_size_gb
  tags = var.tags

  network_interface {
    switch_id  = var.switch_id
    ip_address = var.continuous_backup_nfs_ip_address
    netmask    = var.internal_netmask
    gateway    = var.internal_gateway
  }
}

locals {
  # Prefer an explicitly supplied address (shared/existing NFS appliance);
  # otherwise use the one self-provisioned above. null when continuous
  # backup is disabled, which the dynamic "continuous_backup" block below
  # never evaluates in that case anyway.
  continuous_backup_connect = var.continuous_backup_nfs_connect != null ? var.continuous_backup_nfs_connect : (
    var.enable_continuous_backup ? "nfs://${sakuracloud_nfs.pitr[0].network_interface[0].ip_address}/export" : null
  )
}

# Attaches only to the internal switch (no "shared"/public NIC at all), and
# additionally restricts inbound to the internal CIDR via source_ranges —
# two independent layers enforcing "DBは内部ネットワークからのみ接続".
resource "sakuracloud_database" "primary" {
  name             = "${var.name_prefix}-db"
  database_type    = var.database_type
  database_version = var.database_version
  plan             = var.plan
  username         = var.admin_username
  password         = var.admin_password
  replica_user     = var.replica_user
  replica_password = var.replica_password
  tags             = var.tags

  network_interface {
    switch_id     = var.switch_id
    ip_address    = var.ip_address
    netmask       = var.internal_netmask
    gateway       = var.internal_gateway
    source_ranges = [var.internal_cidr]
  }

  backup {
    time     = var.backup_time
    weekdays = var.backup_weekdays
  }

  dynamic "continuous_backup" {
    for_each = var.enable_continuous_backup ? [1] : []
    content {
      connect      = local.continuous_backup_connect
      time         = var.continuous_backup_time
      days_of_week = var.continuous_backup_weekdays
    }
  }

  monitoring_suite {
    enabled = var.monitoring_suite_enabled
  }
}

# Connects to the appliance as admin_username to provision the restricted
# application role — see variables.tf's comment on admin_username/
# app_username, and docs/database-design.md for why this split exists and
# the network-reachability implication of running this at apply time.
provider "postgresql" {
  host      = var.ip_address
  port      = 5432
  database  = var.database_name
  username  = var.admin_username
  password  = var.admin_password
  sslmode   = var.postgresql_sslmode
  superuser = false
}

resource "postgresql_role" "app" {
  name     = var.app_username
  login    = true
  password = var.app_password
}

# DML only — no CREATE/ALTER/DROP. If the application is ever compromised
# (e.g. a SQL-injection bug), the blast radius stops at row data; it cannot
# alter schema or drop tables.
resource "postgresql_grant" "app_tables" {
  database    = var.database_name
  role        = postgresql_role.app.name
  schema      = "public"
  object_type = "table"
  privileges  = ["SELECT", "INSERT", "UPDATE", "DELETE"]
}

resource "postgresql_grant" "app_sequences" {
  database    = var.database_name
  role        = postgresql_role.app.name
  schema      = "public"
  object_type = "sequence"
  privileges  = ["SELECT", "UPDATE", "USAGE"]
}

# Migrations run as admin_username and create new tables/sequences over
# time; without this, app_username would have no access to anything a
# migration adds after this Terraform config was first applied.
resource "postgresql_default_privileges" "app_future_tables" {
  database    = var.database_name
  role        = postgresql_role.app.name
  owner       = var.admin_username
  schema      = "public"
  object_type = "table"
  privileges  = ["SELECT", "INSERT", "UPDATE", "DELETE"]
}

resource "postgresql_default_privileges" "app_future_sequences" {
  database    = var.database_name
  role        = postgresql_role.app.name
  owner       = var.admin_username
  schema      = "public"
  object_type = "sequence"
  privileges  = ["SELECT", "UPDATE", "USAGE"]
}
