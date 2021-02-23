resource "google_compute_forwarding_rule" "nomad_server" {
  name       = format("%s-4646", local.server_stack_name)
  region     = var.region
  port_range = 4646
  target     = google_compute_target_pool.nomad_server.id
  ip_address = google_compute_address.nomad_server.address
}

resource "google_compute_forwarding_rule" "nomad_server_8500" {
  name       = format("%s-8500", local.server_stack_name)
  region     = var.region
  port_range = 8500
  target     = google_compute_target_pool.nomad_server.id
  ip_address = google_compute_address.nomad_server.address
}

resource "google_compute_target_pool" "nomad_server" {
  name          = local.server_stack_name
  instances     = google_compute_instance.nomad_server.*.self_link
  health_checks = [google_compute_http_health_check.nomad_server.name]
}

resource "google_compute_http_health_check" "nomad_server" {
  name               = "${local.server_stack_name}-leader-status"
  request_path       = "/v1/status/leader"
  check_interval_sec = 5
  timeout_sec        = 1
  port               = 4646
}

resource "google_compute_forwarding_rule" "nomad_client_80" {
  name       = format("%s-80", local.client_stack_name)
  region     = var.region
  port_range = 80
  target     = google_compute_target_pool.nomad_client.id
  ip_address = google_compute_address.nomad_client.address
}

resource "google_compute_forwarding_rule" "nomad_client_9090" {
  name       = format("%s-9090", local.client_stack_name)
  region     = var.region
  port_range = 9090
  target     = google_compute_target_pool.nomad_client.id
  ip_address = google_compute_address.nomad_client.address
}

resource "google_compute_forwarding_rule" "nomad_client_3000" {
  name       = format("%s-3000", local.client_stack_name)
  region     = var.region
  port_range = 3000
  target     = google_compute_target_pool.nomad_client.id
  ip_address = google_compute_address.nomad_client.address
}

resource "google_compute_forwarding_rule" "nomad_client_8081" {
  name       = format("%s-8081", local.client_stack_name)
  region     = var.region
  port_range = 8081
  target     = google_compute_target_pool.nomad_client.id
  ip_address = google_compute_address.nomad_client.address
}

resource "google_compute_target_pool" "nomad_client" {
  name          = local.client_stack_name
  health_checks = [google_compute_http_health_check.nomad_server.name]
}

resource "google_compute_health_check" "nomad_client" {
  name               = "${local.server_stack_name}-tcp-8081"
  timeout_sec        = 3
  check_interval_sec = 30

  tcp_health_check {
    port = "8081"
  }
}
