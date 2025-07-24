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

# 3) Base security group – allow **all** ingress & egress; appliance enforces policy
resource "aws_security_group" "base" {
  name        = "${var.name}-sg-base"
  description = "Architect NAT base SG: allow all ingress and egress; traffic filtering is done by the appliance"
  vpc_id      = var.vpc_id

  ingress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = local.merged_tags
}

##############################
# ENIs & FLOATING PRIVATE IP
##############################

resource "aws_network_interface" "eni_blue" {
  subnet_id       = aws_subnet.architect.id
  private_ips     = [local.primary_blue_ip]
  security_groups = concat([aws_security_group.base.id], var.extra_security_group_ids)
  tags            = merge(local.merged_tags, { Name = "${var.name}-eni-blue" })
}

resource "aws_network_interface" "eni_red" {
  subnet_id       = aws_subnet.architect.id
  private_ips     = [local.primary_red_ip]
  security_groups = concat([aws_security_group.base.id], var.extra_security_group_ids)
  tags            = merge(local.merged_tags, { Name = "${var.name}-eni-red" })
}

resource "aws_network_interface_private_ips" "floating" {
  network_interface_id = aws_network_interface.eni_blue.id
  private_ips          = [local.floating_ip]
}

##############################
# EIPs
##############################

resource "aws_eip" "auto" {
  count = length(var.eip_allocation_ids) == 0 ? var.eip_count : 0
  vpc   = true
  tags  = merge(local.merged_tags, { Name = "${var.name}-eip-${count.index}" })
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
  userdata = <<-EOF
    #!/usr/bin/env bash
    set -euo pipefail

    yum -y update -q
    amazon-linux-extras enable docker && yum -y install docker jq
    systemctl enable --now docker

    docker run --detach --name architect-nat \
      --network host \
      --privileged \
      -e LICENSE_KEY="${var.license_key}" \
      ghcr.io/yourco/architect-nat:${var.nat_version}

    % if var.enable_cloudwatch_agent %
    yum -y install amazon-cloudwatch-agent
    cat >/opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json <<CFG
    {"logs":{"logs_collected":{"files":{"collect_list":[{"file_path":"/var/log/messages","log_group_name":"/architect-nat/${var.name}","log_stream_name":"{instance_id}"}]}}}}
CFG
    /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl -a fetch-config -m ec2 -c file:/opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json -s
    % endif
  EOF
}

##############################
# LAUNCH TEMPLATES & ASGs
##############################

module "architect_nat_nodes" {
  source  = "terraform-aws-modules/autoscaling/aws"
  version = "6.6.1"

  for_each = {
    blue = {
      eni_id   = aws_network_interface.eni_blue.id
      lt_name  = "${var.name}-lt-blue"
      asg_name = "${var.name}-asg-blue"
      tags     = merge(local.merged_tags, { Name = "${var.name}-node-blue" })
    }
    red = {
      eni_id   = aws_network_interface.eni_red.id
      lt_name  = "${var.name}-lt-red"
      asg_name = "${var.name}-asg-red"
      tags     = merge(local.merged_tags, { Name = "${var.name}-node-red" })
    }
  }

  create_launch_template = true
  launch_template_name   = each.value.lt_name
  launch_template_tags   = each.value.tags
  image_id               = var.ami_id
  instance_type          = var.instance_type
  key_name               = var.ssh_key_name != "" ? var.ssh_key_name : null
  iam_instance_profile_name = aws_iam_instance_profile.architect_nat.name

  security_groups        = concat([aws_security_group.base.id], var.extra_security_group_ids)
  user_data_base64       = base64encode(local.userdata)

  network_interfaces = [{
    delete_on_termination = false
    device_index          = 0
    network_interface_id  = each.value.eni_id
  }]

  block_device_mappings = [{
    device_name = "/dev/xvda"
    ebs = {
      volume_size           = var.root_volume_size
      volume_type           = "gp3"
      delete_on_termination = true
    }
  }]

  name               = each.value.asg_name
  vpc_zone_identifier = [aws_subnet.architect.id]
  desired_capacity   = 1
  min_size           = 1
  max_size           = 1
  tags_as_map        = each.value.tags
}