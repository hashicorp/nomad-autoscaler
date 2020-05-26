job "autoscaler" {
  datacenters = ["dc1"]

  group "autoscaler" {
    count = 1

    task "autoscaler" {
      driver = "docker"

      config {
        image   = "hashicorp/nomad-autoscaler:0.0.2"
        command = "nomad-autoscaler"

        args = [
          "agent",
          "-config",
          "${NOMAD_TASK_DIR}/config.hcl",
          "-http-bind-address",
          "0.0.0.0",
        ]

        port_map {
          http = 8080
        }
      }

      ## Alternatively, you could also run the Autoscaler using the exec driver
      # driver = "exec"
      #
      # config {
      #   command = "/usr/local/bin/nomad-autoscaler"
      #   args    = ["agent", "-config", "${NOMAD_TASK_DIR}/config.hcl"]
      # }
      #
      # artifact {
      #   source      = "https://releases.hashicorp.com/nomad-autoscaler/0.0.2/nomad-autoscaler_0.0.2_linux_amd64.zip"
      #   destination = "/usr/local/bin"
      # }

      template {
        data = <<EOF
nomad {
  address = "http://{{env "attr.unique.network.ip-address" }}:4646"
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

        destination = "${NOMAD_TASK_DIR}/config.hcl"
      }

      resources {
        cpu    = 50
        memory = 128

        network {
          mbits = 10
          port "http" {}
        }
      }

      service {
        name = "autoscaler"
        port = "http"

        check {
          type     = "http"
          path     = "/v1/health"
          interval = "5s"
          timeout  = "2s"
        }
      }
    }
  }
}
