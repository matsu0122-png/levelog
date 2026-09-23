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

  # Local backend for now — see environments/staging/versions.tf and
  # terraform/README.md for why, and the s3-compatible config to switch to
  # once the state bucket exists. Production's key path (production/...)
  # must differ from staging's so they can never collide in the same
  # bucket.
  #
  # backend "s3" {
  #   bucket                      = "levelog-terraform-state"
  #   key                         = "production/terraform.tfstate"
  #   region                      = "jp-north-1"
  #   endpoints                   = { s3 = "https://s3.isk01.sakurastorage.jp" }
  #   skip_credentials_validation = true
  #   skip_region_validation      = true
  #   skip_requesting_account_id  = true
  #   skip_s3_checksum            = true
  #   use_path_style              = true
  # }
}

provider "sakuracloud" {
  zone = var.sakura_zone
}
