# Basic Architect NAT Example

This example shows the minimal configuration required to deploy Architect NAT.

## Features Demonstrated

- Basic VPC setup with public and private subnets
- Architect NAT deployment with minimal configuration
- Single EIP allocation
- Route table configuration for private subnet

## Usage

```bash
export TF_VAR_architect_license_key="your-license-key-here"

terraform init
terraform plan
terraform apply
```

## Requirements

- AWS credentials configured
- Architect NAT AMI available in your account
- Valid Architect license key

## Outputs

- `nat_instance_ips` - The ENI IDs for blue and red instances
- `floating_ip` - The floating private IP address
- `nat_public_ips` - The allocated public IP addresses