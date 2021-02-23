terraform {
  required_version = ">= 0.12"

  required_providers {
    azurerm = {
      version = "2.32.0"
    }
  }
}

provider "nomad" {
  address = module.hashistack_cluster.nomad_addr
}

provider "azurerm" {
  features {}
}

module "my_ip_address" {
  source = "matti/resource/shell"

  command = "curl https://ipinfo.io/ip"
}

module "hashistack_cluster" {
  source = "../modules/azure-hashistack"

  allowlist_ip = ["${module.my_ip_address.stdout}/32"]
}

module "hashistack_jobs" {
  source     = "../../../shared/terraform/modules/shared-nomad-jobs"
  depends_on = [module.hashistack_cluster]

  nomad_addr = module.hashistack_cluster.nomad_addr
}
