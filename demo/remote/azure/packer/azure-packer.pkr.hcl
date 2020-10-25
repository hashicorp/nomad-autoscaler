variable "client_id" {}
variable "client_secret" {}
variable "resource_group" {}
variable "subscription_id" {}
variable "location" { default = "East US" }
variable "image_name" { default = "hashistack" }

source "azure-arm" "hashistack" {
  azure_tags = {
    Product = "Hashistack"
  }
  client_id                         = "${var.client_id}"
  client_secret                     = "${var.client_secret}"
  image_offer                       = "UbuntuServer"
  image_publisher                   = "Canonical"
  image_sku                         = "18.04-LTS"
  location                          = "${var.location}"
  managed_image_name                = "${var.image_name}"
  managed_image_resource_group_name = "${var.resource_group}"
  os_type                           = "Linux"
  ssh_username                      = "packer"
  subscription_id                   = "${var.subscription_id}"
}

build {
  sources = [
    "source.azure-arm.hashistack"
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
