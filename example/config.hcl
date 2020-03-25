plugin_dir = "./bin/plugins"

apm "prometheus" {
  driver = "prometheus"

  config = {
    address = "http://127.0.0.1:9090"
  }
}

strategy "target-value" {
  driver = "target-value"
}
