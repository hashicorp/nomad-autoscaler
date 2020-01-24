datacenter = "dc1"

data_dir = "/opt/nomad"

server {
  enabled          = true
  bootstrap_expect = 1
}

client {
  enabled = true
}

telemetry {
  publish_allocation_metrics = true
  publish_node_metrics       = true
  prometheus_metrics         = true
}
