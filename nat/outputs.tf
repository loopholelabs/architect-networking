##############################
# OUTPUTS
##############################

output "architect_subnet_id" {
  value       = aws_subnet.architect.id
  description = "Architect subnet ID"
}

output "eni_ids" {
  value = {
    blue = aws_network_interface.blue.id,
    red  = aws_network_interface.red.id
  }
  description = "Stable ENI IDs (blue = primary, red = standby)"
}

output "floating_private_ip" {
  value       = local.floating_ip
  description = "Floating private IP that moves between ENIs"
}

output "instance_private_ips" {
  value = {
    blue = local.primary_blue_ip
    red  = local.primary_red_ip
  }
  description = "Primary private IPs for SSH/management access to instances"
}

output "eip_allocation_ids" {
  value       = local.effective_eip_ids
  description = "Elastic IP allocation IDs bound to the floating private IP"
}

output "nat_public_ips" {
  value       = [for eip in aws_eip.auto : eip.public_ip]
  description = "Public IP addresses for NAT traffic"
}

output "autoscaling_group_names" {
  value = {
    blue = aws_autoscaling_group.blue.name,
    red  = aws_autoscaling_group.red.name
  }
  description = "Auto Scaling Group names maintaining one Architect NAT instance per ENI"
}

output "updated_route_table_ids" {
  value       = var.route_table_ids
  description = "Route tables whose default route now points to eni‑blue"
}

output "ec2_instance_connect_endpoint_id" {
  value       = var.enable_ec2_instance_connect ? aws_ec2_instance_connect_endpoint.architect[0].id : null
  description = "EC2 Instance Connect endpoint ID (if enabled)"
}
