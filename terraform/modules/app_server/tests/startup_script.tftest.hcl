# Runs with `terraform test` (mocked provider, no credentials). Guards the
# Sakura Cloud limits the first real apply ran into.
mock_provider "sakuracloud" {}

variables {
  name_prefix             = "levelog-production"
  server_count            = 2
  internal_switch_id      = "113500000001"
  public_packet_filter_id = "113500000002"
  ssh_public_key          = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIC9IAyfEj1YZwvcEa+FgINySqin2cHmYLXqwiVlzc2Tb levelog-deploy"
  admin_ssh_cidrs         = ["150.91.132.182/32", "203.0.113.0/24"]
  environment_name        = "production"
}

override_data {
  target = data.sakuracloud_archive.os
  values = {
    id = "113700000001"
  }
}

run "startup_script_fits_note_limit" {
  command = plan

  # Sakura Cloud: "String length must be: Note.Content<=10000".
  assert {
    condition     = length(sakuracloud_note.provision.content) <= 10000
    error_message = "startup script exceeds Sakura Cloud's 10000-character note limit"
  }
}

run "comment_stripping_keeps_code" {
  command = plan

  assert {
    condition     = startswith(sakuracloud_note.provision.content, "#!/bin/bash\n")
    error_message = "the script's own shebang must survive comment stripping"
  }
  assert {
    condition     = length(regexall("(?m)^#!/bin/bash$", sakuracloud_note.provision.content)) == 3
    error_message = "shebangs of the scripts written by heredocs (certbot hook, deploy.sh) must survive too"
  }
  assert {
    condition     = length(regexall("(?m)^\\s*# ", sakuracloud_note.provision.content)) == 0
    error_message = "whole-line comments should be stripped from the uploaded copy"
  }
  assert {
    condition     = strcontains(sakuracloud_note.provision.content, "ufw allow from 150.91.132.182/32 to any port 22 proto tcp")
    error_message = "rendered ufw rules must still be present"
  }
  assert {
    condition     = strcontains(sakuracloud_note.provision.content, "sed -i 's/^#\\?PermitRootLogin.*/PermitRootLogin no/' /etc/ssh/sshd_config")
    error_message = "code lines containing '#' must not be stripped"
  }
}
