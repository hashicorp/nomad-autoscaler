resource "azurerm_network_security_group" "nomad_servers" {
  name                = "nomad-servers-sg"
  location            = azurerm_resource_group.hashistack.location
  resource_group_name = azurerm_resource_group.hashistack.name

  # SSH
  security_rule {
    name = "primary-sgr-22"

    priority  = 100
    direction = "Inbound"
    access    = "Allow"
    protocol  = "Tcp"

    source_address_prefixes    = var.allowlist_ip
    source_port_range          = "*"
    destination_port_range     = 22
    destination_address_prefix = "*"
  }

  # Nomad
  security_rule {
    name = "primary-sgr-4646"

    priority  = 101
    direction = "Inbound"
    access    = "Allow"
    protocol  = "Tcp"

    source_address_prefixes    = var.allowlist_ip
    source_port_range          = "*"
    destination_port_range     = 4646
    destination_address_prefix = "*"
  }

  # Consul
  security_rule {
    name = "primary-sgr-8500"

    priority  = 102
    direction = "Inbound"
    access    = "Allow"
    protocol  = "Tcp"

    source_address_prefixes    = var.allowlist_ip
    source_port_range          = "*"
    destination_port_range     = 8500
    destination_address_prefix = "*"
  }
}

resource "azurerm_network_security_group" "nomad_clients" {
  name                = "nomad-clients-sg"
  location            = azurerm_resource_group.hashistack.location
  resource_group_name = azurerm_resource_group.hashistack.name

  # SSH
  security_rule {
    name = "primary-sgr-22"

    priority  = 100
    direction = "Inbound"
    access    = "Allow"
    protocol  = "Tcp"

    source_address_prefixes    = var.allowlist_ip
    source_port_range          = "*"
    destination_port_range     = 22
    destination_address_prefix = "*"
  }

  # Nomad
  security_rule {
    name = "primary-sgr-4646"

    priority  = 101
    direction = "Inbound"
    access    = "Allow"
    protocol  = "Tcp"

    source_address_prefixes    = var.allowlist_ip
    source_port_range          = "*"
    destination_port_range     = 4646
    destination_address_prefix = "*"
  }

  # Consul
  security_rule {
    name = "primary-sgr-8500"

    priority  = 102
    direction = "Inbound"
    access    = "Allow"
    protocol  = "Tcp"

    source_address_prefixes    = var.allowlist_ip
    source_port_range          = "*"
    destination_port_range     = 8500
    destination_address_prefix = "*"
  }

  # HTTP
  security_rule {
    name = "primary-sgr-80"

    priority  = 103
    direction = "Inbound"
    access    = "Allow"
    protocol  = "Tcp"

    source_address_prefixes    = var.allowlist_ip
    source_port_range          = "*"
    destination_port_range     = 80
    destination_address_prefix = "*"
  }

  # Grafana metrics dashboard.
  security_rule {
    name = "primary-sgr-3000"

    priority  = 104
    direction = "Inbound"
    access    = "Allow"
    protocol  = "Tcp"

    source_address_prefixes    = var.allowlist_ip
    source_port_range          = "*"
    destination_port_range     = 3000
    destination_address_prefix = "*"
  }

  # Prometheus dashboard.
  security_rule {
    name = "primary-sgr-9090"

    priority  = 105
    direction = "Inbound"
    access    = "Allow"
    protocol  = "Tcp"

    source_address_prefixes    = var.allowlist_ip
    source_port_range          = "*"
    destination_port_range     = 9090
    destination_address_prefix = "*"
  }

  # Traefik router.
  security_rule {
    name = "primary-sgr-8081"

    priority  = 106
    direction = "Inbound"
    access    = "Allow"
    protocol  = "Tcp"

    source_address_prefixes    = var.allowlist_ip
    source_port_range          = "*"
    destination_port_range     = 8081
    destination_address_prefix = "*"
  }

  # Nomad dynamic port allocation range.
  security_rule {
    name = "primary-sgr-nomad-alloc-range"

    priority  = 107
    direction = "Inbound"
    access    = "Allow"
    protocol  = "Tcp"

    source_address_prefixes    = var.allowlist_ip
    source_port_range          = "*"
    destination_port_range     = "20000-32000"
    destination_address_prefix = "*"
  }
}
