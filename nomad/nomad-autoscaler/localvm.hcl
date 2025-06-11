

env_vars = [
  {
    key = "ENV_SUBDOMAIN"
    value = "localdomain"
  }
]

extra_templates = [
  {
    data        = <<EOF
nomad {
  address = "http://localhost:4646"
}

telemetry {
  prometheus_metrics = true
  disable_hostname   = true
}

apm "prometheus" {
  driver = "prometheus"
  config = {
    address = "http://localhost:18400/select/0/prometheus"
  }
}

apm "nomad-meta-apm" {
  driver = "nomad-meta-apm"
  config = {
    nomad_address = "http://localhost:4646"
    page_size = "50"
  }
}

apm "nomad-meta-apm-v2" {
  driver = "nomad-meta-apm-v2"
  config = {
    nomad_address = "http://localhost:4646"
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
    nomad_address = "http://localhost:4646"
  }
}

EOF
    destination = "local/config.hcl"
  }
]
