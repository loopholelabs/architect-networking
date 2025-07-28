module "architect_nat" {
  source = "../.."

  vpc_id                = module.vpc.vpc_id
  architect_subnet_cidr = "10.0.255.0/24"
  availability_zone     = data.aws_availability_zones.available.names[0]
  internet_gateway_id   = module.vpc.igw_id
  route_table_ids       = module.vpc.private_route_table_ids

  # Required inputs
  ami_id      = data.aws_ami.architect_nat.id
  license_key = var.architect_license_key
  nat_version = var.nat_version

  # Instance configuration
  instance_type    = "c5n.9xlarge"
  root_volume_size = 20

  # Multiple EIPs for higher availability
  eip_count = 3

  # Enable operational features
  enable_cloudwatch_agent = true
  enable_ssm              = true

  # SSH access (for debugging)
  ssh_key_name = aws_key_pair.architect_nat.key_name

  # Additional security groups
  extra_security_group_ids = [aws_security_group.additional.id]

  # Tags
  tags = {
    Example     = "architect-nat-demo-complete"
    ManagedBy   = "terraform"
  }
}