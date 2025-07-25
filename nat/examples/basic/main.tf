module "architect_nat" {
  source = "../../"

  name                  = "architect-nat-basic"
  vpc_id                = module.vpc.vpc_id
  architect_subnet_cidr = "10.0.255.0/28"
  availability_zone     = data.aws_availability_zones.available.names[0]
  internet_gateway_id   = module.vpc.igw_id
  route_table_ids       = module.vpc.private_route_table_ids

  # SSH access (for debugging)
  ssh_key_name = aws_key_pair.architect_nat.key_name

  # EC2 connect endpoint (for debugging)
  enable_ec2_instance_connect = true

  # Required inputs
  ami_id        = data.aws_ami.architect_nat.id
  license_key   = var.architect_license_key
  nat_version   = "latest"
  instance_type = "t3.micro" # x86_64 instance type
}