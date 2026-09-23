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
