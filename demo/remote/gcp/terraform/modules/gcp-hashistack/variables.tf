variable "project_id" {
  description = "The project ID where resources will be created."
  type        = string
}

variable "stack_name" {
  type    = string
  default = "hashistack"
}

variable "hashistack_image_project_id" {
  description = "The project ID to look for an existing image."
  type        = string
  default     = ""
}

variable "hashistack_image_name" {
  description = "The name of the image to use in the compute instances."
  type        = string
  default     = "hashistack"
}

variable "build_hashistack_image" {
  description = "If set to true, Packer will be invoked to build the image."
  type        = bool
  default     = true
}

variable "region" {
  description = "The region where resources will be created."
  type        = string
  default     = "us-central1"
}

variable "zone" {
  description = "The zone where resources will be created."
  type        = string
  default     = "a"
}

variable "ip_cidr_range" {
  description = "An internal IP address range to provision."
  type        = string
  default     = "10.0.0.0/24"
}

variable "vm_disk_size" {
  description = "The GB size to assign to instances."
  type        = number
  default     = 20
}

variable "server_count" {
  description = "The number of servers to provision."
  type        = number
  default     = 1
}

variable "server_machine_type" {
  description = "The VM machine type to provision for Nomad servers."
  type        = string
  default     = "e2-small"
}

variable "client_machine_type" {
  description = "The VM machine type to provision for Nomad clients."
  type        = string
  default     = "e2-small"
}

variable "client_mig_type" {
  description = "The type of Managed Instance Group to provision. Possible values are `regional` or `zonal`."
  type        = string
  default     = "regional"
}

variable "allowlist_ip" {
  description = "A list of IP addresses to grant access via the LBs."
  type        = list(string)
  default     = ["0.0.0.0/0"]
}

variable "nomad_autoscaler_image" {
  description = "The Docker image to use for the Nomad Autoscaler job."
  type        = string
  default     = "hashicorp/nomad-autoscaler:0.3.0"
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
