# Runs with `terraform test` (mocked provider, no credentials).
mock_provider "sakuracloud" {}

variables {
  name_prefix              = "levelog-staging"
  admin_ssh_cidrs          = ["150.91.132.182/32", "203.0.113.0/24"]
  web_allowed_source_cidrs = ["0.0.0.0/0"]
}

run "single_hosts_are_passed_without_mask" {
  command = plan

  # Sakura Cloud: "SourceNetwork のマスク長は0〜31の範囲で指定してください".
  assert {
    condition     = sakuracloud_packet_filter.app_public.expression[0].source_network == "150.91.132.182"
    error_message = "a /32 admin address must be sent as a bare IP"
  }
  assert {
    condition     = sakuracloud_packet_filter.app_public.expression[1].source_network == "203.0.113.0/24"
    error_message = "real CIDR ranges must be kept as-is"
  }
  assert {
    condition     = sakuracloud_packet_filter.app_public.expression[2].source_network == "0.0.0.0/0"
    error_message = "0.0.0.0/0 must be kept as-is"
  }
}

run "ends_with_deny_all_after_return_traffic" {
  command = plan

  # Unmatched packets are allowed by Sakura Cloud, so the list only
  # restricts anything if it ends in an explicit deny.
  assert {
    condition = (
      sakuracloud_packet_filter.app_public.expression[length(sakuracloud_packet_filter.app_public.expression) - 1].protocol == "ip" &&
      sakuracloud_packet_filter.app_public.expression[length(sakuracloud_packet_filter.app_public.expression) - 1].allow == false
    )
    error_message = "last rule must be an explicit deny-all"
  }
  assert {
    condition = length([
      for e in sakuracloud_packet_filter.app_public.expression : e
      if e.protocol == "tcp" && e.destination_port == "32768-61000" && e.allow
    ]) == 1
    error_message = "return traffic on ephemeral TCP ports must be allowed (stateless filter)"
  }
}
