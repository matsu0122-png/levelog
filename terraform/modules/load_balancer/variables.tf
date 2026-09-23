variable "name_prefix" {
  description = "Prefix applied to every resource name (e.g. \"levelog-production\")."
  type        = string
}

variable "plan" {
  description = "Load balancer plan (verify the exact value against the current sakuracloud provider/plan catalog before the first real apply)."
  type        = string
  default     = "standard"
}

variable "public_netmask" {
  description = "Netmask (prefix length) of the dedicated public IP block this module provisions via sakuracloud_internet. Must be one of 26/27/28 (the smallest, 28, gives 16 addresses total — enough for the active/standby pair + VIP with room to spare)."
  type        = number
  default     = 28
}

variable "band_width" {
  description = "Bandwidth (Mbps) of the provisioned public IP block."
  type        = number
  default     = 100
}

variable "vrid" {
  description = "VRRP router ID — must be unique within the routed switch's broadcast domain."
  type        = number
}

variable "vip_ports" {
  description = "Ports the VIP listens on. 80 is included so plain-HTTP requests still reach a backend (which redirects to HTTPS) instead of being refused outright; 443 is the real traffic path."
  type        = list(number)
  default     = [80, 443]
}

variable "backend_ip_addresses" {
  description = "Public IP addresses of the app servers to load-balance across (from the app_server module's public_ip_addresses output)."
  type        = list(string)
}

variable "health_check_path" {
  description = "HTTP(S) path used for backend health checks — /health/ready so a backend with a broken DB connection is taken out of rotation without being marked fully down. Served unredirected on both plain HTTP and HTTPS (see frontend/nginx.conf) so the port-80 health check doesn't just see a 301."
  type        = string
  default     = "/health/ready"
}
