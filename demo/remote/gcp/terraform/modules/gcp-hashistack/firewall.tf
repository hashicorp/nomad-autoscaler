resource "google_compute_firewall" "nomad_consul_generic" {
  name    = format("%s-allow-consul-nomad", local.stack_name)
  network = google_compute_network.hashistack.name
  project = google_project.hashistack.project_id

  allow {
    protocol = "icmp"
  }

  allow {
    protocol = "tcp"
    ports = [
      "4646",
      "4647",
      "4647",
      "4648",
      "8500",
      "8501",
      "8502",
      "8600",
      "8300",
      "8302",
      "8301",
    ]
  }

  allow {
    protocol = "udp"
    ports = [
      "4648",
      "8600",
      "8302",
      "8301",
    ]
  }

  source_tags = [
    "nomad-server",
    "nomad-client",
  ]
}

resource "google_compute_firewall" "allow-all-internal-dyanmic-ports" {
  name    = format("%s-allow-dynamic-ports", local.stack_name)
  network = google_compute_network.hashistack.name
  project = google_project.hashistack.project_id

  source_tags = [
    "nomad-server",
    "nomad-client",
  ]

  allow {
    protocol = "tcp"
    ports = [
      "20000-32000",
    ]
  }
}

resource "google_compute_firewall" "allow_server_ingress" {
  name          = format("%s-allow-ingress", local.server_stack_name)
  network       = google_compute_network.hashistack.name
  project       = google_project.hashistack.project_id
  direction     = "INGRESS"
  source_ranges = var.allowlist_ip

  allow {
    protocol = "tcp"
    ports = [
      "22",
      "4646",
      "8500",
    ]
  }
}

resource "google_compute_firewall" "allow_client_ingress" {
  name          = format("%s-allow-ingress", local.client_stack_name)
  network       = google_compute_network.hashistack.name
  project       = google_project.hashistack.project_id
  direction     = "INGRESS"
  source_ranges = var.allowlist_ip

  allow {
    protocol = "tcp"
    ports = [
      "80",
      "3000",
      "8081",
      "9090",
    ]
  }
}
