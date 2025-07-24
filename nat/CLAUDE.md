# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Architecture Overview

This is a Terraform module for deploying Architect NAT, a high-availability NAT solution that provides fast failover through a dual-ENI architecture. The module creates infrastructure in AWS with two instances (blue/red) that can fail over in sub-seconds by moving a floating IP between ENIs and updating route tables.

### Key Architectural Decisions

1. **Dual-ENI Pattern**: Unlike traditional HA setups, this uses two persistent ENIs with a floating private IP that moves between them. This avoids slow ENI detachment/reattachment during failover.

2. **Dedicated Subnet**: NAT instances run in an isolated "architect subnet" separate from application workloads for security and network isolation.

3. **No External Dependencies**: The module directly creates ASGs and launch templates instead of using external modules to maintain full control over the configuration.

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

## Critical Implementation Details

### IP Address Allocation
- Blue ENI: `x.x.x.10` (primary) + `x.x.x.12` (floating)
- Red ENI: `x.x.x.11` (primary)
- The floating IP (`x.x.x.12`) starts on blue and moves during failover

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
2. **Max 8 EIPs**: AWS limit for EIP associations per ENI
3. **Required AMI**: User must provide an AMI with Architect NAT pre-installed
4. **License Key**: Valid Architect license required in `var.license_key`

## Testing Considerations

When testing failover:
1. Terminate the blue instance
2. Verify floating IP moves to red ENI
3. Check route tables are updated
4. Confirm EIP associations are maintained
5. Test traffic flow through the new active instance

## Common Issues

1. **Module version conflicts**: If switching from terraform-aws-modules/autoscaling, run `tofu init -upgrade`
2. **EIP limits**: Ensure account has sufficient EIP quota
3. **Subnet conflicts**: The architect_subnet_cidr must not overlap with existing subnets
4. **IAM permissions**: Instances need specific EC2 permissions for failover to work