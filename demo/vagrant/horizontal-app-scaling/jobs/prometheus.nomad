job "prometheus" {
  datacenters = ["dc1"]

  group "prometheus" {
    count = 1
    network {
      port "prometheus_ui" {
        to = 9090
        static = 9090
      }
    }
    task "prometheus" {
      driver = "docker"

      config {
        image = "prom/prometheus:v2.18.1"

        args = [
          "--config.file=/etc/prometheus/config/prometheus.yml",
          "--storage.tsdb.path=/prometheus",
          "--web.console.libraries=/usr/share/prometheus/console_libraries",
          "--web.console.templates=/usr/share/prometheus/consoles",
        ]

        network_mode = "host"

        volumes = [
          "local/config:/etc/prometheus/config",
        ]
        ports = ["prometheus_ui"]
      }

      template {
        data = <<EOH
---
global:
  scrape_interval:     1s
  evaluation_interval: 1s

scrape_configs:
  - job_name: haproxy_exporter
    static_configs:
      - targets: [{{ range service "haproxy-exporter" }}'{{ .Address }}:{{ .Port }}',{{ end }}]

  - job_name: nomad_autoscaler
    metrics_path: /v1/metrics
    params:
      format: ['prometheus']
    static_configs:
      - targets: [{{ range service "autoscaler" }}'{{ .Address }}:{{ .Port }}',{{ end }}]

  - job_name: consul
    metrics_path: /v1/agent/metrics
    params:
      format: ['prometheus']
    static_configs:
    - targets: ['{{ env "attr.unique.network.ip-address" }}:8500']

  - job_name: nomad
    metrics_path: /v1/metrics
    params:
      format: ['prometheus']
    static_configs:
    - targets: ['{{ env "attr.unique.network.ip-address" }}:4646']
EOH

        change_mode   = "signal"
        change_signal = "SIGHUP"
        destination   = "local/config/prometheus.yml"
      }

      resources {
        cpu    = 100
        memory = 256
      }

      service {
        name = "prometheus"
        port = "prometheus_ui"

        check {
          type     = "http"
          path     = "/-/healthy"
          interval = "10s"
          timeout  = "2s"
        }
      }
    }
  }
}
