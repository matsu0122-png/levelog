# External ("外形") uptime monitoring: checks /health/live from outside the
# network, independent of whatever's happening to any one backend server —
# this is deliberately not the same signal as the load balancer's own
# per-backend health check (which uses /health/ready). Metrics/log-based
# monitoring (CPU, 5xx rate, DB connections, etc.) is a separate resource
# set, added alongside this one once that monitoring stack is chosen.
resource "sakuracloud_simple_monitor" "uptime" {
  target               = var.target
  delay_loop           = var.delay_loop_seconds
  notify_email_enabled = var.notify_email_enabled
  notify_slack_enabled = var.notify_slack_webhook != ""
  notify_slack_webhook = var.notify_slack_webhook != "" ? var.notify_slack_webhook : null

  health_check {
    protocol = var.protocol
    path     = var.path
    status   = "200"
  }
}

# Certificate expiry, checked from outside like the uptime check above.
# Renewal itself is automated on every app server (certbot.timer + the
# deploy hook installed by terraform/modules/app_server — see
# docs/tls-design.md section 4), so this firing means that automation has
# silently stopped working, with `cert_remaining_days` left to fix it by
# hand. Let's Encrypt certs are renewed at 30 days remaining, so a
# threshold below that never fires while renewal is healthy.
resource "sakuracloud_simple_monitor" "cert_expiry" {
  target               = var.target
  delay_loop           = var.cert_check_delay_loop_seconds
  notify_email_enabled = var.notify_email_enabled
  notify_slack_enabled = var.notify_slack_webhook != ""
  notify_slack_webhook = var.notify_slack_webhook != "" ? var.notify_slack_webhook : null

  health_check {
    protocol       = "sslcertificate"
    remaining_days = var.cert_remaining_days
    verify_sni     = true
  }
}
