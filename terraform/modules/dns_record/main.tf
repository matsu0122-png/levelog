# One A record in the shared levelog zone. The zone itself belongs to
# terraform/environments/dns, so this looks it up by name instead of
# taking an ID: each environment stays a self-contained root module with
# no cross-state references, and a missing zone fails at plan time with
# "not found" rather than silently creating a second one.
data "sakuracloud_dns" "zone" {
  filter {
    names = [var.zone]
  }

  # Sakura Cloud's name filter is a partial match and the data source takes
  # the first hit, so without this e.g. a future "staging.levelog..." zone
  # could be picked up instead.
  lifecycle {
    postcondition {
      condition     = self.zone == var.zone
      error_message = "Found DNS zone \"${self.zone}\", expected exactly \"${var.zone}\". Apply terraform/environments/dns first."
    }
  }
}

resource "sakuracloud_dns_record" "a" {
  dns_id = data.sakuracloud_dns.zone.id
  name   = var.name
  type   = "A"
  value  = var.ipv4_address
  ttl    = var.ttl
}
