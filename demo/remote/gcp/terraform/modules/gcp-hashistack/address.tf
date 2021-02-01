resource "google_compute_address" "nomad_server" {
  name = local.server_stack_name
}

resource "google_compute_address" "nomad_client" {
  name = local.client_stack_name
}
