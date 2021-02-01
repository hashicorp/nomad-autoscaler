resource "google_compute_router" "hashistack" {
  name    = local.stack_name
  project = google_project.hashistack.project_id
  region  = var.region
  network = google_compute_network.hashistack.name
  bgp {
    asn = 64514
  }
}

resource "google_compute_router_nat" "hashistack" {
  name                               = local.stack_name
  project                            = google_project.hashistack.project_id
  region                             = google_compute_router.hashistack.region
  router                             = google_compute_router.hashistack.name
  nat_ip_allocate_option             = "AUTO_ONLY"
  source_subnetwork_ip_ranges_to_nat = "ALL_SUBNETWORKS_ALL_IP_RANGES"
}

resource "google_compute_network" "hashistack" {
  name                    = local.stack_name
  project                 = google_project.hashistack.project_id
  auto_create_subnetworks = false
}

resource "google_compute_subnetwork" "hashistack" {
  network       = google_compute_network.hashistack.name
  name          = local.stack_name
  region        = var.region
  project       = google_project.hashistack.project_id
  ip_cidr_range = var.ip_cidr_range
}
