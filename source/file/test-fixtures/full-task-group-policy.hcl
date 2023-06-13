# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

scaling "full-task-group-policy" {
  enabled = true
  min     = 1
  max     = 10
  type    = "horizontal"

  policy {

    cooldown            = "1m"
    evaluation_interval = "30s"

    check "cpu_nomad" {
      source = "nomad_apm"
      query  = "avg_cpu"

      strategy "target-value" {
        target = "80"
      }
    }

    check "memory_nomad" {
      source = "nomad_apm"
      query  = "avg_memory"

      strategy "target-value" {
        target = "80"
      }
    }

    target "nomad" {
      Group = "cache"
      Job   = "example"
    }
  }
}
