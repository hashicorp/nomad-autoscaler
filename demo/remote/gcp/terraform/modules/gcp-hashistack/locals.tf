locals {
  zone_id           = format("%s-%s", var.region, var.zone)
  stack_name        = var.stack_name
  client_stack_name = "${var.stack_name}-nomad-client"
  server_stack_name = "${var.stack_name}-nomad-server"
}
