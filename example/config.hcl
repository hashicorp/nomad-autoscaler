plugin_dir = "./plugins"

scan_interval = "5s"

nomad {
  address = "127.0.0.1:4646"
}

apm "prometheus" {
  driver = "prometheus"

  config = {
    #    "address" = "http://127.0.0.1:9090"
    "address" = "http://192.168.50.66:9090"
  }
}

target "x1-nomad" {
  driver = "nomad"

  config = {
    address = "192.168.50.66:4646"
  }
}

strategy "target-value" {
  driver = "target-value"
}
