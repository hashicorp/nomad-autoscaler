job "autoscaler" {
  datacenters = ["dc1"]

  group "autoscaler" {
    count = 1

    task "autoscaler" {
      driver = "docker"

      config {
        image   = "hashicorp/nomad-autoscaler:0.1.0"
        command = "nomad-autoscaler"

        args = [
          "agent",
          "-config",
          "${NOMAD_TASK_DIR}/config.hcl",
          "-http-bind-address",
          "0.0.0.0",
          "-log-level",
          "debug",
          "-policy-dir",
          "${NOMAD_TASK_DIR}/policies/",
        ]

        port_map {
          http = 8080
        }
      }

      template {
        data = <<EOF
nomad {
  address = "http://{{env "attr.unique.network.ip-address" }}:4646"
}

apm "prometheus" {
  driver = "prometheus"
  config = {
    address = "http://{{ range service "prometheus" }}{{ .Address }}:{{ .Port }}{{ end }}"
  }
}

# datadog example
# apm "datadog" {
#   driver = "datadog"
#   config = {
# {{ with secret "secret/autoscaler/datadog" }}
#     dd_api_key = "{{ .Data.data.api_key }}"
#     dd_app_key = "{{ .Data.data.app_key }}"
# {{ end }}
#   }
# }

target "aws-asg" {
  driver = "aws-asg"
  config = {
    aws_region = "{{ $x := env "attr.platform.aws.placement.availability-zone" }}{{ $length := len $x |subtract 1 }}{{ slice $x 0 $length}}"
  }
}

strategy "target-value" {
  driver = "target-value"
}
EOF

        destination = "${NOMAD_TASK_DIR}/config.hcl"
      }

      template {
        data = <<EOF
scaling "cluster_policy" {
  enabled = true
  min     = 1
  max     = 2

  policy {
    cooldown            = "2m"
    evaluation_interval = "1m"

    check "cpu_allocated_percentage" {
      source = "prometheus"
      query  = "scalar(sum(nomad_client_allocated_cpu{node_class=\"hashistack\"}*100/(nomad_client_unallocated_cpu{node_class=\"hashistack\"}+nomad_client_allocated_cpu{node_class=\"hashistack\"}))/count(nomad_client_allocated_cpu{node_class=\"hashistack\"}))"

      strategy "target-value" {
        target = 70
      }
    }
    # datadog example
    # check "cpu_allocated_percentage" {
    #   source = "datadog"
    #   query  = "FROM=2m;TO=0m;QUERY=avg:nomad.client.allocated.cpu{*}/(avg:nomad.client.unallocated.cpu{*}+avg:nomad.client.allocated.cpu{*})*100"
    #   strategy "target-value" {
    #     target = 70
    #   }
    # }

    check "mem_allocated_percentage" {
      source = "prometheus"
      query  = "scalar(sum(nomad_client_allocated_memory{node_class=\"hashistack\"}*100/(nomad_client_unallocated_memory{node_class=\"hashistack\"}+nomad_client_allocated_memory{node_class=\"hashistack\"}))/count(nomad_client_allocated_memory{node_class=\"hashistack\"}))"

      strategy "target-value" {
        target = 70
      }
    }
    # datadog example
    # check "memory_allocated_percentage" {
    #   source = "datadog"
    #   query  = "FROM=2m;TO=0m;QUERY=avg:nomad.client.allocated.memory{*}/(avg:nomad.client.unallocated.memory{*}+avg:nomad.client.allocated.memory{*})*100"
    #   strategy "target-value" {
    #     target = 70
    #   }
    # }

    target "aws-asg" {
      dry-run             = "false"
      aws_asg_name        = "hashistack-nomad_client"
      node_class          = "hashistack"
      node_drain_deadline = "5m"
    }
  }
}
EOF

        destination = "${NOMAD_TASK_DIR}/policies/hashistack.hcl"
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
