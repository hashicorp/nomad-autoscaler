job "autoscaler" {
  datacenters = ["dc1"]

  group "autoscaler" {
    count = 1

    volume "plugins" {
      type      = "host"
      source    = "plugins"
      read_only = true
    }

    task "autoscaler" {
      driver = "exec"

      template {
        data = <<EOF
plugin_dir = "/plugins"
scan_interval = "5s"
nomad {
  address = "{{env "attr.unique.network.ip-address" }}:4646"
}
apm "nomad" {
  driver = "nomad-apm"
  config  = {
    address = "{{env "attr.unique.network.ip-address" }}:4646"
  }
}
apm "prometheus" {
  driver = "prometheus"
  config = {
    address = "http://{{ env "attr.unique.network.ip-address" }}:9090"
  }
}
strategy "target-value" {
  driver = "target-value"
}
          EOF
        destination = "/autoscaler/config.hcl"
      }

      volume_mount {
        volume      = "plugins"
        destination = "/plugins"
      }

      config {
        command = "/usr/local/bin/nomad-autoscaler"
        args    = ["agent", "-config", "/autoscaler/config.hcl"]
      }

      resources {
        cpu    = 50
        memory = 128

        network {
          mbits = 10
        }
      }
    }
  }
}
