# data_dir = "/Users/laoqui/go/src/github.com/hashicorp/nomad-autoscaler/example/data"
data_dir = "/home/laoqui/Projects/nomad-autoscale/mbp/nomad-autoscaler/example/data"

bind_addr = "0.0.0.0" # the default

server {
  enabled          = true
  bootstrap_expect = 1
}

client {
  enabled = true
}

plugin "raw_exec" {
  config {
    enabled = true
  }
}

consul {
  address = "127.0.0.1:8500"
}

telemetry {
  collection_interval        = "1s"
  disable_hostname           = true
  prometheus_metrics         = true
  publish_allocation_metrics = true
  publish_node_metrics       = true
}
