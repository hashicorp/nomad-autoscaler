resource "azurerm_linux_virtual_machine_scale_set" "clients" {
  name                = "clients"
  location            = azurerm_resource_group.hashistack.location
  resource_group_name = azurerm_resource_group.hashistack.name
  sku                 = var.client_vm_size
  source_image_id     = data.azurerm_image.hashistack.id
  custom_data         = base64encode(data.template_file.user_data_client.rendered)
  instances           = var.client_count
  admin_username      = "ubuntu"

  network_interface {
    name                      = "client-vmss-ni"
    primary                   = true
    network_security_group_id = azurerm_network_security_group.nomad_clients.id

    ip_configuration {
      name                                   = "PrivateIPConfiguration"
      primary                                = true
      subnet_id                              = azurerm_subnet.primary.id
      load_balancer_backend_address_pool_ids = [azurerm_lb_backend_address_pool.clients_lb.id]
      public_ip_address {
        name = "client-vmss-public-ip"
      }
    }
  }

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
    identity_ids = [azurerm_user_assigned_identity.clients_vmss.id]
  }
}

# Managed identity
resource "azurerm_user_assigned_identity" "clients_vmss" {
  name                = "clients-vmss"
  resource_group_name = azurerm_resource_group.hashistack.name
  location            = azurerm_resource_group.hashistack.location
}

resource "azurerm_role_assignment" "clients_vmss" {
  scope                = data.azurerm_subscription.main.id
  role_definition_name = "Contributor"
  principal_id         = azurerm_user_assigned_identity.clients_vmss.principal_id
}
