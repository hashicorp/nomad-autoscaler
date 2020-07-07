module "my_ip_address" {
  source = "matti/resource/shell"

  command = "curl https://ipinfo.io/ip"
}

module "hashistack_cluster" {
  source = "../../modules/aws-hashistack"

  owner_name         = var.owner_name
  owner_email        = var.owner_email
  region             = var.region
  availability_zones = var.availability_zones
  ami                = var.ami
  key_name           = var.key_name
  allowlist_ip       = ["${module.my_ip_address.stdout}/32"]
}

module "hashistack_jobs" {
  source = "../../modules/shared-nomad-jobs"

  nomad_addr = "http://${module.hashistack_cluster.server_elb_dns}:4646"
}
