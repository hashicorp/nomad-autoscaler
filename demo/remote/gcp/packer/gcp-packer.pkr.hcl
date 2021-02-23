variable "project_id" {}
variable "image_name" { default = "hashistack" }
variable "source_image" { default = "ubuntu-2004-focal-v20200720" }
variable "ssh_username" { default = "ubuntu" }
variable "zone" { default = "us-central1-a" }

source "googlecompute" "hashistack" {
  image_name   = "${var.image_name}"
  project_id   = "${var.project_id}"
  source_image = "${var.source_image}"
  ssh_username = "${var.ssh_username}"
  zone         = "${var.zone}"
}

build {
  sources = [
    "source.googlecompute.hashistack"
  ]

  provisioner "shell" {
    inline = [
      "sudo mkdir -p /ops",
      "sudo chmod 777 /ops"
    ]
  }

  provisioner "file" {
    source      = "../../shared/packer/"
    destination = "/ops"
  }

  provisioner "shell" {
    script = "../../shared/packer/scripts/setup.sh"
  }
}
