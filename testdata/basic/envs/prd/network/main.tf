variable "vpc_cidrs" {
  type = map(string)
}

# 同じモジュールの複数回呼び出し（for_each 付き + 別名）
module "vpc" {
  for_each = var.vpc_cidrs
  source   = "../../../modules/vpc"

  name = each.key
  cidr = each.value
  azs  = ["ap-northeast-1a", "ap-northeast-1c"]
}

module "vpc_shared" {
  source = "../../../modules/vpc"

  name = "shared"
  cidr = "10.100.0.0/16"
  azs  = ["ap-northeast-1a"]
}

resource "aws_vpc_peering_connection" "main_shared" {
  vpc_id      = module.vpc["main"].vpc_id
  peer_vpc_id = module.vpc_shared.vpc_id
}

resource "aws_instance" "nat" {
  ami       = "ami-123456"
  subnet_id = module.vpc["main"].public_subnet_ids[0]
}

output "vpc_ids" {
  value = { for k, v in module.vpc : k => v.vpc_id }
}
