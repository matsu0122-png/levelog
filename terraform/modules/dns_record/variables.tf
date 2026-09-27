variable "zone" {
  description = "Name of the existing Sakura Cloud DNS zone (created by terraform/environments/dns)."
  type        = string
}

variable "name" {
  description = "Record name relative to the zone: \"@\" for the zone apex, e.g. \"staging\" for staging.<zone>."
  type        = string
}

variable "ipv4_address" {
  description = "Address the A record points at."
  type        = string
}

variable "ttl" {
  description = "TTL in seconds. Short, so a cut-over or rollback propagates quickly (docs/tls-design.md section 6)."
  type        = number
  default     = 300
}
