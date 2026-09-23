locals {
  # Only the load balancer's real IPs may reach app servers on 80/443 —
  # everyone else must go through it, so its health-check-driven routing
  # (and TLS termination, which happens at each app server's own Nginx —
  # see frontend/nginx.conf) can't be bypassed by hitting a server
  # directly. This is deliberately a two-step bootstrap, not an automatic
  # module.load_balancer reference: the load balancer's real IPs are
  # assigned by Sakura Cloud (via sakuracloud_internet) only once it's
  # created, and it needs app_server's output, which needs network's — a
  # network -> load_balancer reference here would be a dependency cycle.
  # First apply with lb_known_ip_addresses left at its default (open to
  # the internet on 80/443, safe enough to bootstrap since nothing
  # sensitive is exposed by the health-check paths alone), then read
  # module.load_balancer.assigned_ip_addresses and re-apply with it set to
  # lock this down for real.
  #
  # Computed once here (not inline on each module block) and reused for
  # both the packet filter (module.network) and the host ufw rules
  # (module.app_server, phase 16) so the two layers can't drift apart —
  # see docs/security-review.md.
  web_allowed_source_cidrs = length(var.lb_known_ip_addresses) > 0 ? [for ip in var.lb_known_ip_addresses : "${ip}/32"] : ["0.0.0.0/0"]
}

module "network" {
  source                   = "../../modules/network"
  name_prefix              = var.name_prefix
  description              = "Levelog production"
  admin_ssh_cidrs          = var.admin_ssh_cidrs
  web_allowed_source_cidrs = local.web_allowed_source_cidrs
}

module "app_server" {
  source = "../../modules/app_server"

  name_prefix              = var.name_prefix
  server_count             = 2
  core                     = 2
  memory_gb                = 4
  internal_switch_id       = module.network.switch_id
  public_packet_filter_id  = module.network.app_public_packet_filter_id
  ssh_public_key           = var.ssh_public_key
  tags                     = ["levelog", "production", "app"]
  environment_name         = "production"
  admin_ssh_cidrs          = var.admin_ssh_cidrs
  web_allowed_source_cidrs = local.web_allowed_source_cidrs
}

module "database" {
  source = "../../modules/database"

  name_prefix      = var.name_prefix
  switch_id        = module.network.switch_id
  internal_gateway = module.network.internal_gateway
  internal_netmask = module.network.internal_netmask
  internal_cidr    = module.network.internal_cidr
  ip_address       = cidrhost(module.network.internal_cidr, 11)
  plan             = "30g"
  admin_username   = var.db_admin_username
  admin_password   = var.db_admin_password
  app_username     = var.db_app_username
  app_password     = var.db_app_password
  tags             = ["levelog", "production", "db"]

  # PITR (continuous_backup): the NFS target this needs is now
  # self-provisioned by the module (phase 15, see
  # docs/backup-restore-design.md) rather than an external prerequisite, so
  # this can default to on for production. database_version must be a
  # concrete value for the provider to accept a continuous_backup block —
  # verify "16" is still the version the appliance would actually be
  # created with (sakuracloud control panel / API) before the first real
  # apply; if it's changed there since this was written, changing it here
  # recreates the appliance (Changing this forces a new resource).
  enable_continuous_backup         = true
  database_version                 = "16"
  continuous_backup_nfs_ip_address = cidrhost(module.network.internal_cidr, 12)

  # Redundancy (replica_user/replica_password) is deliberately left off by
  # default — see docs/database-design.md section 3 for why the actual
  # standby pairing needs a manual step regardless.
}

module "load_balancer" {
  source = "../../modules/load_balancer"

  name_prefix          = var.name_prefix
  vrid                 = var.lb_vrid
  backend_ip_addresses = module.app_server.public_ip_addresses
  # vip_address and vip_ports left at their defaults ([80, 443] for the
  # latter) — see modules/load_balancer.
}

module "monitoring" {
  source = "../../modules/monitoring"

  name_prefix          = var.name_prefix
  target               = var.monitor_target
  notify_slack_webhook = var.monitor_slack_webhook
}
