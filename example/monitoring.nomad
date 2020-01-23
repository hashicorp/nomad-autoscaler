job "monitoring" {
  datacenters = ["dc1"]
  type        = "service"

  group "prometheus" {
    count = 1

    restart {
      attempts = 2
      interval = "30m"
      delay    = "15s"
      mode     = "fail"
    }

    ephemeral_disk {
      size = 300
    }

    task "prometheus" {
      template {
        change_mode   = "signal"
        change_signal = "SIGHUP"
        destination   = "local/prometheus.yml"

        data = <<EOH
---
global:
  scrape_interval:     1s
  evaluation_interval: 1s

scrape_configs:
  - job_name: redis_exporter
    static_configs:
    - targets: [{{ range service "redis-exporter" }}'{{ .Address }}:{{ .Port }}',{{ end }}]

  - job_name: haproxy_exporter
    static_configs:
    - targets: [{{ range service "haproxy-exporter" }}'{{ .Address }}:{{ .Port }}',{{ end }}]

#  - job_name: consul
#    metrics_path: /v1/agent/metrics
#    params:
#      format: ['prometheus']
#    static_configs:
#    - targets: ['127.0.0.1:8500']

  - job_name: 'nomad_metrics'
    consul_sd_configs:
    - server: '127.0.0.1:8500'
      #services: ['nomad-client', 'nomad']
#    - server: 'docker.for.mac.localhost:8500'
#      services: ['nomad-client', 'nomad']

    relabel_configs:
    - source_labels: ['__meta_consul_tags']
      regex: '(.*)http(.*)'
      action: keep

    scrape_interval: 5s
    metrics_path: /v1/metrics
    params:
      format: ['prometheus']
EOH
      }

      driver = "docker"

      config {
        image        = "prom/prometheus:latest"
        network_mode = "host"

        volumes = [
          "local/prometheus.yml:/etc/prometheus/prometheus.yml",
        ]

        port_map {
          prometheus_ui = 9090
        }
      }

      resources {
        network {
          mbits = 10

          port "prometheus_ui" {
            static = 9090
          }
        }
      }

      service {
        name = "prometheus"
        tags = ["urlprefix-/"]
        port = "prometheus_ui"

        check {
          name     = "prometheus_ui port alive"
          type     = "http"
          path     = "/-/healthy"
          interval = "10s"
          timeout  = "2s"
        }
      }
    }
  }

  group "graphana" {
    count = 1

    restart {
      attempts = 2
      interval = "30m"
      delay    = "15s"
      mode     = "fail"
    }

    task "graphana" {
      driver = "docker"

      config {
        image = "grafana/grafana"

        volumes = [
          "local/graphana:/var/lib/grafana grafana/grafana",
        ]

        port_map {
          graphana_ui = 3000
        }
      }

      resources {
        network {
          mbits = 10

          port "graphana_ui" {
            static = 3000
          }
        }
      }
    }
  }
}
