job "das-load-test" {
  datacenters = ["dc1"]
  type        = "batch"

  parameterized {
    payload       = "optional"
    meta_optional = ["requests", "clients"]
  }

  group "redis-benchmark" {
    task "redis-benchmark" {
      driver = "docker"

      config {
        image   = "redis:6.0"
        command = "redis-benchmark"

        args = [
          "-h",
          "${HOST}",
          "-p",
          "${PORT}",
          "-n",
          "${REQUESTS}",
          "-c",
          "${CLIENTS}",
        ]
      }

      template {
        destination = "secrets/env.txt"
        env         = true

        data = <<EOF
{{ with service "redis-lb" }}{{ with index . 0 -}}
HOST={{.Address}}
PORT={{.Port}}
{{- end }}{{ end }}
REQUESTS={{ or (env "NOMAD_META_requests") "100000" }}
CLIENTS={{  or (env "NOMAD_META_clients") "50" }}
EOF
      }

      resources {
        cpu    = 100
        memory = 128
      }
    }
  }
}
