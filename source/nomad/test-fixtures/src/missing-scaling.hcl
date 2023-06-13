# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "missing-scaling" {
  datacenters = ["dc1"]
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
