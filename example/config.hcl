plugin_dir = "./plugins"

scan_interval = "5s"

apm "prometheus" {
  driver = "prometheus"

  config = {
    #    "address" = "http://127.0.0.1:9090"
    "address" = "http://192.168.50.66:9090"
  }
}
