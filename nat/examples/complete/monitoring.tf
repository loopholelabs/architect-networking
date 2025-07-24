# CloudWatch Log Group for Architect NAT logs
resource "aws_cloudwatch_log_group" "architect_nat" {
  name              = "/architect-nat/${module.architect_nat.autoscaling_group_names.blue}"
  retention_in_days = 7

  tags = {
    Name = "architect-nat-logs"
  }
}

# CloudWatch Dashboard for monitoring
resource "aws_cloudwatch_dashboard" "architect_nat" {
  dashboard_name = "architect-nat-complete-example"

  dashboard_body = jsonencode({
    widgets = [
      {
        type   = "metric"
        width  = 12
        height = 6
        properties = {
          metrics = [
            ["AWS/EC2", "NetworkIn", { stat = "Average" }],
            [".", "NetworkOut", { stat = "Average" }]
          ]
          period = 300
          stat   = "Average"
          region = data.aws_region.current.id
          title  = "Network Traffic"
        }
      },
      {
        type   = "metric"
        width  = 12
        height = 6
        properties = {
          metrics = [
            ["AWS/EC2", "CPUUtilization", { stat = "Average" }]
          ]
          period = 300
          stat   = "Average"
          region = data.aws_region.current.id
          title  = "CPU Utilization"
        }
      }
    ]
  })
}

data "aws_region" "current" {}