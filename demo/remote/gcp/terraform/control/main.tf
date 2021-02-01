provider "nomad" {
  address = module.hashistack_cluster.nomad_addr
}

module "my_ip_address" {
  source  = "matti/resource/shell"
  command = "curl https://ipinfo.io/ip"
}

module "hashistack_cluster" {
  source          = "../modules/gcp-hashistack"
  org_id          = var.org_id
  billing_account = var.billing_account
  allowlist_ip    = ["${module.my_ip_address.stdout}/32"]
}

module "hashistack_jobs" {
  source     = "../../../shared/terraform/modules/shared-nomad-jobs"
  depends_on = [module.hashistack_cluster]
  nomad_addr = module.hashistack_cluster.nomad_addr
}
