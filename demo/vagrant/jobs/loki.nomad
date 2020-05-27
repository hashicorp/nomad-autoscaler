job "loki" {
  datacenters = ["dc1"]

  group "loki" {
    count = 1

    task "loki" {
      driver = "docker"

      config {
        image = "grafana/loki:1.5.0"

        args = [
          "--config.file=/etc/loki/config/loki.yml",
        ]

        volumes = [
          "local/config:/etc/loki/config",
        ]

        port_map {
          loki_port = 3100
        }
      }

      template {
        data = <<EOH
---
auth_enabled: false

server:
  http_listen_port: 3100

ingester:
  lifecycler:
    address: 127.0.0.1
    ring:
      kvstore:
        store: inmemory
      replication_factor: 1
    final_sleep: 0s
  chunk_idle_period: 5m
  chunk_retain_period: 30s

schema_config:
  configs:
  - from: 2020-05-15
    store: boltdb
    object_store: filesystem
    schema: v11
    index:
      prefix: index_
      period: 168h

storage_config:
  boltdb:
    directory: /tmp/loki/index

  filesystem:
    directory: /tmp/loki/chunks

limits_config:
  enforce_metric_name: false
  reject_old_samples: true
  reject_old_samples_max_age: 168h
EOH

        change_mode   = "signal"
        change_signal = "SIGHUP"
        destination   = "local/config/loki.yml"
      }

      resources {
        cpu    = 100
        memory = 256

        network {
          mbits = 10

          port "loki_port" {}
        }
      }

      service {
        name = "loki"
        port = "loki_port"

        check {
          type     = "http"
          path     = "/ready"
          interval = "10s"
          timeout  = "2s"
        }
      }
    }
  }
}
