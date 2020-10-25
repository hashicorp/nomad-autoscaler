resource "random_pet" "server" {}

resource "azurerm_resource_group" "hashistack" {
  name     = "hashistack-${random_pet.server.id}"
  location = var.location
}
