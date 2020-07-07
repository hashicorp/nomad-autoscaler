enabled = true
min     = 1
max     = 10

policy {

  cooldown            = "1m"
  evaluation_interval = "30s"

  check "cpu_nomad" {
    source    = "nomad_apm"
    query     = "avg_cpu"

    strategy "target-value" {
      target = "80"
    }
  }

  check "memory_nomad" {
    source    = "nomad_apm"
    query     = "avg_memory"

    strategy "target-value" {
      target = "80"
    }
  }

  target "nomad" {
    Group = "cache"
    Job   = "example"
  }
}
