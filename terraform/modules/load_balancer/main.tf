# Dedicated public IP block for the load balancer (a "Switch+Router" in
# Sakura Cloud terms). Distinct from modules/network's internal switch —
# this one has real internet routing, which is exactly what the LB (and
# only the LB) needs to be reachable from the internet.
resource "sakuracloud_internet" "public" {
  name       = "${var.name_prefix}-lb-internet"
  netmask    = var.public_netmask
  band_width = var.band_width
}

# Index 0/1 go to the active/standby pair below; index 2 (the third
# assigned address) becomes the VIP. Sakura Cloud reserves the network,
# broadcast, and gateway addresses from the block before populating
# ip_addresses, so this should be a genuinely assignable address — but
# confirm against this module's assigned_ip_addresses output after the
# first real apply before treating it as final (e.g. before pointing DNS
# at it).
locals {
  vip_address = sakuracloud_internet.public.ip_addresses[2]
}

resource "sakuracloud_load_balancer" "main" {
  name = "${var.name_prefix}-lb"
  plan = var.plan

  network_interface {
    switch_id    = sakuracloud_internet.public.switch_id
    ip_addresses = slice(sakuracloud_internet.public.ip_addresses, 0, 2)
    netmask      = sakuracloud_internet.public.netmask
    gateway      = sakuracloud_internet.public.gateway
    vrid         = var.vrid
  }

  # One vip block per port (80 and 443 by default): port 80 exists only so
  # a plain-HTTP request reaches a backend at all (which then redirects to
  # HTTPS — see frontend/nginx.conf) instead of the connection being
  # refused outright. Both health-check /health/ready, served unredirected
  # on both plain HTTP and HTTPS so the port-80 check doesn't just see a
  # 301 and mark the backend down.
  dynamic "vip" {
    for_each = var.vip_ports
    content {
      vip  = local.vip_address
      port = vip.value

      dynamic "server" {
        for_each = var.backend_ip_addresses
        content {
          ip_address = server.value
          protocol   = vip.value == 443 ? "https" : "http"
          path       = var.health_check_path
          enabled    = true
        }
      }
    }
  }
}
