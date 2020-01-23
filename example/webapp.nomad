job "demo-webapp" {
  datacenters = ["dc1"]

  group "demo" {
    count = 1

    task "server" {
      env {
        PORT    = "${NOMAD_PORT_http}"
        NODE_IP = "${NOMAD_IP_http}"
      }

      driver = "docker"

      config {
        image          = "hashicorp/demo-webapp-lb-guide"
        cpu_hard_limit = true
      }

      resources {
        cpu = 50

        network {
          mbits = 10
          port  "http"{}
        }
      }

      service {
        name = "demo-webapp"
        port = "http"

        check {
          type     = "http"
          path     = "/"
          interval = "2s"
          timeout  = "2s"
        }
      }
    }
  }
}
