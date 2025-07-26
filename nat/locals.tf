##############################
# LOCALS
##############################

locals {
  merged_tags = merge({
    "Project"   = var.name,
    "Terraform" = "true"
  }, var.tags)

  # Management IPs (primary IPs on ENIs)
  management_blue_ip = cidrhost(var.architect_subnet_cidr, 10)
  management_red_ip  = cidrhost(var.architect_subnet_cidr, 11)

  # Floating IPs for NAT (starting at .20)
  floating_ips = [for i in range(var.eip_count) : cidrhost(var.architect_subnet_cidr, 20 + i)]

  # Legacy support
  primary_blue_ip = local.management_blue_ip
  primary_red_ip  = local.management_red_ip
  floating_ip     = length(local.floating_ips) > 0 ? local.floating_ips[0] : cidrhost(var.architect_subnet_cidr, 20)

  effective_eip_ids = length(var.eip_allocation_ids) == 0 ? [for e in aws_eip.auto : e.id] : var.eip_allocation_ids
}

##############################
# USER-DATA SCRIPT
##############################

locals {
  userdata_blue = templatefile("${path.module}/userdata.tftpl", {
    license_key             = var.license_key
    nat_version             = var.nat_version
    enable_cloudwatch_agent = var.enable_cloudwatch_agent
    name                    = var.name
    is_blue                 = true
    management_ip           = local.management_blue_ip
    floating_ips            = local.floating_ips
  })

  userdata_red = templatefile("${path.module}/userdata.tftpl", {
    license_key             = var.license_key
    nat_version             = var.nat_version
    enable_cloudwatch_agent = var.enable_cloudwatch_agent
    name                    = var.name
    is_blue                 = false
    management_ip           = local.management_red_ip
    floating_ips            = local.floating_ips
  })
}