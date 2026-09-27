variable "sakura_zone" {
  description = "Sakura Cloud zone for the provider. DNS zones are global, so this does not affect where the zone lives."
  type        = string
  default     = "tk1a"
}

variable "dns_zone" {
  description = "The zone delegated from matsu0122.com (managed on Vercel) to Sakura Cloud DNS. staging/production add their A records to it by this name."
  type        = string
  default     = "levelog.matsu0122.com"
}
