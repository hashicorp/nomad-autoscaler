# Copyright IBM Corp. 2020, 2025
# SPDX-License-Identifier: MPL-2.0

job "invalid-check" {
  type = "batch"

  group "test" {
    scaling {
      min     = 0
      max     = 10
      enabled = false

      policy {
        check {}
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
