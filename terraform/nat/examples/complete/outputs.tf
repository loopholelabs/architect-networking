output "nat_instance_ips" {
  description = "The ENI IPs for the NAT instances"
  value       = module.architect_nat.eni_ids
}

output "floating_ip" {
  description = "The floating private IP that moves during failover"
  value       = module.architect_nat.floating_private_ip
}

output "nat_public_ips" {
  description = "The public IPs (EIPs) for NAT"
  value       = module.architect_nat.eip_allocation_ids
}

output "architect_subnet_id" {
  description = "The subnet ID where NAT instances are deployed"
  value       = module.architect_nat.architect_subnet_id
}

output "autoscaling_groups" {
  description = "The Auto Scaling Group names"
  value       = module.architect_nat.autoscaling_group_names
}

output "ssh_private_key_path" {
  description = "Path to the SSH private key file"
  value       = local_file.private_key.filename
}

output "cloudwatch_log_group" {
  description = "CloudWatch Log Group name for NAT logs"
  value       = aws_cloudwatch_log_group.architect_nat.name
}

output "dashboard_url" {
  description = "URL to the CloudWatch dashboard"
  value       = "https://console.aws.amazon.com/cloudwatch/home?region=${data.aws_region.current.id}#dashboards:name=${aws_cloudwatch_dashboard.architect_nat.dashboard_name}"
}