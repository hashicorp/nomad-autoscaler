provider "nomad" {
  address = module.hashistack_cluster.nomad_addr
}

module "my_ip_address" {
  source = "matti/resource/shell"

  command = "curl https://ipinfo.io/ip"
}

module "hashistack_cluster" {
  source = "../modules/azure-hashistack"

  allowlist_ip = ["${module.my_ip_address.stdout}/32"]

  nomad_binary           = "https://releases.hashicorp.com/nomad/1.0.1/nomad_1.0.1_linux_amd64.zip"
  nomad_autoscaler_image = "hashicorp/nomad-autoscaler:0.2.1"
}

module "hashistack_jobs" {
  source     = "../../../terraform/modules/shared-nomad-jobs"
  depends_on = [module.hashistack_cluster]

  nomad_addr = module.hashistack_cluster.nomad_addr
}
