terraform {
  required_version = ">= 1.5"
  backend "s3" {
    bucket = "example-tfstate"
    key    = "dev/network.tfstate"
    region = "ap-northeast-1"
  }
}

provider "aws" {
  region = "ap-northeast-1"
}

provider "aws" {
  alias  = "west"
  region = "us-west-2"
}
