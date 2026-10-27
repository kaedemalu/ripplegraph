variable "vpc_cidr" {
  type = string
}

variable "excluded_az" {
  type    = string
  default = ""
}

variable "ingress_rules" {
  type = list(object({
    port = number
  }))
  default = []

  validation {
    condition     = length(var.ingress_rules) < 10
    error_message = "too many rules"
  }
}

variable "egress_rules" {
  type    = list(object({ port = number }))
  default = []
}
