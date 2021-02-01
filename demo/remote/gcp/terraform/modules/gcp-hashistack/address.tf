resource "google_compute_address" "nomad_server" {
  name    = local.server_stack_name
  project = google_project.hashistack.project_id
}

resource "google_compute_address" "nomad_client" {
  name    = local.client_stack_name
  project = google_project.hashistack.project_id
}
