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