resource "azurerm_public_ip" "servers_lb" {
  name                = "server-lb-ip"
  location            = azurerm_resource_group.hashistack.location
  resource_group_name = azurerm_resource_group.hashistack.name
  allocation_method   = "Static"
}

resource "azurerm_lb" "servers" {
  depends_on = [azurerm_linux_virtual_machine.servers]

  name                = "servers-lb"
  location            = azurerm_resource_group.hashistack.location
  resource_group_name = azurerm_resource_group.hashistack.name

  frontend_ip_configuration {
    name                 = "PublicIPAddress"
    public_ip_address_id = azurerm_public_ip.servers_lb.id
  }
}

resource "azurerm_lb_backend_address_pool" "servers_lb" {
  name                = "servers-lb-backend"
  resource_group_name = azurerm_resource_group.hashistack.name
  loadbalancer_id     = azurerm_lb.servers.id
}

resource "azurerm_lb_probe" "servers_nomad" {
  name                = "servers-nomad-lb-probe"
  resource_group_name = azurerm_resource_group.hashistack.name
  loadbalancer_id     = azurerm_lb.servers.id
  port                = 4646
}

resource "azurerm_lb_rule" "servers_nomad" {
  name                           = "servers-nomad-lb-rule"
  resource_group_name            = azurerm_resource_group.hashistack.name
  loadbalancer_id                = azurerm_lb.servers.id
  backend_address_pool_id        = azurerm_lb_backend_address_pool.servers_lb.id
  probe_id                       = azurerm_lb_probe.servers_nomad.id
  protocol                       = "Tcp"
  frontend_port                  = 4646
  backend_port                   = 4646
  frontend_ip_configuration_name = "PublicIPAddress"
}

resource "azurerm_lb_probe" "servers_consul" {
  name                = "servers-consul-lb-probe"
  resource_group_name = azurerm_resource_group.hashistack.name
  loadbalancer_id     = azurerm_lb.servers.id
  port                = 8500
}

resource "azurerm_lb_rule" "servers_consul" {
  name                           = "servers-consul-lb-rule"
  resource_group_name            = azurerm_resource_group.hashistack.name
  loadbalancer_id                = azurerm_lb.servers.id
  backend_address_pool_id        = azurerm_lb_backend_address_pool.servers_lb.id
  probe_id                       = azurerm_lb_probe.servers_consul.id
  protocol                       = "Tcp"
  frontend_port                  = 8500
  backend_port                   = 8500
  frontend_ip_configuration_name = "PublicIPAddress"
}
