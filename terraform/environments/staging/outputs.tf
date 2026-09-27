output "app_server_public_ips" {
  description = "Public IP address(es) of the staging app server(s)."
  value       = module.app_server.public_ip_addresses
}

output "database_id" {
  description = "ID of the staging database appliance."
  value       = module.database.database_id
}

output "dns_fqdn" {
  description = "Hostname this environment's A record answers for."
  value       = module.dns.fqdn
}
