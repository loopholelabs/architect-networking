# Complete Architect NAT Example

This example demonstrates all available features of the Architect NAT module.

## Features Demonstrated

- VPC with multiple availability zones
- Multiple EIPs (3) for enhanced reliability
- CloudWatch agent integration with log collection
- SSM Session Manager access
- SSH key generation and management
- Additional security groups
- CloudWatch dashboard for monitoring
- Custom instance sizing (t3.medium)
- Larger root volume (20 GB)
- Complete tagging strategy

## Usage

```bash
export TF_VAR_architect_license_key="your-license-key-here"

terraform init
terraform plan
terraform apply
```

## Post-Deployment

### SSH Access

```bash
# Get the instance IPs from AWS Console or CLI
ssh -i architect-nat-key.pem ec2-user@<instance-ip>
```

### SSM Session Manager

```bash
aws ssm start-session --target <instance-id>
```

### Monitoring

- CloudWatch Dashboard: Check the output for the dashboard URL
- CloudWatch Logs: View logs in the created log group

## Testing Failover

1. Connect to a private instance
2. Start a continuous ping: `ping -i 0.2 google.com`
3. Terminate the blue instance
4. Observe the brief interruption and recovery

## Cleanup

```bash
terraform destroy
```

## Requirements

- AWS credentials configured
- Architect NAT AMI available in your account
- Valid Architect license key
- AWS CLI for SSM access (optional)

## Outputs

- `nat_instance_ips` - The ENI IDs for blue and red instances
- `floating_ip` - The floating private IP address
- `nat_public_ips` - The allocated public IP addresses
- `architect_subnet_id` - The dedicated NAT subnet
- `autoscaling_groups` - ASG names for both instances
- `ssh_private_key_path` - Path to generated SSH key
- `cloudwatch_log_group` - Log group for monitoring
- `dashboard_url` - Direct link to CloudWatch dashboard