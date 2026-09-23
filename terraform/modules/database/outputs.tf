output "database_id" {
  description = "ID of the primary database appliance."
  value       = sakuracloud_database.primary.id
}

output "ip_address" {
  description = "Internal IP address of the primary database appliance."
  value       = var.ip_address
}

output "app_role_name" {
  description = "Name of the restricted, DML-only role the running application should connect as."
  value       = postgresql_role.app.name
}

output "database_name" {
  description = "Name of the Postgres database the application should connect to."
  value       = var.database_name
}
