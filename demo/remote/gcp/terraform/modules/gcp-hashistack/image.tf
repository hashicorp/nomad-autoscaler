locals {
  hashistack_image_project_id = var.hashistack_image_project_id != "" ? var.hashistack_image_project_id : google_project.hashistack.project_id
}

resource "null_resource" "packer_build" {
  count      = var.build_hashistack_image ? 1 : 0
  depends_on = [google_project.hashistack, google_project_service.compute]

  provisioner "local-exec" {
    command = <<EOF
cd ../../packer && \
  packer build -force \
    -var zone=${local.zone_id} \
    -var project_id=${local.hashistack_image_project_id} \
    -var image_name=${var.hashistack_image_name} \
    gcp-packer.pkr.hcl
EOF
  }
}

data "google_compute_image" "hashistack" {
  depends_on = [null_resource.packer_build]
  name       = var.hashistack_image_name
  project    = local.hashistack_image_project_id
}
