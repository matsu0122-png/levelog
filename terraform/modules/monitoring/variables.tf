variable "name_prefix" {
  description = "Prefix applied to every resource name (e.g. \"levelog-production\")."
  type        = string
}

variable "target" {
  description = "FQDN or IP address to monitor from outside (external uptime check) — the environment's public domain."
  type        = string
}

variable "protocol" {
  description = "Health check protocol."
  type        = string
  default     = "https"
}

variable "path" {
  description = "HTTP(S) path checked for external uptime monitoring."
  type        = string
  default     = "/health/live"
}

variable "delay_loop_seconds" {
  description = "Seconds between checks."
  type        = number
  default     = 60
}

variable "notify_email_enabled" {
  description = "Send an email notification on state change."
  type        = bool
  default     = true
}

variable "notify_slack_webhook" {
  description = "Slack incoming webhook URL for alert notifications. No default on purpose — must be supplied via a secret-injection mechanism, never committed to .tfvars."
  type        = string
  default     = ""
  sensitive   = true
}

variable "cert_remaining_days" {
  description = "Alert when the served TLS certificate has fewer than this many days left. Keep it below 30 (when certbot renews) so it only fires if renewal is broken."
  type        = number
  default     = 14
}

variable "cert_check_delay_loop_seconds" {
  description = "Seconds between certificate expiry checks. Expiry moves in days, so this does not need the uptime check's frequency."
  type        = number
  default     = 3600
}
