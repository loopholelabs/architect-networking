##############################
# VARIABLES
##############################

variable "name" {
  description = "Name prefix applied to every Architect NAT resource"
  type        = string
  default     = "architect-nat"
}

variable "vpc_id" {
  description = "ID of the existing VPC"
  type        = string
}

variable "architect_subnet_cidr" {
  description = "CIDR block for the Architect subnet"
  type        = string
}

variable "availability_zone" {
  description = "AZ for the Architect subnet / ENIs (e.g. us‑west‑2a)"
  type        = string
}

variable "internet_gateway_id" {
  description = "ID of the IGW already attached to the VPC"
  type        = string
}

variable "route_table_ids" {
  description = "List of route‑table IDs whose 0.0.0.0/0 route will point at eni‑blue"
  type        = list(string)
}

# ───── EIP options ─────
variable "eip_allocation_ids" {
  description = "OPTIONAL list of pre‑allocated EIP allocation IDs (max 8). Leave empty to auto‑allocate."
  type        = list(string)
  default     = []
  validation {
    condition     = length(var.eip_allocation_ids) <= 8
    error_message = "Maximum 8 EIPs supported."
  }
}

variable "eip_count" {
  description = "How many EIPs to auto‑allocate when eip_allocation_ids is empty (1‑8)."
  type        = number
  default     = 1
  validation {
    condition     = var.eip_count >= 1 && var.eip_count <= 8
    error_message = "eip_count must be between 1 and 8."
  }
}

# ───── Instance knobs ─────
variable "ami_id" {
  description = "AMI ID to boot the Architect NAT instances"
  type        = string
}

variable "instance_type" {
  description = "EC2 instance type"
  type        = string
  default     = "t4g.micro"
}

variable "root_volume_size" {
  description = "Root EBS volume size in GiB"
  type        = number
  default     = 8
}

variable "license_key" {
  description = "Architect NAT licence key"
  type        = string
}

variable "nat_version" {
  description = "Version tag pulled by the user‑data script"
  type        = string
}

# Optional operational knobs retained
variable "enable_cloudwatch_agent" {
  description = "Install and configure the CloudWatch agent"
  type        = bool
  default     = false
}

variable "enable_ssm" {
  description = "Attach AmazonSSMManagedInstanceCore policy"
  type        = bool
  default     = false
}

variable "enable_ec2_instance_connect" {
  description = "Create EC2 Instance Connect endpoint in the architect subnet"
  type        = bool
  default     = false
}

variable "ssh_key_name" {
  description = "Name of an existing EC2 key pair for SSH (optional)"
  type        = string
  default     = ""
}

variable "extra_security_group_ids" {
  description = "Additional security group IDs attached to the ENIs & instances"
  type        = list(string)
  default     = []
}

variable "tags" {
  description = "Extra tags applied to every resource"
  type        = map(string)
  default     = {}
}
