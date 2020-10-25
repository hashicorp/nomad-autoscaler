data "azurerm_subscription" "main" {}

data "template_file" "user_data_server" {
  template = file("${path.module}/templates/user-data-server.sh")

  vars = {
    server_count  = var.server_count
    retry_join    = "provider=azure tag_name=ConsulAutoJoin tag_value=auto-join subscription_id=${data.azurerm_subscription.main.subscription_id}"
    consul_binary = var.consul_binary
    nomad_binary  = var.nomad_binary
  }
}

data "template_file" "user_data_client" {
  template = file("${path.module}/templates/user-data-client.sh")

  vars = {
    retry_join    = "provider=azure tag_name=ConsulAutoJoin tag_value=auto-join subscription_id=${data.azurerm_subscription.main.subscription_id}"
    consul_binary = var.consul_binary
    nomad_binary  = var.nomad_binary
    node_class    = "hashistack"
  }
}

data "template_file" "nomad_autoscaler_jobspec" {
  template = file("${path.module}/templates/azure_autoscaler.nomad")

  vars = {
    subscription_id = data.azurerm_subscription.main.subscription_id
    resource_group  = azurerm_resource_group.hashistack.name
  }
}

resource "null_resource" "nomad_autoscaler_jobspec" {
  provisioner "local-exec" {
    command = "echo '${data.template_file.nomad_autoscaler_jobspec.rendered}' > azure_autoscaler.nomad"
  }
}
