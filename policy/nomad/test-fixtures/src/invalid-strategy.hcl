# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "invalid-strategy" {
  type = "batch"

  group "test" {
    scaling {
      min     = 0
      max     = 10
      enabled = false

      policy {
        check "check" {
          strategy {}
        }
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
