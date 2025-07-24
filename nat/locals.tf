##############################
# LOCALS
##############################

locals {
  merged_tags = merge({
    "Project"   = var.name,
    "Terraform" = "true"
  }, var.tags)

  primary_blue_ip = cidrhost(var.architect_subnet_cidr, 10)
  primary_red_ip  = cidrhost(var.architect_subnet_cidr, 11)
  floating_ip     = cidrhost(var.architect_subnet_cidr, 12)

  effective_eip_ids = length(var.eip_allocation_ids) == 0 ? [for e in aws_eip.auto : e.id] : var.eip_allocation_ids
}

##############################
# USER-DATA SCRIPT
##############################

locals {
  userdata = templatefile("${path.module}/userdata.tftpl", {
    license_key              = var.license_key
    nat_version              = var.nat_version
    enable_cloudwatch_agent  = var.enable_cloudwatch_agent
    name                     = var.name
  })
}