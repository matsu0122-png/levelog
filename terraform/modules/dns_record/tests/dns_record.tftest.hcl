# Runs with `terraform test` (no Sakura Cloud credentials needed: the
# provider is mocked, and the zone lookup is overridden per case).
mock_provider "sakuracloud" {}

variables {
  zone         = "levelog.matsu0122.com"
  ipv4_address = "192.0.2.10"
}

run "apex_record_points_at_given_address" {
  command = plan

  variables {
    name = "@"
  }

  override_data {
    target = data.sakuracloud_dns.zone
    values = {
      id   = "113500000001"
      zone = "levelog.matsu0122.com"
    }
  }

  assert {
    condition     = sakuracloud_dns_record.a.dns_id == "113500000001"
    error_message = "record must be created in the zone that was looked up"
  }
  assert {
    condition     = sakuracloud_dns_record.a.type == "A" && sakuracloud_dns_record.a.value == "192.0.2.10"
    error_message = "expected an A record for the given address"
  }
  assert {
    condition     = sakuracloud_dns_record.a.ttl == 300
    error_message = "default TTL should be 300s"
  }
  assert {
    condition     = output.fqdn == "levelog.matsu0122.com"
    error_message = "apex record's fqdn should be the zone itself"
  }
}

run "subdomain_record_fqdn" {
  command = plan

  variables {
    name = "staging"
  }

  override_data {
    target = data.sakuracloud_dns.zone
    values = {
      id   = "113500000001"
      zone = "levelog.matsu0122.com"
    }
  }

  assert {
    condition     = output.fqdn == "staging.levelog.matsu0122.com"
    error_message = "subdomain fqdn should be <name>.<zone>"
  }
}

# The name filter is a partial match: a different zone that merely
# contains the name must be rejected, not silently used.
run "rejects_partial_zone_match" {
  command = plan

  variables {
    name = "staging"
  }

  override_data {
    target = data.sakuracloud_dns.zone
    values = {
      id   = "113500000002"
      zone = "staging.levelog.matsu0122.com"
    }
  }

  expect_failures = [data.sakuracloud_dns.zone]
}
