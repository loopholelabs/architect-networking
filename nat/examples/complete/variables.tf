variable "architect_license_key" {
  description = "License key for Architect NAT"
  type        = string
  sensitive   = true
}

variable "nat_version" {
  description = "Version of Architect NAT to deploy"
  type        = string
  default     = "sha-6456256"
}

variable "name" {
  description = "Deployment Name"
  type        = string
  sensitive   = true
}