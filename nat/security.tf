##############################
# SECURITY GROUPS
##############################

# Base security group – allow ingress only from within VPC
resource "aws_security_group" "base" {
  name        = "${var.name}-sg-base"
  description = "Architect NAT base SG: allow ingress from VPC only"
  vpc_id      = var.vpc_id

  tags = local.merged_tags
}

# Get VPC data for CIDR blocks
data "aws_vpc" "main" {
  id = var.vpc_id
}

# Allow all ingress from within the VPC
resource "aws_security_group_rule" "ingress_from_vpc" {
  type              = "ingress"
  from_port         = 0
  to_port           = 0
  protocol          = "-1"
  cidr_blocks       = data.aws_vpc.main.cidr_block_associations[*].cidr_block
  security_group_id = aws_security_group.base.id
  description       = "Allow all traffic from within VPC"
}

# Allow all egress
# Note: IPv6 is not currently supported by Architect NAT
resource "aws_security_group_rule" "egress_all" {
  type              = "egress"
  from_port         = 0
  to_port           = 0
  protocol          = "-1"
  cidr_blocks = ["0.0.0.0/0"]
  security_group_id = aws_security_group.base.id
  description       = "Allow all outbound traffic"
}