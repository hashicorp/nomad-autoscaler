# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "missing-strategy" {
  type        = "batch"

  group "test" {
    scaling {
      min     = 0
      max     = 10
      enabled = false

      policy {
        check "check" {
          source = "source"
          query  = "query"
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
