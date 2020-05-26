job "haproxy" {
  datacenters = ["dc1"]

  group "haproxy" {
    count = 1

    task "haproxy" {
      driver = "docker"

      config {
        image        = "haproxy:2.1.4"
        network_mode = "host"

        volumes = [
          "local/haproxy.cfg:/usr/local/etc/haproxy/haproxy.cfg",
        ]
      }

      template {
        data = <<EOF
global
   maxconn 256

defaults
   mode http

frontend stats
   bind *:{{ env "NOMAD_PORT_haproxy_ui" }}
   stats uri /
   stats show-legends
   no log

frontend http_front
   bind *:{{ env "NOMAD_PORT_webapp" }}
   default_backend http_back

backend http_back
    balance roundrobin
    server-template mywebapp 20 _toxiproxy-webapp._tcp.service.consul resolvers consul resolve-opts allow-dup-ip resolve-prefer ipv4 check

resolvers consul
  nameserver consul {{ env "attr.unique.network.ip-address" }}:8600
  accepted_payload_size 8192
  hold valid 5s
EOF

        destination   = "local/haproxy.cfg"
        change_mode   = "signal"
        change_signal = "SIGUSR1"
      }

      service {
        name = "haproxy-ui"
        port = "haproxy_ui"

        check {
          type     = "http"
          path     = "/"
          interval = "10s"
          timeout  = "2s"
        }
      }

      service {
        name = "haproxy-webapp"
        port = "webapp"
      }

      resources {
        cpu    = 500
        memory = 128

        network {
          mbits = 10

          port "webapp" {
            static = 8000
          }

          port "haproxy_ui" {
            static = 1936
          }
        }
      }
    }

    task "haproxy_prometheus" {
      driver = "docker"

      lifecycle {
        hook    = "prestart"
        sidecar = true
      }

      config {
        image = "prom/haproxy-exporter:v0.10.0"

        args = ["--haproxy.scrape-uri", "http://${NOMAD_ADDR_haproxy_haproxy_ui}/?stats;csv"]

        port_map {
          http = 9101
        }
      }

      service {
        name = "haproxy-exporter"
        port = "http"

        check {
          type     = "http"
          path     = "/metrics"
          interval = "10s"
          timeout  = "2s"
        }
      }

      resources {
        cpu    = 100
        memory = 32

        network {
          mbits = 10

          port "http" {}
        }
      }
    }
  }
}
