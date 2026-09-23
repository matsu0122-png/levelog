data "sakuracloud_archive" "os" {
  os_type = var.os_type
}

resource "sakuracloud_ssh_key" "deploy" {
  name       = "${var.name_prefix}-deploy-key"
  public_key = var.ssh_public_key
}

# Runs once at first boot (Sakura Cloud's startup-script mechanism): OS
# hardening, Docker, the deploy user, node_exporter, and Grafana Alloy
# (ships node_exporter/app metrics and container logs to Grafana Cloud —
# see docs/monitoring-design.md for why and docs/app-server-design.md for
# the rest of this script's rationale). Actually deploying the app itself
# happens later, via CD (phase 13).
resource "sakuracloud_note" "provision" {
  name  = "${var.name_prefix}-app-provision"
  class = "shell"
  content = templatefile("${path.module}/templates/startup.sh.tftpl", {
    deploy_user              = var.deploy_user
    ssh_public_key           = var.ssh_public_key
    environment_name         = var.environment_name
    admin_ssh_cidrs          = var.admin_ssh_cidrs
    web_allowed_source_cidrs = var.web_allowed_source_cidrs
  })
}

resource "sakuracloud_disk" "boot" {
  count = var.server_count

  name              = "${var.name_prefix}-app-${count.index + 1}-disk"
  plan              = "ssd"
  connector         = "virtio"
  size              = var.disk_size_gb
  source_archive_id = data.sakuracloud_archive.os.id
}

# Weekly whole-disk snapshot of each app server's boot disk. This is a
# backstop for the handful of files on this disk that are neither in git
# nor reproducible by re-running Terraform/the startup script: .env
# (docs/deployment-design.md section 5), monitoring.env
# (docs/monitoring-design.md section 4), and the TLS private key
# (docs/tls-design.md) — all deliberately created out-of-band by a manual
# step, precisely so CI/Terraform never handle them, which also means
# nothing else has a copy. Restoring from this creates a new disk (doesn't
# modify the running one) — see docs/backup-restore-design.md for the
# runbook. A whole-disk snapshot is coarser than backing up just those
# files, but reuses an existing declarative resource instead of building
# and operating a separate secret-backup path — consistent with this
# project's general bias (Grafana Cloud over self-hosting, GHCR over a
# separate registry, etc.).
resource "sakuracloud_auto_backup" "boot" {
  count = var.server_count

  name           = "${var.name_prefix}-app-${count.index + 1}-autobackup"
  disk_id        = sakuracloud_disk.boot[count.index].id
  weekdays       = var.auto_backup_weekdays
  max_backup_num = var.auto_backup_max_generations
  tags           = var.tags
}

# Two NICs per server: "shared" is Sakura Cloud's public internet segment
# (production reaches this only through the load balancer; staging, which
# has no load balancer, is reached here directly), and the internal switch
# is used only for reaching the database — it has no internet uplink at all
# (see modules/network). Static internal addressing and any further
# hardening (disabling the public NIC where unnecessary, etc.) is finalized
# once the network/app-server phases work out real routing against Sakura
# Cloud's zone.
resource "sakuracloud_server" "app" {
  count = var.server_count

  name   = "${var.name_prefix}-app-${count.index + 1}"
  core   = var.core
  memory = var.memory_gb
  disks  = [sakuracloud_disk.boot[count.index].id]
  tags   = var.tags

  network_interface {
    upstream         = "shared"
    packet_filter_id = var.public_packet_filter_id
  }

  network_interface {
    upstream = var.internal_switch_id
  }

  disk_edit_parameter {
    hostname        = "${var.name_prefix}-app-${count.index + 1}"
    disable_pw_auth = true
    ssh_key_ids     = [sakuracloud_ssh_key.deploy.id]
    note_ids        = [sakuracloud_note.provision.id]
  }
}
