terraform {
  required_version = ">= 1.5.0"

  required_providers {
    sakuracloud = {
      source  = "sacloud/sakuracloud"
      version = "~> 2.25"
    }
  }

  # Local backend for now, same as staging/production (see
  # terraform/README.md). Its own key when moved to the remote backend:
  #
  # backend "s3" {
  #   bucket                      = "levelog-terraform-state"
  #   key                         = "dns/terraform.tfstate"
  #   region                      = "jp-north-1"
  #   endpoints                   = { s3 = "https://s3.isk01.sakurastorage.jp" }
  #   skip_credentials_validation = true
  #   skip_region_validation      = true
  #   skip_requesting_account_id  = true
  #   skip_s3_checksum            = true
  #   use_path_style              = true
  # }
}

# Credentials: SAKURACLOUD_ACCESS_TOKEN / SAKURACLOUD_ACCESS_TOKEN_SECRET
# environment variables, as in the other environments. DNS zones are a
# global resource, so the zone here only satisfies the provider.
provider "sakuracloud" {
  zone = var.sakura_zone
}
