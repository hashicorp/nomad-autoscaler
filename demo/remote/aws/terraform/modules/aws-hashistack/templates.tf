data "template_file" "user_data_server" {
  template = file("${path.module}/templates/user-data-server.sh")

  vars = {
    server_count  = var.server_count
    region        = var.region
    retry_join    = var.retry_join
    consul_binary = var.consul_binary
    nomad_binary  = var.nomad_binary
  }
}

data "template_file" "user_data_client" {
  template = file("${path.module}/templates/user-data-client.sh")

  vars = {
    region        = var.region
    retry_join    = var.retry_join
    consul_binary = var.consul_binary
    nomad_binary  = var.nomad_binary
    node_class    = "hashistack"
  }
}

data "template_file" "nomad_autoscaler_jobspec" {
  template = file("${path.module}/templates/aws_autoscaler.nomad")

  vars = {
    nomad_autoscaler_image = var.nomad_autoscaler_image
    client_asg_name        = aws_autoscaling_group.nomad_client.name
  }
}

resource "null_resource" "nomad_autoscaler_jobspec" {
  provisioner "local-exec" {
    command = "echo '${data.template_file.nomad_autoscaler_jobspec.rendered}' > aws_autoscaler.nomad"
  }
}
