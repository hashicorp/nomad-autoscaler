module "my_ip_address" {
  source = "matti/resource/shell"

  command = "curl https://ipinfo.io/ip"
}

module "hashistack_cluster" {
  source = "../modules/azure-hashistack"

  allowlist_ip = ["${module.my_ip_address.stdout}/32"]
}

module "hashistack_jobs" {
  source = "../../../terraform/modules/shared-nomad-jobs"

  nomad_addr = module.hashistack_cluster.nomad_addr
}
