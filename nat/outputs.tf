##############################
# OUTPUTS
##############################

output "architect_subnet_id" {
  value       = aws_subnet.architect.id
  description = "Architect subnet ID"
}

output "eni_ids" {
  value       = {
    blue = aws_network_interface.eni_blue.id,
    red  = aws_network_interface.eni_red.id
  }
  description = "Stable ENI IDs (blue = primary, red = standby)"
}

output "floating_private_ip" {
  value       = local.floating_ip
  description = "Floating private IP that moves between ENIs"
}

output "eip_allocation_ids" {
  value       = local.effective_eip_ids
  description = "Elastic IP allocation IDs bound to the floating private IP"
}

output "autoscaling_group_names" {
  value       = {
    blue = module.architect_nat_nodes["blue"].autoscaling_group_name,
    red  = module.architect_nat_nodes["red"].autoscaling_group_name
  }
  description = "Auto Scaling Group names maintaining one Architect NAT instance per ENI"
}

output "updated_route_table_ids" {
  value       = var.route_table_ids
  description = "Route tables whose default route now points to eni‑blue"
}
