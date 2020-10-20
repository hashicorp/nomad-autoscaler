variable "created_email" {}
variable "created_name" {}
variable "region" { default = "us-east-1" }

source "amazon-ebs" "hashistack" {
  ami_name      = "Hashistack {{timestamp}}"
  region        = var.region
  instance_type = "t2.medium"

  source_ami_filter {
    filters = {
      virtualization-type = "hvm"
      name                = "ubuntu/images/*ubuntu-focal-20.04-amd64-server-*"
      root-device-type    = "ebs"
    }
    owners      = ["099720109477"] # Canonical's owner ID
    most_recent = true
  }

  communicator = "ssh"
  ssh_username = "ubuntu"

  tags = {
    OS_Version    = "Ubuntu"
    Release       = "20.04"
    Architecture  = "amd64"
    Created_Email = var.created_email
    Created_Name  = var.created_name
  }
}

build {
  sources = [
    "source.amazon-ebs.hashistack"
  ]

  provisioner "shell" {
    inline = [
      "sudo mkdir -p /ops",
      "sudo chmod 777 /ops"
    ]
  }

  provisioner "file" {
    source      = "./"
    destination = "/ops"
  }

  provisioner "shell" {
    script = "./scripts/setup.sh"
  }
}
