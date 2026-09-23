output "load_balancer_id" {
  description = "ID of the load balancer."
  value       = sakuracloud_load_balancer.main.id
}

output "vip_address" {
  description = "The virtual IP DNS should point at. Verify against assigned_ip_addresses after the first real apply before treating it as final."
  value       = local.vip_address
}

output "assigned_ip_addresses" {
  description = "All global IP addresses Sakura Cloud assigned to this module's public IP block — inspect this after the first real apply to confirm vip_address and active_standby_ip_addresses are actually in range."
  value       = sakuracloud_internet.public.ip_addresses
}

output "active_standby_ip_addresses" {
  description = "The two real IPs the load balancer's active/standby pair uses. These are what actually reach app servers, so this is what production's lb_known_ip_addresses variable should be set to (as a follow-up apply — see environments/production/main.tf) to lock the app servers' packet filter down to only the LB."
  value       = slice(sakuracloud_internet.public.ip_addresses, 0, 2)
}

output "assigned_gateway" {
  description = "Gateway address of the assigned public IP block."
  value       = sakuracloud_internet.public.gateway
}
