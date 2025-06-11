job_name     = "nomad-autoscaler"
docker_image = "kentik-nomad-autoscaler"

resources = {
  cpu    = 500
  memory = 256
}

network = {
  mode = "host"
  ports = {
    "http" = {
      check_type      = "http"
      check_path      = "/v1/health"
      metrics         = true
      metrics_path    = "/v1/metrics?format=prometheus"
      metrics_version = "2"
    }
  }
}

args = [
  "/usr/local/bin/nomad-autoscaler",
  "agent",
  "-plugin-dir=/opt/autoscaler-plugins",
  "-config",
  "/local/config.hcl",
  "-http-bind-address",
  "0.0.0.0",
  "-http-bind-port",
  "$${NOMAD_PORT_http}",
]

constraints = [
  {
    attribute = "$${node.class}"
    operator  = "set_contains_any"
    value     = "compute,storage"
  }
]


extra_templates = [
  {
    data        = <<EOF
nomad {
  address = "https://nomad.{{ env "domain_prefix" }}.kentik.com"
}

telemetry {
  prometheus_metrics = true
  disable_hostname   = true
}

apm "prometheus" {
  driver = "prometheus"
  config = {
    address = "http://victoriametrics.{{ env "domain_prefix" }}.kentik.com:18400/select/0/prometheus"
  }
}

apm "nomad-meta-apm-v2" {
  driver = "nomad-meta-apm-v2"
  config = {
    nomad_address = "https://nomad.{{ env "domain_prefix" }}.kentik.com"
    page_size = "50"
  }
}

apm "static-count-apm" {
  driver = "static-count-apm"
}

strategy "target-value" {
  driver = "target-value"
}

strategy "pass-through" {
  driver = "pass-through"
}

target "kentik-nomad-target" {
  driver = "kentik-nomad-target"
  config = {
    nomad_address = "https://nomad.{{ env "domain_prefix" }}.kentik.com"
  }
}

EOF
    destination = "local/config.hcl"
  }
]
