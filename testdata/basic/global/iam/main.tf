provider "aws" {
  region = "ap-northeast-1"
}

resource "aws_iam_role" "ci" {
  name               = "ci"
  assume_role_policy = data.aws_iam_policy_document.ci.json
}

data "aws_iam_policy_document" "ci" {
  statement {
    actions = ["sts:AssumeRole"]
  }
}
