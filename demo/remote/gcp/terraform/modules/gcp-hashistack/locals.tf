locals {
  zone_id           = format("%s-%s", var.region, var.zone)
  stack_name        = google_project.hashistack.name
  client_stack_name = "${google_project.hashistack.name}-nomad-client"
  server_stack_name = "${google_project.hashistack.name}-nomad-server"
}
