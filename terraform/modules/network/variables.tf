variable "name_prefix" {
  description = "Prefix applied to every resource name created by this module (e.g. \"levelog-staging\")."
  type        = string
}

variable "description" {
  description = "Description applied to the switch."
  type        = string
  default     = ""
}

variable "internal_cidr" {
  description = "CIDR of the internal (non-routed) network app servers and the database share. Must not overlap between environments if they ever end up in the same zone."
  type        = string
  default     = "192.168.100.0/24"
}

variable "admin_ssh_cidrs" {
  description = "Source CIDRs allowed to reach app servers on port 22. No default on purpose — an empty/wildcard allowlist here would defeat the point of restricting SSH, so every environment must state its admin source IPs explicitly."
  type        = list(string)

  validation {
    condition     = length(var.admin_ssh_cidrs) > 0
    error_message = "admin_ssh_cidrs must not be empty — SSH would otherwise be unreachable, or (if someone \"fixes\" that with 0.0.0.0/0) unrestricted."
  }
}

variable "web_allowed_source_cidrs" {
  description = "Source CIDRs allowed to reach app servers on 80/443. Defaults to the whole internet (correct for staging, which has no load balancer in front). Production should narrow this to the load balancer's address once it exists (see terraform/README.md) so app servers can no longer be reached directly, bypassing the LB's health-check-based routing."
  type        = list(string)
  default     = ["0.0.0.0/0"]
}
