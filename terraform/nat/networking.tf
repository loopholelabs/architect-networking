##############################
# NETWORK BUILD-OUT
##############################

resource "aws_subnet" "architect" {
  vpc_id                  = var.vpc_id
  cidr_block              = var.architect_subnet_cidr
  availability_zone       = var.availability_zone
  map_public_ip_on_launch = false
  tags                    = merge(local.merged_tags, { Name = "${var.name}-subnet" })
}

resource "aws_route_table" "architect" {
  vpc_id = var.vpc_id
  tags   = merge(local.merged_tags, { Name = "${var.name}-rt" })
}

resource "aws_route" "architect_default" {
  route_table_id         = aws_route_table.architect.id
  destination_cidr_block = "0.0.0.0/0"
  gateway_id             = var.internet_gateway_id
}

resource "aws_route_table_association" "architect" {
  subnet_id      = aws_subnet.architect.id
  route_table_id = aws_route_table.architect.id
}

##############################
# ROUTE-TABLE REWRITE
##############################

resource "aws_route" "replace_default" {
  count = length(var.route_table_ids != null ? var.route_table_ids : [])

  route_table_id         = var.route_table_ids[count.index]
  destination_cidr_block = "0.0.0.0/0"
  network_interface_id   = aws_network_interface.blue.id
}