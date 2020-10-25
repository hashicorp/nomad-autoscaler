variable "location" {
  description = "The Azure location to deploy to."
  type        = string
  default     = "East US"
}

variable "server_vm_size" {
  description = "The Azure VM size to use for servers."
  type        = string
  default     = "Standard_DS1_v2"
}

variable "server_count" {
  description = "The number of servers to provision."
  type        = number
  default     = 1
}

variable "client_vm_size" {
  description = "The Azure VM size to use for clients."
  type        = string
  default     = "Standard_DS1_v2"
}

variable "client_count" {
  description = "The number of clients to provision."
  type        = number
  default     = 1
}

variable "nomad_binary" {
  description = "The URL to download a custom Nomad binary if desired."
  type        = string
  default     = "none"
}

variable "consul_binary" {
  description = "The URL to download a custom Consul binary if desired."
  type        = string
  default     = "none"
}

variable "allowlist_ip" {
  description = "A list of IP address to grant access via the LBs."
  type        = list(string)
  default     = ["0.0.0.0/0"]
}
