# The levelog.matsu0122.com zone, delegated from matsu0122.com. The parent
# zone stays on Vercel; only this subdomain is handed to Sakura Cloud DNS,
# via NS records added on Vercel by hand (see docs/tls-design.md section 6).
# Having the zone on Sakura Cloud is what lets certbot-dns-sakuracloud do
# DNS-01 with a Sakura API key instead of a Vercel token that could control
# the whole Vercel account.
#
# Its own root module and state, separate from staging and production:
# both environments add records to this one zone, and neither should be
# able to delete it (destroying staging must never take production's
# records down with it). Apply this first; the environments look the zone
# up by name.
resource "sakuracloud_dns" "levelog" {
  zone        = var.dns_zone
  description = "Levelog (delegated from matsu0122.com on Vercel)"
  tags        = ["levelog", "dns"]

  # No inline `record` blocks on purpose. Records live in
  # sakuracloud_dns_record resources in each environment, and certbot adds
  # and removes _acme-challenge TXT records here at runtime. `record` is
  # Optional+Computed in the provider, so leaving it unset means Terraform
  # neither owns nor tries to remove any of them.
}
