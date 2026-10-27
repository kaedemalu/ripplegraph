variable "name" {
  type = string
}

resource "aws_lb" "this" {
  name = var.name
}

output "url" {
  value = "https://${aws_lb.this.dns_name}"
}
