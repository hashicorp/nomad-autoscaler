plugin_dir = "./plugins"

scan_interval = "5s"

nomad {
  address = "127.0.0.1:4646"
}

apm "prometheus" {
  driver = "prometheus"

  config = {
    address = "http://127.0.0.1:9090"
  }
}

strategy "target-value" {
  driver = "target-value"
}
