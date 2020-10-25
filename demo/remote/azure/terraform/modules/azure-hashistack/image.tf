resource "null_resource" "packer_build" {
  depends_on = [azurerm_resource_group.hashistack]

  provisioner "local-exec" {
    command = <<EOF
cd ../../packer && \
  packer build -force \
    -var "client_id=$ARM_CLIENT_ID" \
    -var "client_secret=$ARM_CLIENT_SECRET" \
    -var "resource_group=${azurerm_resource_group.hashistack.name}" \
    -var "subscription_id=$ARM_SUBSCRIPTION_ID" \
    -var "location=${azurerm_resource_group.hashistack.location}" \
    -var "image_name=hashistack" \
    azure-packer.pkr.hcl
EOF
  }
}

data "azurerm_image" "hashistack" {
  depends_on = [null_resource.packer_build]

  name                = "hashistack"
  resource_group_name = azurerm_resource_group.hashistack.name
}
