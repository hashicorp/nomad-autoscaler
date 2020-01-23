plugin_dir = "./plugins"

apm "prometheus" {
  driver = "prometheus"

  config = {
    "address" = "http://192.168.50.66:9090"
  }
}
