##############################################################
# Terraform Module: architect‑nat
# -----------------------------------------------------------
# Highly‑available **Architect NAT** deployment for a *single* AZ.
# It launches **two EC2 instances** (blue & red) wired to two
# dedicated ENIs inside an *Architect subnet*. A secondary
# *floating private IP* plus up to eight Elastic IPs provide
# sub‑second fail‑over without Gateway Load Balancer.
#
# ───────────────────────── MODULE HIGHLIGHTS ─────────────────
# • Builds an **architect subnet** in the caller‑chosen AZ.
# • Creates **eni‑blue** & **eni‑red** with deterministic primary
#   addresses (x.x.x.10 & x.x.x.11) and reserves a **floating
#   private IP** (x.x.x.12) that initially lives on eni‑blue.
# • Allocates or attaches 1‑8 **public IPs (EIPs)** to that
#   floating private IP. `allow_reassociation=true` lets the
#   same association survive a fail‑over.
# • Two Auto Scaling Groups (1 instance each) ensure one EC2 per
#   ENI at all times. Instances carry an IAM role that allows
#   them to run the fail‑over logic (ReplaceRoute, move private
#   IP, re‑associate EIPs).
# • Adds additional configuration knobs (CloudWatch agent, SSM, SSH
#   key, extra SGs, sizing, etc.).
##############################################################

##############################
# LOCALS
##############################

locals {
  merged_tags = merge({
    "Project"   = var.name,
    "Terraform" = "true"
  }, var.tags)

  primary_blue_ip = cidrhost(var.architect_subnet_cidr, 10)
  primary_red_ip  = cidrhost(var.architect_subnet_cidr, 11)
  floating_ip     = cidrhost(var.architect_subnet_cidr, 12)

  effective_eip_ids = length(var.eip_allocation_ids) == 0 ? [for e in aws_eip.auto : e.id] : var.eip_allocation_ids
}

##############################
# NETWORK BUILD‑OUT
##############################

resource "aws_subnet" "architect" {
  vpc_id                  = var.vpc_id
  cidr_block              = var.architect_subnet_cidr
  availability_zone       = var.availability_zone
  map_public_ip_on_launch = false
  tags                    = merge(local.merged_tags, { Name = "${var.name}-subnet" })
}

resource "aws_route_table" "architect" {
  vpc_id = var.vpc_id
  tags   = merge(local.merged_tags, { Name = "${var.name}-rt" })
}

resource "aws_route" "architect_default" {
  route_table_id         = aws_route_table.architect.id
  destination_cidr_block = "0.0.0.0/0"
  gateway_id             = var.internet_gateway_id
}

resource "aws_route_table_association" "architect" {
  subnet_id      = aws_subnet.architect.id
  route_table_id = aws_route_table.architect.id
}

# 3) Base security group – allow ingress only from within VPC
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
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.base.id
  description       = "Allow all outbound traffic"
}

##############################
# ENIs & FLOATING PRIVATE IP
##############################

resource "aws_network_interface" "eni_blue" {
  subnet_id         = aws_subnet.architect.id
  private_ips       = [local.primary_blue_ip, local.floating_ip]
  security_groups   = concat([aws_security_group.base.id], var.extra_security_group_ids)
  source_dest_check = false
  tags              = merge(local.merged_tags, { Name = "${var.name}-eni-blue" })
}

resource "aws_network_interface" "eni_red" {
  subnet_id         = aws_subnet.architect.id
  private_ips       = [local.primary_red_ip]
  security_groups   = concat([aws_security_group.base.id], var.extra_security_group_ids)
  source_dest_check = false
  tags              = merge(local.merged_tags, { Name = "${var.name}-eni-red" })
}

##############################
# EIPs
##############################

resource "aws_eip" "auto" {
  count  = length(var.eip_allocation_ids) == 0 ? var.eip_count : 0
  domain = "vpc"
  tags   = merge(local.merged_tags, { Name = "${var.name}-eip-${count.index}" })
}

resource "aws_eip_association" "floating" {
  for_each = { for idx, id in local.effective_eip_ids : idx => id }

  allocation_id        = each.value
  network_interface_id = aws_network_interface.eni_blue.id
  private_ip_address   = local.floating_ip
  allow_reassociation  = true
}

##############################
# ROUTE‑TABLE REWRITE
##############################

resource "aws_route" "replace_default" {
  for_each = toset(var.route_table_ids)

  route_table_id         = each.value
  destination_cidr_block = "0.0.0.0/0"
  network_interface_id   = aws_network_interface.eni_blue.id
}

##############################
# IAM ROLE
##############################

data "aws_iam_policy_document" "assume_ec2" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["ec2.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "architect_nat" {
  name               = "${var.name}-role"
  assume_role_policy = data.aws_iam_policy_document.assume_ec2.json
}

resource "aws_iam_role_policy" "architect_nat" {
  name   = "${var.name}-policy"
  role   = aws_iam_role.architect_nat.id
  policy = jsonencode({
    Version = "2012-10-17",
    Statement = [
      {
        Effect   = "Allow",
        Action   = [
          "ec2:ReplaceRoute",
          "ec2:AssignPrivateIpAddresses",
          "ec2:UnassignPrivateIpAddresses",
          "ec2:AssociateAddress",
          "ec2:DisassociateAddress",
          "ec2:ModifyNetworkInterfaceAttribute",
          "ec2:Describe*"
        ],
        Resource = "*"
      }
    ]
  })
}

resource "aws_iam_role_policy_attachment" "ssm" {
  count      = var.enable_ssm ? 1 : 0
  role       = aws_iam_role.architect_nat.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
}

resource "aws_iam_instance_profile" "architect_nat" {
  name = "${var.name}-profile"
  role = aws_iam_role.architect_nat.name
}

##############################
# USER‑DATA SCRIPT
##############################

locals {
  userdata = templatefile("${path.module}/userdata.tftpl", {
    license_key              = var.license_key
    nat_version              = var.nat_version
    enable_cloudwatch_agent  = var.enable_cloudwatch_agent
    name                     = var.name
  })
}

##############################
# LAUNCH TEMPLATES & ASGs
##############################

# Blue Launch Template
resource "aws_launch_template" "blue" {
  name_prefix   = "${var.name}-lt-blue-"
  image_id      = var.ami_id
  instance_type = var.instance_type
  key_name      = var.ssh_key_name != "" ? var.ssh_key_name : null

  iam_instance_profile {
    name = aws_iam_instance_profile.architect_nat.name
  }

  network_interfaces {
    delete_on_termination = false
    device_index          = 0
    network_interface_id  = aws_network_interface.eni_blue.id
  }

  block_device_mappings {
    device_name = "/dev/xvda"
    ebs {
      volume_size           = var.root_volume_size
      volume_type           = "gp3"
      delete_on_termination = true
      encrypted             = true
    }
  }

  user_data = base64encode(local.userdata)

  tag_specifications {
    resource_type = "instance"
    tags          = merge(local.merged_tags, { Name = "${var.name}-node-blue" })
  }

  tag_specifications {
    resource_type = "volume"
    tags          = merge(local.merged_tags, { Name = "${var.name}-node-blue-root" })
  }

  tags = merge(local.merged_tags, { Name = "${var.name}-lt-blue" })
}

# Red Launch Template
resource "aws_launch_template" "red" {
  name_prefix   = "${var.name}-lt-red-"
  image_id      = var.ami_id
  instance_type = var.instance_type
  key_name      = var.ssh_key_name != "" ? var.ssh_key_name : null

  iam_instance_profile {
    name = aws_iam_instance_profile.architect_nat.name
  }

  network_interfaces {
    delete_on_termination = false
    device_index          = 0
    network_interface_id  = aws_network_interface.eni_red.id
  }

  block_device_mappings {
    device_name = "/dev/xvda"
    ebs {
      volume_size           = var.root_volume_size
      volume_type           = "gp3"
      delete_on_termination = true
      encrypted             = true
    }
  }

  user_data = base64encode(local.userdata)

  tag_specifications {
    resource_type = "instance"
    tags          = merge(local.merged_tags, { Name = "${var.name}-node-red" })
  }

  tag_specifications {
    resource_type = "volume"
    tags          = merge(local.merged_tags, { Name = "${var.name}-node-red-root" })
  }

  tags = merge(local.merged_tags, { Name = "${var.name}-lt-red" })
}

# Blue Auto Scaling Group
resource "aws_autoscaling_group" "blue" {
  name                = "${var.name}-asg-blue"
  vpc_zone_identifier = [aws_subnet.architect.id]
  desired_capacity    = 1
  min_size            = 1
  max_size            = 1
  health_check_type   = "EC2"
  health_check_grace_period = 300

  launch_template {
    id      = aws_launch_template.blue.id
    version = "$Latest"
  }

  tag {
    key                 = "Name"
    value               = "${var.name}-node-blue"
    propagate_at_launch = true
  }

  dynamic "tag" {
    for_each = local.merged_tags

    content {
      key                 = tag.key
      value               = tag.value
      propagate_at_launch = false
    }
  }
}

# Red Auto Scaling Group
resource "aws_autoscaling_group" "red" {
  name                = "${var.name}-asg-red"
  vpc_zone_identifier = [aws_subnet.architect.id]
  desired_capacity    = 1
  min_size            = 1
  max_size            = 1
  health_check_type   = "EC2"
  health_check_grace_period = 300

  launch_template {
    id      = aws_launch_template.red.id
    version = "$Latest"
  }

  tag {
    key                 = "Name"
    value               = "${var.name}-node-red"
    propagate_at_launch = true
  }

  dynamic "tag" {
    for_each = local.merged_tags

    content {
      key                 = tag.key
      value               = tag.value
      propagate_at_launch = false
    }
  }
}