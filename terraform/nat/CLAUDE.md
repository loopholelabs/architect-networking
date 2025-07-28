# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Architecture Overview

This is a Terraform module for deploying Architect NAT, a high-availability NAT solution that provides fast failover. The module creates infrastructure in AWS with two instances (blue/red) that can fail over in sub-seconds by moving floating IPs between ENIs and updating route tables.

### Key Architectural Decisions

1. **Single ENI per Instance**: Each instance has one ENI with multiple IPs:
   - Management IP (primary) - Used for instance management and default routing
   - Floating IPs (secondary) - Used for NAT traffic, can be moved during failover

2. **Dedicated Subnet**: NAT instances run in an isolated "architect subnet" separate from application workloads for security and network isolation.

3. **Management + NAT Separation**: Management IPs have their own EIPs for reliable instance access, while floating IPs have separate EIPs for NAT traffic.

4. **No External Dependencies**: The module directly creates ASGs and launch templates instead of using external modules to maintain full control over the configuration.

## Development Commands

```bash
# Initialize Terraform (using OpenTofu)
tofu init

# Validate configuration
tofu validate

# Plan changes
tofu plan

# Apply changes
tofu apply

# Format code
tofu fmt -recursive

# Clean up
rm -rf .terraform .terraform.lock.hcl
```

## Code Structure

The module is split across multiple files for clarity:

- `networking.tf` - VPC subnet, route tables, and routing configuration
- `security.tf` - Security groups restricting access to VPC CIDR only
- `eni.tf` - Network interfaces and EIP management
- `iam.tf` - IAM roles and policies for failover operations
- `compute.tf` - Launch templates and Auto Scaling Groups
- `locals.tf` - Computed values and user data template references
- `userdata.tftpl` - EC2 user data script for instance initialization
- `ec2_instance_connect.tf` - Optional EC2 Instance Connect endpoint

## Critical Implementation Details

### IP Address Allocation

With a /24 subnet (recommended):
- `.10` - Blue instance management IP (primary, with management EIP)
- `.11` - Red instance management IP (primary, with management EIP)
- `.20` onwards - Floating IPs for NAT (with NAT EIPs)

### ENI Configuration

- Uses `private_ip_list_enabled = true` and `private_ip_list` to ensure IP ordering
- First IP in the list is guaranteed to be primary (management IP)
- This prevents AWS from randomly selecting which IP is primary

### Routing Configuration

The userdata script configures:
- Default route uses management IP as source (has internet via management EIP)
- NAT traffic uses floating IPs (have internet via NAT EIPs)
- This separation ensures instances always have internet access

### Failover Permissions

The IAM policy must include:

- `ec2:ReplaceRoute` - Update route tables
- `ec2:AssignPrivateIpAddresses` / `ec2:UnassignPrivateIpAddresses` - Move floating IP
- `ec2:AssociateAddress` / `ec2:DisassociateAddress` - Update EIP associations
- `ec2:ModifyNetworkInterfaceAttribute` - Modify ENI attributes

### Security Considerations

- ENIs must have `source_dest_check = false` for NAT functionality
- Security group only allows ingress from VPC CIDR blocks
- IPv6 is not currently supported by Architect NAT

## Module Constraints

1. **Single AZ**: This module deploys to a single availability zone
2. **Max 8 EIPs**: AWS limit for secondary IPs per ENI
3. **Required AMI**: User must provide an AMI with Architect NAT pre-installed
4. **License Key**: Valid Architect license required in `var.license_key`
5. **Subnet Size**: Recommend /24 or larger to accommodate all IPs

## Testing Considerations

When testing failover:

1. Terminate the blue instance
2. Verify floating IPs move to red ENI
3. Check route tables are updated
4. Confirm EIP associations are maintained
5. Test traffic flow through the new active instance
6. Verify management access remains available

## Common Issues

1. **Module version conflicts**: If switching from terraform-aws-modules/autoscaling, run `tofu init -upgrade`
2. **EIP limits**: Ensure account has sufficient EIP quota (management + NAT EIPs)
3. **Subnet conflicts**: The architect_subnet_cidr must not overlap with existing subnets
4. **IAM permissions**: Instances need specific EC2 permissions for failover to work
5. **Primary IP ordering**: Always use `private_ip_list_enabled` and `private_ip_list` to ensure correct IP ordering