variable "sakura_zone" {
  description = "Sakura Cloud zone (e.g. \"tk1a\", \"tk1b\", \"is1a\", \"is1b\")."
  type        = string
  default     = "tk1a"
}

variable "name_prefix" {
  description = "Prefix applied to every resource name in this environment."
  type        = string
  default     = "levelog-production"
}

variable "ssh_public_key" {
  description = "SSH public key installed on app servers for the deploy user."
  type        = string
}

variable "admin_ssh_cidrs" {
  description = "Source CIDRs allowed to SSH into app servers (e.g. [\"203.0.113.4/32\"]). No default — see modules/network."
  type        = list(string)
}

variable "db_admin_username" {
  description = "Database appliance admin/migration user name (DDL rights; not used by the running app — see modules/database)."
  type        = string
  default     = "levelog_migrate"
}

variable "db_admin_password" {
  description = "Password for db_admin_username. No default — supply via TF_VAR_db_admin_password from a secret store, never in terraform.tfvars."
  type        = string
  sensitive   = true
}

variable "db_app_username" {
  description = "Restricted, DML-only database user name the running application connects as."
  type        = string
  default     = "levelog_app"
}

variable "db_app_password" {
  description = "Password for db_app_username. No default — supply via TF_VAR_db_app_password from a secret store, never in terraform.tfvars."
  type        = string
  sensitive   = true
}

# --- Load balancer: modules/load_balancer now provisions its own public
# IP block (sakuracloud_internet) and derives the active/standby pair and
# VIP from it directly, so no external switch/IP/netmask/gateway/VIP
# variables are needed here anymore (see docs/network-design.md section 9
# / terraform/README.md for what changed and why).
variable "lb_vrid" {
  description = "VRRP router ID for the load balancer pair — any value unique within the routed switch's broadcast domain (e.g. 1)."
  type        = number
}

# Two-step bootstrap value — see the comment on the network module call in
# main.tf. Left empty (open to the internet on 80/443) until the load
# balancer's real IPs are known from a first apply, then set to
# module.load_balancer.active_standby_ip_addresses and re-applied.
variable "lb_known_ip_addresses" {
  description = "The load balancer's real active/standby IPs, once known (see main.tf's network module call). Empty by default."
  type        = list(string)
  default     = []
}

variable "monitor_target" {
  description = "FQDN checked by the external uptime monitor."
  type        = string
  default     = "levelog.matsu0122.com"
}

variable "monitor_slack_webhook" {
  description = "Slack incoming webhook URL for alert notifications. No default — supply via a secret-injection mechanism, never in terraform.tfvars."
  type        = string
  default     = ""
  sensitive   = true
}
