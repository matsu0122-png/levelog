output "dns_zone_id" {
  description = "ID of the levelog.matsu0122.com zone."
  value       = sakuracloud_dns.levelog.id
}

output "name_servers" {
  description = "Sakura Cloud name servers for this zone. Add one NS record per entry on Vercel (name: levelog) to delegate the subdomain."
  value       = sakuracloud_dns.levelog.dns_servers
}
