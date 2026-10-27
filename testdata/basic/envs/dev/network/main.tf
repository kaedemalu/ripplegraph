locals {
  prefix = "dev"
  # for 式の中の参照（ループ変数 z は参照にならない）
  azs          = [for z in data.aws_availability_zones.available.names : z if z != var.excluded_az]
  egress_cidrs = [module.vpc.cidr]
}

data "aws_availability_zones" "available" {
  state = "available"
}

data "aws_ami" "nat" {
  most_recent = true
  owners      = ["amazon"]
}

module "vpc" {
  source = "../../../modules/vpc"

  name = "${local.prefix}-vpc"
  cidr = var.vpc_cidr
  azs  = local.azs

  providers = {
    aws = aws.west
  }
}

resource "aws_security_group" "nat" {
  name   = "${local.prefix}-nat"
  vpc_id = module.vpc.vpc_id

  # デフォルトのイテレータ名（ingress）
  dynamic "ingress" {
    for_each = var.ingress_rules
    content {
      from_port   = ingress.value.port
      to_port     = ingress.value.port
      protocol    = "tcp"
      cidr_blocks = [module.vpc.cidr]
    }
  }

  # iterator 指定
  dynamic "egress" {
    for_each = var.egress_rules
    iterator = rule
    content {
      from_port   = rule.value.port
      to_port     = rule.value.port
      protocol    = "tcp"
      cidr_blocks = local.egress_cidrs
    }
  }

  lifecycle {
    create_before_destroy = true
    ignore_changes        = [tags, description]
  }
}

resource "aws_instance" "nat" {
  count = 2

  provider               = aws.west
  ami                    = data.aws_ami.nat.id
  instance_type          = "t3.micro"
  subnet_id              = module.vpc.public_subnet_ids[count.index]
  vpc_security_group_ids = [aws_security_group.nat.id]

  tags = {
    Name = "${local.prefix}-nat-${count.index}"
  }
}

resource "aws_eip" "nat" {
  count    = length(aws_instance.nat)
  instance = aws_instance.nat[count.index].id

  depends_on = [module.vpc]

  lifecycle {
    replace_triggered_by = [aws_instance.nat[0].id]

    postcondition {
      condition     = self.public_ip != ""
      error_message = "no public ip"
    }
  }
}

# registry ソースのモジュール（中身の見えないノードになる）
module "flow_logs" {
  source  = "terraform-aws-modules/vpc-flow-log/aws"
  version = "1.0.0"

  vpc_id = module.vpc.vpc_id
}

# 未解決の参照
resource "aws_route53_record" "nat" {
  zone_id = var.zone_id
  name    = "nat.example.com"
  type    = "A"
  records = aws_instance.bastion[*].private_ip
}
