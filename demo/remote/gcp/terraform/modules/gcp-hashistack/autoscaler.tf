data "template_file" "nomad_autoscaler_jobspec" {
  template = file("${path.module}/templates/gcp_autoscaler.nomad.tpl")

  vars = {
    nomad_autoscaler_image = var.nomad_autoscaler_image
    project                = var.project_id
    region                 = var.region
    mig_name               = google_compute_region_instance_group_manager.nomad_client.name
  }
}

resource "null_resource" "nomad_autoscaler_jobspec" {
  provisioner "local-exec" {
    command = "echo '${data.template_file.nomad_autoscaler_jobspec.rendered}' > gcp_autoscaler.nomad"
  }
}
