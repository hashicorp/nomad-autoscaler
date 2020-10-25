resource "azurerm_public_ip" "clients_lb" {
  name                = "clients-lb-ip"
  location            = azurerm_resource_group.hashistack.location
  resource_group_name = azurerm_resource_group.hashistack.name
  allocation_method   = "Static"
}

resource "azurerm_lb" "clients" {
  name                = "clients-lb"
  location            = azurerm_resource_group.hashistack.location
  resource_group_name = azurerm_resource_group.hashistack.name

  frontend_ip_configuration {
    name                 = "PublicIPAddress"
    public_ip_address_id = azurerm_public_ip.clients_lb.id
  }
}

resource "azurerm_lb_backend_address_pool" "clients_lb" {
  name                = "clients-lb-backend"
  resource_group_name = azurerm_resource_group.hashistack.name
  loadbalancer_id     = azurerm_lb.clients.id
}

# Nomad rule
resource "azurerm_lb_probe" "clients_nomad" {
  name                = "clients-nomad-lb-probe"
  resource_group_name = azurerm_resource_group.hashistack.name
  loadbalancer_id     = azurerm_lb.clients.id
  port                = 4646
}

resource "azurerm_lb_rule" "clients_nomad" {
  name                           = "clients-nomad-lb-rule"
  resource_group_name            = azurerm_resource_group.hashistack.name
  loadbalancer_id                = azurerm_lb.clients.id
  backend_address_pool_id        = azurerm_lb_backend_address_pool.clients_lb.id
  probe_id                       = azurerm_lb_probe.clients_nomad.id
  protocol                       = "Tcp"
  frontend_port                  = 4646
  backend_port                   = 4646
  frontend_ip_configuration_name = "PublicIPAddress"
}

# Consul rule
resource "azurerm_lb_probe" "clients_consul" {
  name                = "clients-consul-lb-probe"
  resource_group_name = azurerm_resource_group.hashistack.name
  loadbalancer_id     = azurerm_lb.clients.id
  port                = 8500
}

resource "azurerm_lb_rule" "clients_consul" {
  name                           = "clients-consul-lb-rule"
  resource_group_name            = azurerm_resource_group.hashistack.name
  loadbalancer_id                = azurerm_lb.clients.id
  backend_address_pool_id        = azurerm_lb_backend_address_pool.clients_lb.id
  probe_id                       = azurerm_lb_probe.clients_consul.id
  protocol                       = "Tcp"
  frontend_port                  = 8500
  backend_port                   = 8500
  frontend_ip_configuration_name = "PublicIPAddress"
}

# Grafana rule
resource "azurerm_lb_probe" "clients_grafana" {
  name                = "clients-grafana-lb-probe"
  resource_group_name = azurerm_resource_group.hashistack.name
  loadbalancer_id     = azurerm_lb.clients.id
  port                = 3000
}

resource "azurerm_lb_rule" "clients_grafana" {
  name                           = "clients-grafana-lb-rule"
  resource_group_name            = azurerm_resource_group.hashistack.name
  loadbalancer_id                = azurerm_lb.clients.id
  backend_address_pool_id        = azurerm_lb_backend_address_pool.clients_lb.id
  probe_id                       = azurerm_lb_probe.clients_grafana.id
  protocol                       = "Tcp"
  frontend_port                  = 3000
  backend_port                   = 3000
  frontend_ip_configuration_name = "PublicIPAddress"
}

# Prometheus rule
resource "azurerm_lb_probe" "clients_prometheus" {
  name                = "clients-prometheus-lb-probe"
  resource_group_name = azurerm_resource_group.hashistack.name
  loadbalancer_id     = azurerm_lb.clients.id
  port                = 9090
}

resource "azurerm_lb_rule" "clients_prometheus" {
  name                           = "clients-prometheus-lb-rule"
  resource_group_name            = azurerm_resource_group.hashistack.name
  loadbalancer_id                = azurerm_lb.clients.id
  backend_address_pool_id        = azurerm_lb_backend_address_pool.clients_lb.id
  probe_id                       = azurerm_lb_probe.clients_prometheus.id
  protocol                       = "Tcp"
  frontend_port                  = 9090
  backend_port                   = 9090
  frontend_ip_configuration_name = "PublicIPAddress"
}

# Traefik rule
resource "azurerm_lb_probe" "clients_traefik" {
  name                = "clients-traefik-lb-probe"
  resource_group_name = azurerm_resource_group.hashistack.name
  loadbalancer_id     = azurerm_lb.clients.id
  port                = 8081
}

resource "azurerm_lb_rule" "clients_traefik" {
  name                           = "clients-traefik-lb-rule"
  resource_group_name            = azurerm_resource_group.hashistack.name
  loadbalancer_id                = azurerm_lb.clients.id
  backend_address_pool_id        = azurerm_lb_backend_address_pool.clients_lb.id
  probe_id                       = azurerm_lb_probe.clients_traefik.id
  protocol                       = "Tcp"
  frontend_port                  = 8081
  backend_port                   = 8081
  frontend_ip_configuration_name = "PublicIPAddress"
}

# HTTP rule
resource "azurerm_lb_probe" "clients_http" {
  name                = "clients-http-lb-probe"
  resource_group_name = azurerm_resource_group.hashistack.name
  loadbalancer_id     = azurerm_lb.clients.id
  port                = 80
}

resource "azurerm_lb_rule" "clients_http" {
  name                           = "clients-http-lb-rule"
  resource_group_name            = azurerm_resource_group.hashistack.name
  loadbalancer_id                = azurerm_lb.clients.id
  backend_address_pool_id        = azurerm_lb_backend_address_pool.clients_lb.id
  probe_id                       = azurerm_lb_probe.clients_http.id
  protocol                       = "Tcp"
  frontend_port                  = 80
  backend_port                   = 80
  frontend_ip_configuration_name = "PublicIPAddress"
}
