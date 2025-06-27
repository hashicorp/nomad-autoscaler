# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "missing-scaling" {
  type        = "batch"

  group "test" {
    task "echo" {
      driver = "raw_exec"
      config {
        command = "echo"
        args    = ["hi"]
      }
    }
  }
}
