data "template_file" "nomad_autoscaler_jobspec" {
  template = file("${path.module}/templates/gcp_autoscaler.nomad.tpl")

  vars = {
    nomad_autoscaler_image = var.nomad_autoscaler_image
    project                = var.project_id
    region                 = var.region
    zone                   = local.zone_id
    mig_type               = var.client_mig_type
    mig_name               = local.client_mig_name
  }
}

resource "null_resource" "nomad_autoscaler_jobspec" {
  provisioner "local-exec" {
    command = "echo '${data.template_file.nomad_autoscaler_jobspec.rendered}' > gcp_autoscaler.nomad"
  }
}
