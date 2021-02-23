resource "google_compute_instance" "nomad_server" {
  count        = var.server_count
  name         = local.server_stack_name
  machine_type = var.server_machine_type
  zone         = local.zone_id

  depends_on = [google_project_service.compute]

  boot_disk {
    initialize_params {
      image = data.google_compute_image.hashistack.id
      size  = var.vm_disk_size
    }
  }

  network_interface {
    subnetwork         = google_compute_subnetwork.hashistack.name
    subnetwork_project = var.project_id

    access_config {
      network_tier = "STANDARD"
    }
  }

  tags = ["nomad-server"]

  service_account {
    scopes = [
      "https://www.googleapis.com/auth/compute",
    ]
  }

  metadata = {
    startup-script = templatefile("${path.module}/templates/user-data-server.sh", {
      server_count  = var.server_count
      retry_join    = "provider=gce project_name={$var.project_id} tag_value=nomad-server"
      consul_binary = var.consul_binary
      nomad_binary  = var.nomad_binary
    })
  }
}
