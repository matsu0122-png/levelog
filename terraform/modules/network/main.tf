# Private switch: app servers and the database appliance attach to this and
# nothing else. It has no router/internet uplink, so it cannot be reached
# from the public internet by construction (satisfies "DBは内部ネットワーク
# からのみ接続" — this alone, before any packet filter, is what keeps the
# database off the internet).
resource "sakuracloud_switch" "internal" {
  name        = "${var.name_prefix}-internal-switch"
  description = var.description != "" ? var.description : "${var.name_prefix} internal network (app <-> db)"
}

# Sakura Cloud packet filters take a single host as a bare address and
# reject a "/32" mask (valid masks are 0-31), while ufw and the rest of this
# config speak CIDR. Normalize here so callers can keep passing "x.x.x.x/32".
locals {
  filter_sources = {
    for cidr in distinct(concat(var.admin_ssh_cidrs, var.web_allowed_source_cidrs)) :
    cidr => trimsuffix(cidr, "/32")
  }
}

# Inbound filter for the app servers' public-facing ("shared") NIC.
#
# Two properties of Sakura Cloud packet filters shape this (phase 21 fix —
# the original version assumed the opposite of both):
#   - A packet that matches no rule is ALLOWED. Only the explicit "deny all"
#     rule at the end makes this an allow-list; without it the SSH
#     restriction above it would restrict nothing.
#   - Filters are stateless. Replies to connections the server itself opens
#     (apt, Docker Hub/GHCR pulls, certbot, DNS lookups, Grafana Cloud
#     pushes) arrive on the Linux ephemeral port range and need their own
#     allow rules, as do IP fragments. Same set as the provider's own
#     example (sacloud/terraform-provider-sakuracloud, packet_filter docs).
# Rules are evaluated in order, first match wins, so "deny all" must stay
# last.
resource "sakuracloud_packet_filter" "app_public" {
  name        = "${var.name_prefix}-app-public"
  description = "Public-facing NIC filter for app servers: SSH from admin IPs only, HTTP/HTTPS from web_allowed_source_cidrs, ICMP; everything else denied"

  dynamic "expression" {
    for_each = var.admin_ssh_cidrs
    content {
      protocol         = "tcp"
      source_network   = local.filter_sources[expression.value]
      destination_port = "22"
      allow            = true
      description      = "SSH from admin CIDR ${expression.value}"
    }
  }

  dynamic "expression" {
    for_each = var.web_allowed_source_cidrs
    content {
      protocol         = "tcp"
      source_network   = local.filter_sources[expression.value]
      destination_port = "80"
      allow            = true
    }
  }

  dynamic "expression" {
    for_each = var.web_allowed_source_cidrs
    content {
      protocol         = "tcp"
      source_network   = local.filter_sources[expression.value]
      destination_port = "443"
      allow            = true
    }
  }

  expression {
    protocol = "icmp"
    allow    = true
  }

  expression {
    protocol    = "fragment"
    allow       = true
    description = "IP fragments (stateless filter)"
  }

  # Return traffic for outbound connections (Linux ephemeral ports,
  # net.ipv4.ip_local_port_range = 32768-60999).
  expression {
    protocol         = "tcp"
    destination_port = "32768-61000"
    allow            = true
    description      = "Return traffic (TCP ephemeral ports)"
  }

  expression {
    protocol         = "udp"
    destination_port = "32768-61000"
    allow            = true
    description      = "Return traffic (UDP ephemeral ports, e.g. DNS replies)"
  }

  # DHCP replies: without this the NIC can lose its address on lease
  # renewal once "deny all" below is in place.
  expression {
    protocol         = "udp"
    source_port      = "67"
    destination_port = "68"
    allow            = true
    description      = "DHCP replies"
  }

  expression {
    protocol    = "ip"
    allow       = false
    description = "Deny all (must stay last: unmatched packets are otherwise allowed)"
  }
}
