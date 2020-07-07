resource "null_resource" "wait_for_nomad_api" {
  provisioner "local-exec" {
    command = "while ! nomad server members > /dev/null 2>&1; do echo 'waiting for nomad api...'; sleep 10; done"
    environment = {
      NOMAD_ADDR = var.nomad_addr
    }
  }
}

resource "nomad_job" "traefik" {
  depends_on = [null_resource.wait_for_nomad_api]
  jobspec    = file("${path.module}/files/traefik.nomad")
}

resource "nomad_job" "prometheus" {
  depends_on = [null_resource.wait_for_nomad_api]
  jobspec    = file("${path.module}/files/prometheus.nomad")
}

resource "nomad_job" "grafana" {
  depends_on = [null_resource.wait_for_nomad_api]
  jobspec    = file("${path.module}/files/grafana.nomad")
}

resource "nomad_job" "webapp" {
  depends_on = [null_resource.wait_for_nomad_api]
  jobspec    = file("${path.module}/files/webapp.nomad")
}
