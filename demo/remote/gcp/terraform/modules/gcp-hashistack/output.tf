output "google_project_id" {
  value = var.project_id
}

output "nomad_addr" {
  value = "http://${google_compute_address.nomad_server.address}:4646"
}

output "consul_addr" {
  value = "http://${google_compute_address.nomad_server.address}:8500"
}

output "client_public_ip_addr" {
  value = google_compute_address.nomad_client.address
}
