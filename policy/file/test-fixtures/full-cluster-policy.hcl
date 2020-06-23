enabled = true
min     = 10
max     = 100

policy {

  cooldown            = "10m"
  evaluation_interval = "1m"

  check "cpu_nomad" {
    source    = "nomad_apm"
    query     = "cpu_high-memory"

    strategy "target-value" {
      target = "80"
    }
  }

  check "memory_prom" {
    source    = "prometheus"
    query     = "nomad_client_allocated_memory*100/(nomad_client_allocated_memory+nomad_client_unallocated_memory)"

    strategy "target-value" {
      target = "80"
    }
  }

  target "aws-asg" {
      asg_name       = "my-target-asg"
      class          = "high-memory"
      drain_deadline = "15m"
  }
}
