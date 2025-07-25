##############################
# ENIs & FLOATING PRIVATE IP
##############################

# Primary ENI for blue instance (for SSH/management)
resource "aws_network_interface" "eni_blue_primary" {
  subnet_id         = aws_subnet.architect.id
  private_ips       = [local.primary_blue_ip]
  security_groups   = concat([aws_security_group.base.id], var.extra_security_group_ids)
  source_dest_check = true  # Normal check for management interface
  tags              = merge(local.merged_tags, { Name = "${var.name}-eni-blue-primary" })
}

# NAT ENI for blue instance (with floating IP)
resource "aws_network_interface" "eni_blue_nat" {
  subnet_id         = aws_subnet.architect.id
  private_ips       = [local.floating_ip]
  security_groups   = concat([aws_security_group.base.id], var.extra_security_group_ids)
  source_dest_check = false  # Required for NAT
  tags              = merge(local.merged_tags, { Name = "${var.name}-eni-blue-nat" })
}

# Primary ENI for red instance (for SSH/management)
resource "aws_network_interface" "eni_red_primary" {
  subnet_id         = aws_subnet.architect.id
  private_ips       = [local.primary_red_ip]
  security_groups   = concat([aws_security_group.base.id], var.extra_security_group_ids)
  source_dest_check = true  # Normal check for management interface
  tags              = merge(local.merged_tags, { Name = "${var.name}-eni-red-primary" })
}

# NAT ENI for red instance (placeholder for when floating IP moves)
resource "aws_network_interface" "eni_red_nat" {
  subnet_id         = aws_subnet.architect.id
  security_groups   = concat([aws_security_group.base.id], var.extra_security_group_ids)
  source_dest_check = false  # Required for NAT
  tags              = merge(local.merged_tags, { Name = "${var.name}-eni-red-nat" })
}

##############################
# EIPs
##############################

resource "aws_eip" "auto" {
  count  = length(var.eip_allocation_ids) == 0 ? var.eip_count : 0
  domain = "vpc"
  tags   = merge(local.merged_tags, { Name = "${var.name}-eip-${count.index}" })
}

resource "aws_eip_association" "floating" {
  for_each = { for idx, id in local.effective_eip_ids : idx => id }

  allocation_id        = each.value
  network_interface_id = aws_network_interface.eni_blue_nat.id
  private_ip_address   = local.floating_ip
  allow_reassociation  = true
}