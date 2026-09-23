variable "name_prefix" {
  description = "Prefix applied to every resource name (e.g. \"levelog-staging\")."
  type        = string
}

variable "switch_id" {
  description = "ID of the internal switch (from the network module) this database attaches to. It has no other network path — this is what keeps it unreachable from the public internet."
  type        = string
}

variable "internal_gateway" {
  description = "Gateway address on the internal network (from the network module)."
  type        = string
}

variable "internal_netmask" {
  description = "Netmask (prefix length) of the internal network (from the network module)."
  type        = number
}

variable "internal_cidr" {
  description = "CIDR of the internal network; used as source_ranges so only app servers on this network can reach the database port."
  type        = string
}

variable "ip_address" {
  description = "Static internal IP address for the primary database appliance."
  type        = string
}

variable "plan" {
  description = "Database appliance plan. Must be one of 10g/30g/90g/240g/500g/1t (verified against the sakuracloud provider's documented values as of this writing — recheck before the first real apply in case the catalog changed)."
  type        = string
  default     = "10g"
}

variable "database_type" {
  description = "Database engine."
  type        = string
  default     = "postgres"
}

variable "database_version" {
  description = "Database engine version. Required (non-null) if enable_continuous_backup is true — the provider only accepts a continuous_backup block when this is set. Left unset otherwise so the appliance's own default version is used."
  type        = string
  default     = null
}

# --- Appliance-level default user: this is a single, broadly privileged
# account (the appliance schema exposes exactly one). We use it only for
# migrations (DDL) and for provisioning the restricted app role below —
# the running application never uses these credentials directly. See
# docs/database-design.md for the full rationale.
variable "admin_username" {
  description = "Name of the database appliance's single default user. Used for running migrations (DDL) and for provisioning app_username below — never given to the running application."
  type        = string
  default     = "levelog_migrate"
}

variable "admin_password" {
  description = "Password for admin_username. No default on purpose — supply via TF_VAR_admin_password (or an equivalent secret-injection mechanism), never in .tfvars."
  type        = string
  sensitive   = true
}

# --- Restricted application user (DML only: SELECT/INSERT/UPDATE/DELETE on
# the public schema, no DDL). Created via the postgresql provider, which
# connects directly to the appliance as admin_username — see
# docs/database-design.md for the operational implication (this requires
# network reachability to the appliance's internal IP at apply time).
variable "app_username" {
  description = "Name of the restricted, DML-only role the running application connects as."
  type        = string
  default     = "levelog_app"
}

variable "app_password" {
  description = "Password for app_username. No default on purpose — supply via TF_VAR_app_password (or an equivalent secret-injection mechanism), never in .tfvars."
  type        = string
  sensitive   = true
}

variable "database_name" {
  description = "Name of the Postgres database to connect to and to scope role grants to. The sakuracloud_database resource doesn't expose a way to name this, so it defaults to \"postgres\" (the appliance's and the postgresql provider's own default) — verify this against the actual provisioned appliance before relying on it."
  type        = string
  default     = "postgres"
}

variable "postgresql_sslmode" {
  description = "sslmode used by the postgresql provider (role/grant management) when connecting to the appliance. \"require\" encrypts the connection without verifying the server certificate; switch to \"verify-full\" once the appliance's CA certificate is confirmed obtainable."
  type        = string
  default     = "require"
}

variable "backup_time" {
  description = "Daily backup time (HH:mm, appliance-local)."
  type        = string
  default     = "03:00"
}

variable "backup_weekdays" {
  description = "Weekdays to run the daily backup on."
  type        = list(string)
  default     = ["mon", "tue", "wed", "thu", "fri", "sat", "sun"]
}

variable "enable_continuous_backup" {
  description = "Enable PITR-style continuous backup to an NFS target. Requires database_version to be set (the provider rejects a continuous_backup block otherwise). When true and continuous_backup_nfs_connect is left unset, this module provisions its own sakuracloud_nfs appliance (see continuous_backup_nfs_ip_address below) rather than requiring a pre-existing one — see docs/backup-restore-design.md for why."
  type        = bool
  default     = false
}

variable "continuous_backup_nfs_connect" {
  description = "NFS server address for continuous (PITR) backup storage, e.g. \"nfs://192.0.2.1/export\". No default. Leave unset to have this module provision and use its own sakuracloud_nfs appliance instead (the common case); set explicitly only to point at an already-existing/shared NFS appliance."
  type        = string
  default     = null
}

variable "continuous_backup_nfs_ip_address" {
  description = "Static internal IP address for the self-provisioned PITR NFS appliance (see continuous_backup_nfs_connect above). Required only when enable_continuous_backup is true and continuous_backup_nfs_connect is left unset; ignored otherwise."
  type        = string
  default     = null
}

variable "continuous_backup_nfs_plan" {
  description = "Plan for the self-provisioned PITR NFS appliance (\"hdd\" or \"ssd\"). Backup storage has no latency requirement, so the cheaper hdd plan is the default."
  type        = string
  default     = "hdd"
}

variable "continuous_backup_nfs_size_gb" {
  description = "Size (GiB) of the self-provisioned PITR NFS appliance."
  type        = number
  default     = 100
}

variable "continuous_backup_weekdays" {
  description = "Weekdays continuous backup runs on (only used when enable_continuous_backup is true)."
  type        = list(string)
  default     = ["mon", "tue", "wed", "thu", "fri", "sat", "sun"]
}

variable "continuous_backup_time" {
  description = "Daily continuous-backup start time (HH:mm, only used when enable_continuous_backup is true)."
  type        = string
  default     = "04:00"
}

variable "monitoring_suite_enabled" {
  description = "Enable Sakura Cloud's Monitoring Suite signal reporting for this appliance."
  type        = bool
  default     = true
}

# --- Redundancy: the sakuracloud provider's sakuracloud_database resource
# exposes replica_user/replica_password (credentials a standby connects to
# the primary with), but its schema has no attribute linking a *second*
# sakuracloud_database resource to this one as that standby — that pairing
# isn't something this provider models declaratively. This module can only
# configure the primary side; provisioning and pairing the standby appliance
# is a manual step (Sakura Cloud control panel / API) documented in
# docs/database-design.md. Left unset (null) by default, which omits
# replica_user/replica_password from the resource entirely (no replication
# user provisioned until deliberately configured).
variable "replica_user" {
  description = "Name of the replication user the primary appliance exposes for a standby to connect as. No default — see the module-level comment above before setting this."
  type        = string
  default     = null
}

variable "replica_password" {
  description = "Password for replica_user. No default on purpose — supply via TF_VAR_replica_password, never in .tfvars."
  type        = string
  default     = null
  sensitive   = true
}

variable "tags" {
  description = "Tags applied to the database appliance(s)."
  type        = list(string)
  default     = []
}
