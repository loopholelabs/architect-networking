variable "architect_license_key" {
  description = "License key for Architect NAT"
  type        = string
  sensitive   = true
}

variable "nat_version" {
  description = "Version of Architect NAT to deploy"
  type        = string
  default     = "latest"
}