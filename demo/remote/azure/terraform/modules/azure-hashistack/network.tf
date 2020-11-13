resource "azurerm_virtual_network" "primary" {
  name                = "virtual-network"
  address_space       = ["10.0.0.0/16"]
  location            = azurerm_resource_group.hashistack.location
  resource_group_name = azurerm_resource_group.hashistack.name
}

resource "azurerm_subnet" "primary" {
  name                 = "subnet"
  resource_group_name  = azurerm_resource_group.hashistack.name
  virtual_network_name = azurerm_virtual_network.primary.name
  address_prefixes     = ["10.0.2.0/24"]
}
