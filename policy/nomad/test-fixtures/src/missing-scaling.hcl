# Copyright IBM Corp. 2020, 2025
# SPDX-License-Identifier: MPL-2.0

job "missing-scaling" {
  type = "batch"

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
