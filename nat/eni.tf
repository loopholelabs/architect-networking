##############################
# ENIs & FLOATING PRIVATE IP
##############################

resource "aws_network_interface" "eni_blue" {
  subnet_id         = aws_subnet.architect.id
  private_ips       = [local.primary_blue_ip, local.floating_ip]
  security_groups   = concat([aws_security_group.base.id], var.extra_security_group_ids)
  source_dest_check = false
  tags              = merge(local.merged_tags, { Name = "${var.name}-eni-blue" })
}

resource "aws_network_interface" "eni_red" {
  subnet_id         = aws_subnet.architect.id
  private_ips       = [local.primary_red_ip]
  security_groups   = concat([aws_security_group.base.id], var.extra_security_group_ids)
  source_dest_check = false
  tags              = merge(local.merged_tags, { Name = "${var.name}-eni-red" })
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
  network_interface_id = aws_network_interface.eni_blue.id
  private_ip_address   = local.floating_ip
  allow_reassociation  = true
}