# Generate SSH key for debugging access
resource "tls_private_key" "architect_nat" {
  algorithm = "RSA"
  rsa_bits  = 4096
}

resource "aws_key_pair" "architect_nat" {
  key_name   = "architect-nat-complete"
  public_key = tls_private_key.architect_nat.public_key_openssh

  tags = {
    Name = "architect-nat-complete"
  }
}

# Save private key locally for SSH access
resource "local_file" "private_key" {
  content         = tls_private_key.architect_nat.private_key_pem
  filename        = "${path.module}/architect-nat-key.pem"
  file_permission = "0600"
}