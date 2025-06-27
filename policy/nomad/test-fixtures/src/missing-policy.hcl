# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "missing-policy" {
  type        = "batch"

  group "test" {
    scaling {
      min     = 2
      max     = 10
      enabled = false
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
