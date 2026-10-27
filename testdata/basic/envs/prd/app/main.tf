terraform {
  backend "s3" {
    bucket = "example-tfstate"
    key    = "prd/app.tfstate"
    region = "ap-northeast-1"
  }
}

provider "aws" {
  region = "ap-northeast-1"
}

module "app" {
  for_each = toset(["a", "b"])
  source   = "../../../modules/service"

  name = each.key
}

# 存在しないディレクトリを指すモジュール
module "legacy" {
  source = "../../../modules/does-not-exist"
}

resource "aws_route53_record" "app_a" {
  zone_id = "Z123"
  name    = "a.example.com"
  type    = "CNAME"
  records = [module.app["a"].url]
}

output "a_url" {
  value = module.app["a"].url
}
