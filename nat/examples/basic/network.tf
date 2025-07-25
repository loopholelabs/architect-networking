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

# Create a route table for private subnet
resource "aws_route_table" "private" {
  vpc_id = module.vpc.vpc_id

  tags = {
    Name = "architect-nat-basic-private"
  }
}

# Associate the route table with the private subnet
resource "aws_route_table_association" "private" {
  subnet_id      = module.vpc.private_subnets[0]
  route_table_id = aws_route_table.private.id
}