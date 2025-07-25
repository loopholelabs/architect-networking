##############################
# ENIs WITH MANAGEMENT & FLOATING IPs
##############################

resource "aws_network_interface" "blue" {
  subnet_id         = aws_subnet.architect.id
  private_ip        = local.management_blue_ip  # Explicitly set as primary
  private_ips       = concat([local.management_blue_ip], local.floating_ips)
  security_groups   = concat([aws_security_group.base.id], var.extra_security_group_ids)
  source_dest_check = false
  tags              = merge(local.merged_tags, { Name = "${var.name}-eni-blue" })
}

resource "aws_network_interface" "red" {
  subnet_id         = aws_subnet.architect.id
  private_ip        = local.management_red_ip  # Explicitly set as primary
  private_ips       = [local.management_red_ip]
  security_groups   = concat([aws_security_group.base.id], var.extra_security_group_ids)
  source_dest_check = false
  tags              = merge(local.merged_tags, { Name = "${var.name}-eni-red" })
}

##############################
# MANAGEMENT EIPs
##############################

resource "aws_eip" "management_blue" {
  domain = "vpc"
  tags   = merge(local.merged_tags, { Name = "${var.name}-management-blue" })
}

resource "aws_eip" "management_red" {
  domain = "vpc"
  tags   = merge(local.merged_tags, { Name = "${var.name}-management-red" })
}

resource "aws_eip_association" "management_blue" {
  allocation_id        = aws_eip.management_blue.id
  network_interface_id = aws_network_interface.blue.id
  private_ip_address   = local.management_blue_ip
}

resource "aws_eip_association" "management_red" {
  allocation_id        = aws_eip.management_red.id
  network_interface_id = aws_network_interface.red.id
  private_ip_address   = local.management_red_ip
}

##############################
# NAT EIPs
##############################

resource "aws_eip" "auto" {
  count  = length(var.eip_allocation_ids) == 0 ? var.eip_count : 0
  domain = "vpc"
  tags   = merge(local.merged_tags, { Name = "${var.name}-nat-${count.index}" })
}

resource "aws_eip_association" "floating" {
  for_each = { for idx, id in local.effective_eip_ids : idx => id }

  allocation_id        = each.value
  network_interface_id = aws_network_interface.blue.id
  private_ip_address   = local.floating_ips[tonumber(each.key)]
  allow_reassociation  = true
}