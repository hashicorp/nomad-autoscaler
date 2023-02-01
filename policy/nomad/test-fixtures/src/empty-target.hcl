# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "empty-target" {
  datacenters = ["dc1"]
  type        = "batch"

  group "test" {
    scaling {
      min     = 0
      max     = 10
      enabled = false

      policy {
        target "target" {}
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
