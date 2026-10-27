terraform {
  backend "s3" {
    bucket = "example-tfstate"
    key    = "prd/network.tfstate"
    region = "ap-northeast-1"
  }
}

provider "aws" {
  region = "ap-northeast-1"
}
