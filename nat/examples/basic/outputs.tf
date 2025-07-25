output "nat_instance_ips" {
  description = "The private IPs for SSH/management access to NAT instances"
  value       = module.architect_nat.instance_private_ips
}

output "floating_ip" {
  description = "The floating private IP that moves during failover"
  value       = module.architect_nat.floating_private_ip
}

output "nat_public_ips" {
  description = "The public IPs (EIPs) for NAT traffic"
  value       = module.architect_nat.nat_public_ips
}

output "eip_allocation_ids" {
  description = "The EIP allocation IDs"
  value       = module.architect_nat.eip_allocation_ids
}

output "eni_ids" {
  description = "The ENI IDs for debugging"
  value       = module.architect_nat.eni_ids
}