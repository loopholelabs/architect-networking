# Architect NAT Terraform Module

This Terraform module deploys a highly-available NAT (Network Address Translation) solution using Architect's NAT
implementation. It provides a cost-effective alternative to AWS NAT Gateway with enhanced control and failover
capabilities.

## Architecture Overview

The module implements a dual-instance architecture with fast failover:

- **Two EC2 instances** (blue & red) running Architect NAT software
- **Two dedicated ENIs** (Elastic Network Interfaces) with deterministic private IPs
- **Floating private IP** that can move between ENIs during failover
- **Elastic IPs** bound to the floating private IP for external connectivity
- **Dedicated subnet** for NAT instances, isolated from other VPC resources

### Key Design Decisions

1. **ENI-based Failover**: Instead of relying on slow ENI detachment/reattachment, the module updates route tables to
   point to the healthy ENI, achieving sub-second failover.

2. **Dedicated Subnet**: NAT instances run in their own subnet for better isolation and security.

3. **Auto Scaling Groups**: Each instance (blue/red) has its own ASG to ensure high availability.

## Usage

```hcl
module "architect_nat" {
  source = "./nat"

  name                  = "my-nat"
  vpc_id                = aws_vpc.main.id
  architect_subnet_cidr = "10.0.255.0/24"
  availability_zone     = "us-west-2a"
  internet_gateway_id   = aws_internet_gateway.main.id
  route_table_ids = [aws_route_table.private.id]

  ami_id = "ami-xxxxxxxxx"  # Your Architect NAT AMI
  license_key = var.architect_license_key
  nat_version = "1.0.0"
}
```

## Prerequisites

- Existing VPC with Internet Gateway
- AMI with Architect NAT software
- Valid Architect license key
- Route table IDs for private subnets that need NAT access

## Module Features

### High Availability

- Automatic instance replacement via Auto Scaling Groups
- Fast failover using floating IP and route table updates
- Health checks to detect and replace failed instances

### Security

- Dedicated security group restricting ingress to VPC CIDR only
- Source/destination check disabled for NAT functionality
- IAM roles with minimal required permissions

### Operational

- Optional CloudWatch agent integration
- SSM Session Manager support (optional)
- SSH key support for debugging (optional)
- Configurable instance types and EBS volumes

## Inputs

| Name                    | Description                             | Type           | Default           | Required |
|-------------------------|-----------------------------------------|----------------|-------------------|----------|
| `name`                  | Name prefix for all resources           | `string`       | `"architect-nat"` | no       |
| `vpc_id`                | VPC ID to deploy NAT into               | `string`       | -                 | yes      |
| `architect_subnet_cidr` | CIDR block for the dedicated NAT subnet | `string`       | -                 | yes      |
| `availability_zone`     | AZ for NAT deployment                   | `string`       | -                 | yes      |
| `internet_gateway_id`   | IGW ID for outbound connectivity        | `string`       | -                 | yes      |
| `route_table_ids`       | List of route tables to update          | `list(string)` | -                 | yes      |
| `ami_id`                | AMI ID with Architect NAT               | `string`       | -                 | yes      |
| `license_key`           | Architect NAT license key               | `string`       | -                 | yes      |
| `nat_version`           | Version of Architect NAT to run         | `string`       | -                 | yes      |
| `instance_type`         | EC2 instance type                       | `string`       | `"t4g.micro"`     | no       |
| `eip_allocation_ids`    | Pre-allocated EIP IDs (max 8)           | `list(string)` | `[]`              | no       |
| `eip_count`             | Number of EIPs to auto-allocate         | `number`       | `1`               | no       |

## Outputs

| Name                      | Description                        |
|---------------------------|------------------------------------|
| `architect_subnet_id`     | ID of the created NAT subnet       |
| `eni_ids`                 | Map of ENI IDs (blue/red)          |
| `floating_private_ip`     | The floating private IP address    |
| `eip_allocation_ids`      | List of EIP allocation IDs         |
| `autoscaling_group_names` | Map of ASG names (blue/red)        |
| `updated_route_table_ids` | Route tables configured to use NAT |

## How It Works

1. **Initial State**: The floating IP (x.x.x.12) starts on the blue ENI (x.x.x.10)
2. **Traffic Flow**: Private subnets route 0.0.0.0/0 traffic to the blue ENI
3. **Failure Detection**: Health checks monitor instance health
4. **Failover Process**:
    - Floating IP moves from blue ENI to red ENI (x.x.x.11)
    - Route tables are updated to point to the red ENI
    - EIP associations are updated to maintain external connectivity
5. **Recovery**: When blue instance is replaced, it's ready for the next failover

## Maintenance

- Monitor CloudWatch metrics (if enabled)
- Keep AMI updated with latest Architect NAT version
- Review instance sizing based on bandwidth requirements
- Ensure license key remains valid

## Troubleshooting

1. **NAT not working**: Check security groups, route tables, and source/dest check
2. **Failover issues**: Verify IAM permissions for route/EIP management
3. **Performance**: Consider larger instance types for higher throughput
4. **Connectivity**: Ensure Internet Gateway is properly attached to VPC

## Examples

- [Basic Example](examples/basic) - Minimal configuration with single EIP
- [Complete Example](examples/complete) - Full feature demonstration with monitoring

## Requirements

| Name | Version |
|------|---------|
| terraform | >= 1.3 |
| aws | >= 4.0 |

## Providers

| Name | Version |
|------|---------|
| aws | >= 4.0 |

## Resources

| Name | Type |
|------|------|
| [aws_autoscaling_group.blue](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/autoscaling_group) | resource |
| [aws_autoscaling_group.red](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/autoscaling_group) | resource |
| [aws_eip.auto](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/eip) | resource |
| [aws_eip_association.floating](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/eip_association) | resource |
| [aws_iam_instance_profile.architect_nat](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_instance_profile) | resource |
| [aws_iam_role.architect_nat](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role) | resource |
| [aws_iam_role_policy.architect_nat](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role_policy) | resource |
| [aws_iam_role_policy_attachment.ssm](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_role_policy_attachment) | resource |
| [aws_launch_template.blue](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/launch_template) | resource |
| [aws_launch_template.red](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/launch_template) | resource |
| [aws_network_interface.eni_blue](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/network_interface) | resource |
| [aws_network_interface.eni_red](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/network_interface) | resource |
| [aws_route.architect_default](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/route) | resource |
| [aws_route.replace_default](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/route) | resource |
| [aws_route_table.architect](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/route_table) | resource |
| [aws_route_table_association.architect](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/route_table_association) | resource |
| [aws_security_group.base](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/security_group) | resource |
| [aws_security_group_rule.egress_all](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/security_group_rule) | resource |
| [aws_security_group_rule.ingress_from_vpc](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/security_group_rule) | resource |
| [aws_subnet.architect](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/subnet) | resource |
| [aws_iam_policy_document.assume_ec2](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/iam_policy_document) | data source |
| [aws_vpc.main](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/vpc) | data source |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| ami\_id | AMI ID to boot the Architect NAT instances | `string` | n/a | yes |
| architect\_subnet\_cidr | CIDR block for the Architect subnet | `string` | n/a | yes |
| availability\_zone | AZ for the Architect subnet / ENIs (e.g. us‑west‑2a) | `string` | n/a | yes |
| eip\_allocation\_ids | OPTIONAL list of pre‑allocated EIP allocation IDs (max 8). Leave empty to auto‑allocate. | `list(string)` | `[]` | no |
| eip\_count | How many EIPs to auto‑allocate when eip\_allocation\_ids is empty (1‑8). | `number` | `1` | no |
| enable\_cloudwatch\_agent | Install and configure the CloudWatch agent | `bool` | `false` | no |
| enable\_ssm | Attach AmazonSSMManagedInstanceCore policy | `bool` | `false` | no |
| extra\_security\_group\_ids | Additional security group IDs attached to the ENIs & instances | `list(string)` | `[]` | no |
| instance\_type | EC2 instance type | `string` | `"t4g.micro"` | no |
| internet\_gateway\_id | ID of the IGW already attached to the VPC | `string` | n/a | yes |
| license\_key | Architect NAT licence key | `string` | n/a | yes |
| name | Name prefix applied to every Architect NAT resource | `string` | `"architect-nat"` | no |
| nat\_version | Version tag pulled by the user‑data script | `string` | n/a | yes |
| root\_volume\_size | Root EBS volume size in GiB | `number` | `8` | no |
| route\_table\_ids | List of route‑table IDs whose 0.0.0.0/0 route will point at eni‑blue | `list(string)` | n/a | yes |
| ssh\_key\_name | Name of an existing EC2 key pair for SSH (optional) | `string` | `""` | no |
| tags | Extra tags applied to every resource | `map(string)` | `{}` | no |
| vpc\_id | ID of the existing VPC | `string` | n/a | yes |

## Outputs

| Name | Description |
|------|-------------|
| architect\_subnet\_id | Architect subnet ID |
| autoscaling\_group\_names | Auto Scaling Group names maintaining one Architect NAT instance per ENI |
| eip\_allocation\_ids | Elastic IP allocation IDs bound to the floating private IP |
| eni\_ids | Stable ENI IDs (blue = primary, red = standby) |
| floating\_private\_ip | Floating private IP that moves between ENIs |
| updated\_route\_table\_ids | Route tables whose default route now points to eni‑blue |

## License

This Terraform module is available under the Apache License 2.0. See the parent repository for full license details.