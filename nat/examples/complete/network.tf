data "aws_availability_zones" "available" {
  state = "available"
}

module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "~> 6.0"

  name = "architect-nat-complete"
  cidr = "10.0.0.0/16"

  azs             = slice(data.aws_availability_zones.available.names, 0, 2)
  private_subnets = ["10.0.1.0/24", "10.0.2.0/24"]
  public_subnets  = ["10.0.101.0/24", "10.0.102.0/24"]

  enable_nat_gateway   = false # We're using Architect NAT instead
  enable_vpn_gateway   = false
  enable_dns_hostnames = true
  enable_dns_support   = true
}

# Additional security group for custom rules
resource "aws_security_group" "additional" {
  name        = "architect-nat-additional"
  description = "Additional security group for Architect NAT"
  vpc_id      = module.vpc.vpc_id

  # Example: Allow HTTPS from specific CIDR
  ingress {
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["10.0.0.0/16"]
    description = "Allow HTTPS from VPC"
  }

  tags = {
    Name = "architect-nat-additional"
  }
}