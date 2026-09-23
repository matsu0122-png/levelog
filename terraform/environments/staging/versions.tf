terraform {
  required_version = ">= 1.5.0"

  required_providers {
    sakuracloud = {
      source  = "sacloud/sakuracloud"
      version = "~> 2.25"
    }
    postgresql = {
      source  = "cyrilgdn/postgresql"
      version = "~> 1.21"
    }
  }

  # Local backend for now (state file lives at terraform.tfstate next to
  # this configuration). Once the remote state bucket exists, replace this
  # with the s3-compatible backend pointed at Sakura Cloud object storage —
  # see terraform/README.md for the bootstrapping steps and why this can't
  # be wired up before that bucket is provisioned.
  #
  # backend "s3" {
  #   bucket                      = "levelog-terraform-state"
  #   key                         = "staging/terraform.tfstate"
  #   region                      = "jp-north-1"
  #   endpoints                   = { s3 = "https://s3.isk01.sakurastorage.jp" }
  #   skip_credentials_validation = true
  #   skip_region_validation      = true
  #   skip_requesting_account_id  = true
  #   skip_s3_checksum            = true
  #   use_path_style              = true
  # }
}

# Credentials come from the standard SAKURACLOUD_ACCESS_TOKEN /
# SAKURACLOUD_ACCESS_TOKEN_SECRET environment variables — never put them in
# this file or in terraform.tfvars.
provider "sakuracloud" {
  zone = var.sakura_zone
}
