resource "random_pet" "hashistack" {}

resource "google_project" "hashistack" {
  name            = "hashistack-${random_pet.hashistack.id}"
  project_id      = "hashistack-${random_pet.hashistack.id}"
  org_id          = var.org_id
  billing_account = var.billing_account
}

resource "google_project_service" "compute" {
  project            = google_project.hashistack.project_id
  service            = "compute.googleapis.com"
  disable_on_destroy = false
}
