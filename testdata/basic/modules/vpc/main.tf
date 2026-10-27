variable "name" {
  type = string
}

variable "cidr" {
  type = string
}

variable "azs" {
  type = list(string)
}

resource "aws_vpc" "this" {
  cidr_block = var.cidr
  tags = {
    Name = var.name
  }
}

# 入れ子のモジュール呼び出し（同じモジュールを2回）
module "public_subnets" {
  source = "../subnet"

  vpc_id = aws_vpc.this.id
  azs    = var.azs
  cidrs  = [for i, az in var.azs : cidrsubnet(var.cidr, 8, i)]
}

module "private_subnets" {
  source = "../subnet"

  vpc_id = aws_vpc.this.id
  azs    = var.azs
  cidrs  = [for i, az in var.azs : cidrsubnet(var.cidr, 8, i + 100)]
}

output "vpc_id" {
  value = aws_vpc.this.id
}

output "cidr" {
  value = aws_vpc.this.cidr_block
}

output "public_subnet_ids" {
  value = module.public_subnets.ids
}

output "private_subnet_ids" {
  value = module.private_subnets.ids
}
