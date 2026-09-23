# Private switch: app servers and the database appliance attach to this and
# nothing else. It has no router/internet uplink, so it cannot be reached
# from the public internet by construction (satisfies "DBは内部ネットワーク
# からのみ接続" — this alone, before any packet filter, is what keeps the
# database off the internet).
resource "sakuracloud_switch" "internal" {
  name        = "${var.name_prefix}-internal-switch"
  description = var.description != "" ? var.description : "${var.name_prefix} internal network (app <-> db)"
}

# Inbound filter for the app servers' public-facing ("shared") NIC.
# Rule order doesn't affect correctness here (every rule below is an allow,
# and nothing overlaps another rule's port), but SSH is listed first as the
# most security-sensitive rule. Anything not explicitly allowed is dropped
# by the implicit final deny — there is no catch-all allow rule.
resource "sakuracloud_packet_filter" "app_public" {
  name        = "${var.name_prefix}-app-public"
  description = "Public-facing NIC filter for app servers: SSH from admin IPs only, HTTP/HTTPS from web_allowed_source_cidrs, ICMP; everything else denied"

  dynamic "expression" {
    for_each = var.admin_ssh_cidrs
    content {
      protocol         = "tcp"
      source_network   = expression.value
      destination_port = "22"
      allow            = true
      description      = "SSH from admin CIDR ${expression.value}"
    }
  }

  dynamic "expression" {
    for_each = var.web_allowed_source_cidrs
    content {
      protocol         = "tcp"
      source_network   = expression.value
      destination_port = "80"
      allow            = true
    }
  }

  dynamic "expression" {
    for_each = var.web_allowed_source_cidrs
    content {
      protocol         = "tcp"
      source_network   = expression.value
      destination_port = "443"
      allow            = true
    }
  }

  expression {
    protocol = "icmp"
    allow    = true
  }
}
