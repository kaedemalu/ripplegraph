output "nat_ips" {
  value = aws_eip.nat[*].public_ip
}

output "vpc_id" {
  value = module.vpc.vpc_id
}
