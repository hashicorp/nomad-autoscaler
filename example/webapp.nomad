job "webapp" {
  datacenters = ["dc1"]

  group "demo" {
    count = 1

    task "server" {
      driver = "docker"

      config {
        image          = "hashicorp/demo-webapp-lb-guide"
        cpu_hard_limit = true

        ulimit {
          nofile = "512:512"
        }
      }

      env {
        PORT    = "${NOMAD_PORT_http}"
        NODE_IP = "${NOMAD_IP_http}"
      }

      resources {
        cpu = 50

        network {
          mbits = 10
          port  "http"{}
        }
      }

      service {
        name = "webapp"
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
