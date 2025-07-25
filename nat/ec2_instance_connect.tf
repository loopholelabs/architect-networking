##############################
# EC2 INSTANCE CONNECT ENDPOINT
##############################

# Security group for EC2 Instance Connect endpoint
resource "aws_security_group" "ec2_instance_connect" {
  count = var.enable_ec2_instance_connect ? 1 : 0

  name        = "${var.name}-ec2-instance-connect"
  description = "Security group for EC2 Instance Connect endpoint"
  vpc_id      = var.vpc_id

  # Allow outbound SSH to instances in the subnet
  egress {
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = [var.architect_subnet_cidr]
  }

  tags = merge(local.merged_tags, { Name = "${var.name}-ec2-instance-connect" })
}

# EC2 Instance Connect endpoint
resource "aws_ec2_instance_connect_endpoint" "architect" {
  count = var.enable_ec2_instance_connect ? 1 : 0

  subnet_id          = aws_subnet.architect.id
  security_group_ids = [aws_security_group.ec2_instance_connect[0].id]

  tags = merge(local.merged_tags, { Name = "${var.name}-ec2-instance-connect" })
}