data "aws_availability_zones" "available" {
  state = "available"
}

module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "~> 6.0"

  name = "architect-nat-basic"
  cidr = "10.0.0.0/16"

  azs             = [data.aws_availability_zones.available.names[0]]
  private_subnets = ["10.0.1.0/24"]
  public_subnets  = ["10.0.101.0/24"]

  enable_nat_gateway = false # We're using Architect NAT instead
  enable_vpn_gateway = false
}

# Example AMI lookup (you would replace this with your actual AMI)
data "aws_ami" "architect_nat" {
  most_recent = true
  owners      = ["self"]

  filter {
    name   = "name"
    values = ["architect-nat-*"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }
}