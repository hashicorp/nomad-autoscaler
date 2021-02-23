resource "google_compute_instance_template" "nomad_client" {
  name         = local.client_stack_name
  machine_type = var.client_machine_type

  disk {
    source_image = data.google_compute_image.hashistack.id
    disk_size_gb = var.vm_disk_size
    auto_delete  = true
    boot         = true
  }

  network_interface {
    subnetwork         = google_compute_subnetwork.hashistack.name
    subnetwork_project = var.project_id

    access_config {
      network_tier = "STANDARD"
    }
  }

  tags = ["nomad-client"]

  service_account {
    scopes = [
      "https://www.googleapis.com/auth/compute",
    ]
  }

  metadata = {
    startup-script = templatefile("${path.module}/templates/user-data-client.sh", {
      retry_join    = format("provider=gce project_name=%s tag_value=nomad-server", var.project_id)
      consul_binary = var.consul_binary
      nomad_binary  = var.nomad_binary
      node_class    = "hashistack"
    })
  }
}

resource "google_compute_instance_group_manager" "nomad_client" {
  count = local.client_regional_mig ? 0 : 1

  name               = local.client_stack_name
  base_instance_name = local.client_stack_name
  zone               = local.zone_id
  target_size        = "1"
  target_pools       = [google_compute_target_pool.nomad_client.id]

  version {
    name              = local.client_stack_name
    instance_template = google_compute_instance_template.nomad_client.id
  }
}

resource "google_compute_region_instance_group_manager" "nomad_client" {
  count = local.client_regional_mig ? 1 : 0

  name                      = local.client_stack_name
  base_instance_name        = local.client_stack_name
  region                    = var.region
  distribution_policy_zones = [local.zone_id]
  target_size               = "1"
  target_pools              = [google_compute_target_pool.nomad_client.id]

  version {
    name              = local.client_stack_name
    instance_template = google_compute_instance_template.nomad_client.id
  }
}
