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

  filter {
    name   = "architecture"
    values = ["x86_64"]
  }
}