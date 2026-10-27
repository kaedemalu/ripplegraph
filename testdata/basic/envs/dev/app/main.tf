terraform {
  backend "s3" {
    bucket = "example-tfstate"
    key    = "dev/app.tfstate"
    region = "ap-northeast-1"
  }
}

provider "aws" {
  region = "ap-northeast-1"
}

data "terraform_remote_state" "network" {
  backend = "s3"
  config = {
    bucket = "example-tfstate"
    key    = "dev/network.tfstate"
    region = "ap-northeast-1"
  }
}

resource "aws_instance" "web" {
  count     = 2
  ami       = "ami-123456"
  subnet_id = data.terraform_remote_state.network.outputs.vpc_id
}

resource "aws_lb_target_group" "web" {
  name     = "web"
  port     = 80
  protocol = "HTTP"
  vpc_id   = data.terraform_remote_state.network.outputs.vpc_id
}

# インデックス付きの参照
resource "aws_lb_target_group_attachment" "web" {
  target_group_arn = aws_lb_target_group.web.arn
  target_id        = aws_instance.web[0].id
}
