# Providers
provider "nomad" {
  address = module.hashistack_cluster.nomad_addr
}

provider "google" {
  region = var.region
  zone   = var.zone
}

provider "google" {
  alias   = "with_project"
  region  = var.region
  zone    = var.zone
  project = google_project.hashistack.project_id
}

# Modules
module "my_ip_address" {
  source  = "matti/resource/shell"
  command = "curl https://ipinfo.io/ip"
}

module "hashistack_cluster" {
  source = "../modules/gcp-hashistack"
  providers = {
    google = google.with_project
  }

  project_id   = google_project.hashistack.project_id
  allowlist_ip = ["${module.my_ip_address.stdout}/32"]
}

module "hashistack_jobs" {
  source     = "../../../shared/terraform/modules/shared-nomad-jobs"
  depends_on = [module.hashistack_cluster]
  nomad_addr = module.hashistack_cluster.nomad_addr
}

# GCP Project
resource "random_pet" "hashistack" {}

resource "google_project" "hashistack" {
  name            = "hashistack-${random_pet.hashistack.id}"
  project_id      = "hashistack-${random_pet.hashistack.id}"
  org_id          = var.org_id
  billing_account = var.billing_account
}
