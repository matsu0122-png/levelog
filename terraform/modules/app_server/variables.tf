variable "name_prefix" {
  description = "Prefix applied to every resource name (e.g. \"levelog-staging\")."
  type        = string
}

variable "server_count" {
  description = "Number of app servers to create (1 for staging, 2+ for production rolling deploys)."
  type        = number
  default     = 1
}

variable "core" {
  description = "vCPU core count per app server."
  type        = number
  default     = 2
}

variable "memory_gb" {
  description = "Memory (GB) per app server."
  type        = number
  default     = 4
}

variable "disk_size_gb" {
  description = "Boot disk size (GB) per app server."
  type        = number
  default     = 20
}

variable "internal_switch_id" {
  description = "ID of the internal switch (from the network module) used to reach the database."
  type        = string
}

variable "public_packet_filter_id" {
  description = "Packet filter applied to each server's public (\"shared\") NIC."
  type        = string
}

variable "ssh_public_key" {
  description = "SSH public key installed for the deploy user. Password authentication is disabled."
  type        = string
}

variable "deploy_user" {
  description = "Name of the non-root user created at boot for deployment/operations, with docker group membership (not full sudo)."
  type        = string
  default     = "deploy"
}

variable "os_type" {
  description = "Base OS image identifier for the source archive lookup (verify the exact value against the current sakuracloud provider/archive catalog before the first real apply)."
  type        = string
  default     = "ubuntu2404"
}

variable "tags" {
  description = "Tags applied to every app server (e.g. environment name, for the load balancer / monitoring modules to select by)."
  type        = list(string)
  default     = []
}

variable "admin_ssh_cidrs" {
  description = "Source CIDRs allowed to reach app servers on port 22 — rendered into the host ufw rules (startup.sh.tftpl) so they match modules/network's packet filter allow-list for the same port. No default, same reasoning as modules/network's identically-named variable: an empty/wildcard allowlist here would defeat the point."
  type        = list(string)

  validation {
    condition     = length(var.admin_ssh_cidrs) > 0
    error_message = "admin_ssh_cidrs must not be empty — SSH would otherwise be unreachable, or (if someone \"fixes\" that with 0.0.0.0/0) unrestricted."
  }
}

variable "web_allowed_source_cidrs" {
  description = "Source CIDRs allowed to reach app servers on 80/443 — rendered into the host ufw rules (startup.sh.tftpl) so they match modules/network's packet filter allow-list for the same ports. Defaults to the whole internet (correct for staging, which has no load balancer in front); production should pass the load balancer's real addresses, same value as its modules/network call."
  type        = list(string)
  default     = ["0.0.0.0/0"]
}

variable "auto_backup_weekdays" {
  description = "Weekdays the boot disk's whole-disk snapshot backup runs on (see sakuracloud_auto_backup.boot). Weekly (one day) by default — this disk's contents change rarely (mostly at initial manual setup; app deploys go through CD/GHCR, not this disk)."
  type        = list(string)
  default     = ["sun"]
}

variable "auto_backup_max_generations" {
  description = "Number of past backup generations to retain per app server (sakuracloud_auto_backup's max_backup_num, provider-enforced range 1-10)."
  type        = number
  default     = 2
}

variable "environment_name" {
  description = "Non-secret environment label (\"staging\"/\"production\") baked into the Grafana Alloy config as the `environment` label on every metric/log series it ships — lets one Grafana Cloud account distinguish the two environments. Not a secret: the actual Grafana Cloud endpoint/credentials are supplied out-of-band via /etc/levelog/monitoring.env (see docs/monitoring-design.md), never through Terraform."
  type        = string
}
