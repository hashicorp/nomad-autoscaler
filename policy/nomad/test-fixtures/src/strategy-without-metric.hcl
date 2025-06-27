# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "strategy-without-metric" {
  type        = "batch"

  group "test" {
    scaling {
      max = 10

      policy {
        check "check" {
          strategy "fixed-value" {
            int_config  = 2
            bool_config = true
            str_config  = "str"
          }
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
