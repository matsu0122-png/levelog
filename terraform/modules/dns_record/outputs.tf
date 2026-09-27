output "fqdn" {
  description = "Fully qualified name of the record."
  value       = var.name == "@" ? var.zone : "${var.name}.${var.zone}"
}
