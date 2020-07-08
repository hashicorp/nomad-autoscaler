terraform {
  required_version = ">= 0.12"
}

provider "nomad" {
  version = "~> 1.4.6"
  address = var.nomad_addr
}