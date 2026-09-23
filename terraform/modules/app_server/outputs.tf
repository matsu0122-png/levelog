output "server_ids" {
  description = "IDs of the created app servers."
  value       = sakuracloud_server.app[*].id
}

output "public_ip_addresses" {
  description = "Public IP addresses of the app servers (shared NIC)."
  value       = sakuracloud_server.app[*].ip_address
}
