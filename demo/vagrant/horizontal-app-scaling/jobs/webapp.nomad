job "webapp" {
  datacenters = ["dc1"]

  group "demo" {
    count = 3

    network {

      port "api"{
        to = 8474
      }
      port "webapp"{}
      port  "http"{}
    }

    scaling {
      enabled = true
      min     = 1
      max     = 20

      policy {

        cooldown = "20s"

        check "avg_instance_sessions" {
          source   = "prometheus"
          query    = "scalar(avg((haproxy_server_current_sessions{backend=\"http_back\"}) and (haproxy_server_up{backend=\"http_back\"} == 1)))"


          strategy "target-value" {
            target = 10
          }
        }
      }
    }

    task "webapp" {
      driver = "docker"

      config {
        image = "hashicorp/demo-webapp-lb-guide"
        network_mode = "host"

      }

      env {
        PORT    = "${NOMAD_PORT_http}"
        NODE_IP = "${NOMAD_IP_http}"
      }

      resources {
        cpu    = 100
        memory = 16
      }

      service {
        name = "demo-webapp"
        port = "http"
        check {
          type     = "http"
          path     = "/"
          interval = "3s"
          timeout  = "1s"
        }
      }
    }

    task "toxiproxy" {
      driver = "docker"

      lifecycle {
        hook    = "prestart"
        sidecar = true
      }

      config {
        image      = "shopify/toxiproxy:2.1.4"
        entrypoint = ["/entrypoint.sh"]
        volumes = [
          "local/entrypoint.sh:/entrypoint.sh",
        ]
        #network_mode = "host"
        ports = ["api", "webapp"]
      }

      template {
        data = <<EOH
#!/bin/sh

set -ex

/go/bin/toxiproxy -host 0.0.0.0  &

while ! wget --spider -q http://localhost:${NOMAD_PORT_api}/version; do
  echo "toxiproxy not ready yet"
  sleep 0.2
done

/go/bin/toxiproxy-cli create webapp -l 0.0.0.0:${NOMAD_PORT_webapp} -u ${NOMAD_ADDR_http}
/go/bin/toxiproxy-cli toxic add -n latency -t latency -a latency=1000 -a jitter=500 webapp
tail -f /dev/null
        EOH

        destination = "local/entrypoint.sh"
        perms       = "755"
      }

      service {
        name = "toxiproxy-api"
        port = "api"
        check {
          type     = "http"
          path     = "/proxies/webapp"
          interval = "3s"
          timeout  = "1s"
        }
      }

      service {
        name = "toxiproxy-webapp"
        port = "webapp"
      }

      resources {
        cpu    = 100
        memory = 32
      }
    }
  }
}
