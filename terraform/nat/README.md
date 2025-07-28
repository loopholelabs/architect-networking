# Architect NAT Terraform Module

This Terraform module deploys a highly-available NAT (Network Address Translation) solution using Architect's NAT implementation. It provides a cost-effective alternative to AWS NAT Gateway with enhanced control and failover capabilities.

## Architecture Overview

The module implements a dual-instance architecture with fast failover:

- **Two EC2 instances** (blue & red) running Architect NAT software
- **Single ENI per instance** with multiple IP addresses:
  - Management IP (primary) for instance access and default routing
  - Floating IPs (secondary) for NAT traffic that can move during failover
- **Multiple Elastic IPs**:
  - Management EIPs for reliable instance access
  - NAT EIPs (up to 8) for outbound NAT traffic
- **Dedicated subnet** for NAT instances, isolated from other VPC resources
- **Route table updates** for seamless failover

### Key Design Decisions

1. **Single ENI Architecture**: Each instance uses one ENI with both management and NAT IPs, simplifying the design while maintaining separation of concerns through routing.

2. **IP Ordering Control**: Uses `private_ip_list_enabled` and `private_ip_list` to ensure deterministic IP assignment (management IP as primary).

3. **Management/NAT Separation**: Management IPs have dedicated EIPs for instance access, while floating IPs have separate EIPs for NAT traffic.

4. **Userdata Routing**: Instances configure their default route to use the management IP, ensuring reliable internet access.

## Usage

```hcl
module "architect_nat" {
  source = "./nat"

  name                  = "my-nat"
  vpc_id                = aws_vpc.main.id
  architect_subnet_cidr = "10.0.255.0/24"  # Recommend /24 or larger
  availability_zone     = "us-west-2a"
  internet_gateway_id   = aws_internet_gateway.main.id
  route_table_ids       = [aws_route_table.private.id]

  ami_id      = "ami-xxxxxxxxx"  # Your Architect NAT AMI
  license_key = var.architect_license_key
  nat_version = "1.0.0"
  
  # Optional: Multiple NAT EIPs
  eip_count = 3  # Creates 3 NAT EIPs for load distribution
  
  # Optional: EC2 Instance Connect
  enable_ec2_instance_connect = true
}
```

## Prerequisites

- Existing VPC with Internet Gateway
- AMI with Architect NAT software pre-installed
- Valid Architect license key
- Route table IDs for private subnets that need NAT access
- Sufficient EIP quota (2 management + N NAT EIPs)

## Module Features

### High Availability

- Automatic instance replacement via Auto Scaling Groups
- Fast failover by moving floating IPs between instances
- Health checks to detect and replace failed instances
- Persistent management access during failover

### Multiple EIP Support

- Up to 8 NAT EIPs per deployment (AWS limit)
- Load distribution across multiple public IPs
- Option to bring your own EIP allocation IDs

### Security

- Dedicated security group restricting ingress to VPC CIDR only
- Source/destination check disabled for NAT functionality
- IAM roles with minimal required permissions
- Optional EC2 Instance Connect endpoint for secure access

### Operational

- Userdata script configures proper routing
- Optional CloudWatch agent integration
- SSM Session Manager support (optional)
- SSH key support for debugging (optional)
- Configurable instance types and EBS volumes

## IP Address Allocation

With a /24 subnet (recommended):
- `.10` - Blue instance management IP (with management EIP)
- `.11` - Red instance management IP (with management EIP)
- `.20` onwards - Floating IPs for NAT (with NAT EIPs)

## Inputs

| Name                       | Description                             | Type           | Default           | Required |
|----------------------------|-----------------------------------------|----------------|-------------------|----------|
| `name`                     | Name prefix for all resources           | `string`       | `"architect-nat"` | no       |
| `vpc_id`                   | VPC ID to deploy NAT into               | `string`       | -                 | yes      |
| `architect_subnet_cidr`    | CIDR block for the dedicated NAT subnet | `string`       | -                 | yes      |
| `availability_zone`        | AZ for NAT deployment                   | `string`       | -                 | yes      |
| `internet_gateway_id`      | IGW ID for outbound connectivity        | `string`       | -                 | yes      |
| `route_table_ids`          | List of route tables to update          | `list(string)` | -                 | yes      |
| `ami_id`                   | AMI ID with Architect NAT               | `string`       | -                 | yes      |
| `license_key`              | Architect NAT license key               | `string`       | -                 | yes      |
| `nat_version`              | Version of Architect NAT to run         | `string`       | -                 | yes      |
| `instance_type`            | EC2 instance type                       | `string`       | `"t4g.micro"`     | no       |
| `eip_allocation_ids`       | Pre-allocated EIP IDs (max 8)           | `list(string)` | `[]`              | no       |
| `eip_count`                | Number of EIPs to auto-allocate         | `number`       | `1`               | no       |
| `enable_ec2_instance_connect` | Create EC2 Instance Connect endpoint | `bool`         | `false`           | no       |

## Outputs

| Name                         | Description                              |
|------------------------------|------------------------------------------|
| `architect_subnet_id`        | ID of the created NAT subnet             |
| `eni_ids`                    | Map of ENI IDs (blue/red)                |
| `floating_private_ip`        | First floating private IP address        |
| `instance_private_ips`       | Management IPs for SSH/instance access   |
| `eip_allocation_ids`         | List of NAT EIP allocation IDs           |
| `nat_public_ips`             | Public IP addresses for NAT traffic      |
| `autoscaling_group_names`    | Map of ASG names (blue/red)              |
| `updated_route_table_ids`    | Route tables configured to use NAT       |
| `ec2_instance_connect_endpoint_id` | EC2 Instance Connect endpoint ID   |

## How It Works

1. **Initial State**: 
   - Blue instance has management IP (.10) and floating IPs (.20+)
   - Red instance has only management IP (.11)
   - Route tables point to blue ENI for NAT

2. **Traffic Flow**:
   - Instance default routes use management IPs (have internet via management EIPs)
   - NAT traffic uses floating IPs (have internet via NAT EIPs)
   - Private subnets route 0.0.0.0/0 to blue ENI

3. **Failover Process**:
   - Floating IPs move from blue ENI to red ENI
   - Route tables are updated to point to red ENI
   - NAT EIP associations follow the floating IPs
   - Management access remains available throughout

4. **Recovery**: When blue instance is replaced, it's ready for the next failover

## Routing Configuration

The userdata script automatically configures:
- Default route uses management IP as source
- This ensures instances always have internet access via management EIP
- NAT functionality uses the floating IPs with their associated EIPs

## Maintenance

- Monitor CloudWatch metrics (if enabled)
- Keep AMI updated with latest Architect NAT version
- Review instance sizing based on bandwidth requirements
- Ensure license key remains valid
- Check EIP quota before increasing `eip_count`

## Troubleshooting

1. **NAT not working**: 
   - Check security groups allow traffic from VPC CIDR
   - Verify route tables point to correct ENI
   - Ensure source/dest check is disabled

2. **EIP association errors**:
   - Verify the ENI has the IP address before associating EIP
   - Check AWS EIP quota limits

3. **Primary IP issues**:
   - Ensure using `private_ip_list_enabled = true`
   - First IP in `private_ip_list` becomes primary

4. **Routing issues**:
   - Check userdata script completed successfully
   - Verify management IP has route to internet

## Examples

- [Basic Example](examples/basic) - Minimal configuration with single NAT EIP
- [Complete Example](examples/complete) - Full features including multiple EIPs and monitoring

## Requirements

| Name | Version |
|------|---------|
| terraform | >= 1.3 |
| aws | >= 4.0 |

## License

This Terraform module is available under the Apache License 2.0. See the parent repository for full license details.