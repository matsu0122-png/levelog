variable "sakura_zone" {
  description = "Sakura Cloud zone (e.g. \"tk1a\", \"tk1b\", \"is1a\", \"is1b\")."
  type        = string
  default     = "tk1a"
}

variable "name_prefix" {
  description = "Prefix applied to every resource name in this environment."
  type        = string
  default     = "levelog-staging"
}

variable "ssh_public_key" {
  description = "SSH public key installed on the app server for the deploy user."
  type        = string
}

variable "admin_ssh_cidrs" {
  description = "Source CIDRs allowed to SSH into the app server (e.g. [\"203.0.113.4/32\"]). No default — see modules/network."
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

variable "monitor_target" {
  description = "FQDN checked by the external uptime monitor."
  type        = string
  default     = "staging.levelog.matsu0122.com"
}

variable "monitor_slack_webhook" {
  description = "Slack incoming webhook URL for alert notifications. No default — supply via a secret-injection mechanism, never in terraform.tfvars."
  type        = string
  default     = ""
  sensitive   = true
}

variable "dns_zone" {
  description = "Sakura Cloud DNS zone the A record goes in (created by terraform/environments/dns)."
  type        = string
  default     = "levelog.matsu0122.com"
}
