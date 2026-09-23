output "switch_id" {
  description = "ID of the internal switch that app servers and the database attach to."
  value       = sakuracloud_switch.internal.id
}

output "app_public_packet_filter_id" {
  description = "ID of the packet filter for app servers' public-facing NIC."
  value       = sakuracloud_packet_filter.app_public.id
}

output "internal_gateway" {
  description = "Gateway address on the internal network (first usable host)."
  value       = cidrhost(var.internal_cidr, 1)
}

output "internal_netmask" {
  description = "Netmask (prefix length) of the internal network."
  value       = tonumber(split("/", var.internal_cidr)[1])
}

output "internal_cidr" {
  description = "The internal network's CIDR, passed through for callers that need to compute host addresses."
  value       = var.internal_cidr
}
