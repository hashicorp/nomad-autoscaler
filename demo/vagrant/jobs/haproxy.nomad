job "haproxy" {
  datacenters = ["dc1"]

  group "haproxy" {
    count = 1

    task "haproxy" {
      driver = "docker"

      config {
        image        = "haproxy:2.0"
        network_mode = "host"

        volumes = [
          "local/haproxy.cfg:/usr/local/etc/haproxy/haproxy.cfg",
        ]
      }

      template {
        data = <<EOF
defaults
   mode http

frontend stats
   bind *:1936
   stats uri /
   stats show-legends
   no log

frontend http_front
   bind *:8000
   default_backend http_back

backend http_back
    balance roundrobin
    server-template mywebapp 10 _webapp._tcp.service.consul resolvers consul resolve-opts allow-dup-ip resolve-prefer ipv4 check

resolvers consul
  nameserver consul 127.0.0.1:8600
  accepted_payload_size 8192
  hold valid 5s
EOF

        destination = "local/haproxy.cfg"
        change_mode = "restart"
      }

      service {
        name = "haproxy"
        port = "haproxy_ui"

        check {
          name     = "haproxy alive"
          type     = "http"
          path     = "/"
          interval = "10s"
          timeout  = "2s"
        }
      }

      service {
        name = "webapp-haproxy"
        port = "webapp"
      }

      resources {
        cpu    = 200
        memory = 512

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

      config {
        image = "prom/haproxy-exporter"

        args = ["--haproxy.scrape-uri", "http://${NOMAD_ADDR_haproxy_haproxy_ui}/?stats;csv"]

        port_map {
          http = 9101
        }
      }

      service {
        name = "haproxy-exporter"
        port = "http"

        check {
          name     = "haproxy_exporter port alive"
          type     = "http"
          path     = "/metrics"
          interval = "10s"
          timeout  = "2s"
        }
      }

      resources {
        cpu    = 200
        memory = 128

        network {
          mbits = 10

          port "http" {}
        }
      }
    }
  }
}
