output "app_server_public_ips" {
  description = "Public IP addresses of the production app servers."
  value       = module.app_server.public_ip_addresses
}

output "database_id" {
  description = "ID of the production database appliance."
  value       = module.database.database_id
}

output "load_balancer_vip" {
  description = "The virtual IP DNS points at."
  value       = module.load_balancer.vip_address
}
