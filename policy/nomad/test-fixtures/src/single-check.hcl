# Copyright IBM Corp. 2020, 2025
# SPDX-License-Identifier: MPL-2.0

job "single-check" {
  type = "batch"

  group "test" {
    scaling {
      min     = 0
      max     = 10
      enabled = false

      policy {
        check "check" {
          source = "source"
          query  = "query"

          strategy "strategy" {
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
