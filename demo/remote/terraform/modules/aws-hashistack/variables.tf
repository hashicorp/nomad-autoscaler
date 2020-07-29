variable "stack_name" {
  description = "The name to prefix onto resources."
  type        = string
  default     = "hashistack"
}

variable "owner_name" {
  description = "Your name so resources can be easily assigned."
  type        = string
}

variable "owner_email" {
  description = "Your email so you can be contacted about resources."
  type        = string
}

variable "region" {
  description = "The AWS region to deploy into."
  type        = string
  default     = "us-east-1"
}

variable "availability_zones" {
  description = "The AWS region AZs to deploy into."
  type        = list(string)
  default     = ["us-east-1a"]
}

variable "vpc_id" {
  description = "The AWS VPC to use for resources. If left empty, the default will be used."
  type        = string
  default     = ""
}

variable "ami" {
  description = "The AMI to use, preferably built by the supplied Packer scripts."
  type        = string
}

variable "key_name" {
  description = "The EC2 key pair to use for EC2 instance SSH access."
  type        = string
}

variable "server_instance_type" {
  description = "The EC2 instance type to launch for Nomad servers."
  type        = string
  default     = "t3.small"
}

variable "server_count" {
  description = "The number of Nomad servers to run."
  type        = number
  default     = 1
}

variable "client_instance_type" {
  description = "The EC2 instance type to launch for Nomad clients."
  type        = string
  default     = "t3.small"
}
variable "client_count" {
  description = "The number of Nomad clients to run."
  type        = number
  default     = 1
}

variable "root_block_device_size" {
  description = "The number of GB to assign as a block device on instances."
  type        = number
  default     = 16
}

variable "retry_join" {
  description = "The retry join configuration to use."
  type        = string
  default     = "provider=aws tag_key=ConsulAutoJoin tag_value=auto-join"
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
