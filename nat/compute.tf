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
    network_interface_id  = aws_network_interface.blue.id
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

  user_data = base64encode(local.userdata_blue)

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
    network_interface_id  = aws_network_interface.red.id
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

  user_data = base64encode(local.userdata_red)

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
  name = "${var.name}-asg-blue"
  # vpc_zone_identifier is omitted when using pre-existing ENIs
  availability_zones        = [var.availability_zone]
  desired_capacity          = 1
  min_size                  = 1
  max_size                  = 1
  health_check_type         = "EC2"
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
  name = "${var.name}-asg-red"
  # vpc_zone_identifier is omitted when using pre-existing ENIs
  availability_zones        = [var.availability_zone]
  desired_capacity          = 1
  min_size                  = 1
  max_size                  = 1
  health_check_type         = "EC2"
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