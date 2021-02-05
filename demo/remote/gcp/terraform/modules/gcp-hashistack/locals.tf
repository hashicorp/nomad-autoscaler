locals {
  zone_id             = format("%s-%s", var.region, var.zone)
  client_regional_mig = var.client_mig_type == "regional"
  client_mig_name     = local.client_regional_mig ? google_compute_region_instance_group_manager.nomad_client[0].name : google_compute_instance_group_manager.nomad_client[0].name
  stack_name          = var.stack_name
  client_stack_name   = "${var.stack_name}-nomad-client"
  server_stack_name   = "${var.stack_name}-nomad-server"
}
