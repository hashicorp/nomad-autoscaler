# Copyright IBM Corp. 2020, 2025
# SPDX-License-Identifier: MPL-2.0

job "missing-policy" {
  type = "batch"

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
