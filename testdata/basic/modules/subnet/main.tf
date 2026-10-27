variable "vpc_id" {
  type = string
}

variable "azs" {
  type = list(string)
}

variable "cidrs" {
  type = list(string)
}

locals {
  subnets = { for i, c in var.cidrs : var.azs[i] => c }
}

resource "aws_subnet" "this" {
  for_each = local.subnets

  vpc_id            = var.vpc_id
  cidr_block        = each.value
  availability_zone = each.key
}

output "ids" {
  value = [for s in aws_subnet.this : s.id]
}
