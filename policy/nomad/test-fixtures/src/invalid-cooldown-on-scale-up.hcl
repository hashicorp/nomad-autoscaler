# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "invalid-cooldown-on-scale-up" {
  type = "batch"

  group "test" {
    scaling {
      min     = 0
      max     = 10
      enabled = false

      policy {
        cooldown_on_scale_up = "invalid"
      }
    }

    task "echo" {
      driver = "raw_exec"
      config {
        command = "echo"
        args    = ["hi"]
      }
    }
  }
}
