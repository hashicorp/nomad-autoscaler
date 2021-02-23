output "stack_detail" {
  value = <<CONFIGURATION

You can set the gcloud project setting for CLI use with `gcloud config set project
${module.hashistack_cluster.google_project_id}`, otherwise you will need to set the `--project`
flag on each command.

To connect to any instance running within the environment you can use the
`gcloud compute ssh ubuntu@<instance_name>` command within your terminal or use the UI.

You can test the integrity of the cluster by running:

  $ consul members
  $ nomad server members
  $ nomad node status

The Nomad UI can be accessed at ${module.hashistack_cluster.nomad_addr}/ui
The Consul UI can be accessed at ${module.hashistack_cluster.consul_addr}/ui
Grafana dashbaord can be accessed at http://${module.hashistack_cluster.client_public_ip_addr}:3000/d/AQphTqmMk/demo?orgId=1&refresh=5s
Traefik can be accessed at http://${module.hashistack_cluster.client_public_ip_addr}:8081
Prometheus can be accessed at http://${module.hashistack_cluster.client_public_ip_addr}:9090
Webapp can be accessed at http://${module.hashistack_cluster.client_public_ip_addr}:80

CLI environment variables:
export NOMAD_CLIENT_DNS=http://${module.hashistack_cluster.client_public_ip_addr}
export NOMAD_ADDR=${module.hashistack_cluster.nomad_addr}

CONFIGURATION
}
