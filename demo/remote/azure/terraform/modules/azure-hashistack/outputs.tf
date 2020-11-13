output "server_public_ips" {
  value = azurerm_linux_virtual_machine.servers.*.public_ip_address
}

output "server_private_ips" {
  value = azurerm_linux_virtual_machine.servers.*.private_ip_address
}

output "server_addresses" {
  value = join("\n", formatlist(
    " * instance %v - Public: %v, Private: %v",
    azurerm_linux_virtual_machine.servers.*.name,
    azurerm_linux_virtual_machine.servers.*.public_ip_address,
    azurerm_linux_virtual_machine.servers.*.private_ip_address
  ))
}

output "server_lb_id" {
  value = azurerm_lb.servers.id
}

output "server_lb_public_ip" {
  value = azurerm_public_ip.servers_lb.ip_address
}

output "clients_lb_id" {
  value = azurerm_lb.clients.id
}

output "clients_lb_public_ip" {
  value = azurerm_public_ip.clients_lb.ip_address
}

output "nomad_addr" {
  value = "http://${azurerm_public_ip.servers_lb.ip_address}:4646"

  depends_on = [
    azurerm_linux_virtual_machine.servers,
    azurerm_linux_virtual_machine_scale_set.clients,
  ]
}

output "consul_addr" {
  value = "http://${azurerm_public_ip.servers_lb.ip_address}:8500"

  depends_on = [
    azurerm_linux_virtual_machine.servers,
    azurerm_linux_virtual_machine_scale_set.clients,
  ]
}

output "client_vmss_id" {
  value = azurerm_linux_virtual_machine_scale_set.clients.id
}

output "client_vmss_name" {
  value = azurerm_linux_virtual_machine_scale_set.clients.name
}
