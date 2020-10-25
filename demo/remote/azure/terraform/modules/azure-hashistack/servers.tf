resource "azurerm_linux_virtual_machine" "servers" {
  count               = var.server_count
  name                = "server-${count.index + 1}"
  location            = azurerm_resource_group.hashistack.location
  resource_group_name = azurerm_resource_group.hashistack.name
  size                = var.server_vm_size
  source_image_id     = data.azurerm_image.hashistack.id
  custom_data         = base64encode(data.template_file.user_data_server.rendered)
  admin_username      = "ubuntu"

  network_interface_ids = [
    azurerm_network_interface.server_ni[count.index].id,
  ]

  os_disk {
    caching              = "ReadWrite"
    storage_account_type = "Standard_LRS"
  }

  admin_ssh_key {
    username   = "ubuntu"
    public_key = tls_private_key.main.public_key_openssh
  }

  identity {
    type         = "UserAssigned"
    identity_ids = [azurerm_user_assigned_identity.consul_auto_join.id]
  }
}

# Networking
resource "azurerm_public_ip" "server_public_ip" {
  count               = var.server_count
  name                = "server-${count.index + 1}-ip"
  location            = azurerm_resource_group.hashistack.location
  resource_group_name = azurerm_resource_group.hashistack.name
  allocation_method   = "Static"
}

resource "azurerm_network_interface" "server_ni" {
  count               = var.server_count
  name                = "server-${count.index + 1}-ni"
  location            = azurerm_resource_group.hashistack.location
  resource_group_name = azurerm_resource_group.hashistack.name

  ip_configuration {
    name                          = "server-ipc"
    subnet_id                     = azurerm_subnet.primary.id
    private_ip_address_allocation = "dynamic"
    public_ip_address_id          = azurerm_public_ip.server_public_ip[count.index].id
  }

  tags = {
    ConsulAutoJoin = "auto-join"
  }
}

# Load balancer
resource "azurerm_network_interface_backend_address_pool_association" "server_ni_bap" {
  count                   = var.server_count
  ip_configuration_name   = "server-ipc"
  network_interface_id    = azurerm_network_interface.server_ni[count.index].id
  backend_address_pool_id = azurerm_lb_backend_address_pool.servers_lb.id
}

# Security group
resource "azurerm_network_interface_security_group_association" "server_ni_sg" {
  count                     = var.server_count
  network_interface_id      = azurerm_network_interface.server_ni[count.index].id
  network_security_group_id = azurerm_network_security_group.nomad_servers.id
}

# Consul Auto Join credentials
resource "azurerm_user_assigned_identity" "consul_auto_join" {
  name                = "consul-auto-join"
  resource_group_name = azurerm_resource_group.hashistack.name
  location            = azurerm_resource_group.hashistack.location
}

resource "azurerm_role_assignment" "consul_auto_join" {
  scope                = data.azurerm_subscription.main.id
  role_definition_name = "Reader"
  principal_id         = azurerm_user_assigned_identity.consul_auto_join.principal_id
}
