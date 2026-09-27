module "network" {
  source          = "../../modules/network"
  name_prefix     = var.name_prefix
  description     = "Levelog staging"
  admin_ssh_cidrs = var.admin_ssh_cidrs
  # web_allowed_source_cidrs left at its default (0.0.0.0/0): staging has
  # no load balancer, so the app server's own public NIC is the front door.
}

# No load balancer in staging — DNS points directly at the single app
# server (see docs/staging-environment.md section 6/7 for why).
module "app_server" {
  source = "../../modules/app_server"

  name_prefix             = var.name_prefix
  server_count            = 1
  core                    = 1
  memory_gb               = 2
  internal_switch_id      = module.network.switch_id
  public_packet_filter_id = module.network.app_public_packet_filter_id
  ssh_public_key          = var.ssh_public_key
  tags                    = ["levelog", "staging", "app"]
  environment_name        = "staging"
  admin_ssh_cidrs         = var.admin_ssh_cidrs
  # web_allowed_source_cidrs left at its default (0.0.0.0/0), same as the
  # network module call above and for the same reason.
}

module "database" {
  source = "../../modules/database"

  name_prefix      = var.name_prefix
  switch_id        = module.network.switch_id
  internal_gateway = module.network.internal_gateway
  internal_netmask = module.network.internal_netmask
  internal_cidr    = module.network.internal_cidr
  ip_address       = cidrhost(module.network.internal_cidr, 11)
  plan             = "10g"
  admin_username   = var.db_admin_username
  admin_password   = var.db_admin_password
  app_username     = var.db_app_username
  app_password     = var.db_app_password
  tags             = ["levelog", "staging", "db"]

  # Staging is a single unreplicated node and doesn't need continuous
  # backup / redundancy — daily backup (module default) is enough.
}

module "monitoring" {
  source = "../../modules/monitoring"

  name_prefix          = var.name_prefix
  target               = var.monitor_target
  notify_slack_webhook = var.monitor_slack_webhook
}

# staging.levelog.matsu0122.com -> the single app server (no load balancer
# in staging). The zone is created by terraform/environments/dns (apply
# that first), delegated from Vercel.
module "dns" {
  source = "../../modules/dns_record"

  zone         = var.dns_zone
  name         = "staging"
  ipv4_address = module.app_server.public_ip_addresses[0]
}
