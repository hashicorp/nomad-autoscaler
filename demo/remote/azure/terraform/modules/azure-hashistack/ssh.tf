resource "tls_private_key" "main" {
  algorithm = "RSA"
}

resource "null_resource" "main" {
  provisioner "local-exec" {
    command = "echo \"${tls_private_key.main.private_key_pem}\" > azure-hashistack.pem"
  }

  provisioner "local-exec" {
    command = "chmod 600 azure-hashistack.pem"
  }
}
