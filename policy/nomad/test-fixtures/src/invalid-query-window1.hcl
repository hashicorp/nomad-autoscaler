# Copyright IBM Corp. 2020, 2025
# SPDX-License-Identifier: MPL-2.0

job "invalid-query-window1" {
  type = "batch"

  group "test" {
    scaling {
      max = 10

      policy {
        check "check" {
          query        = "query"
          query_window = 5

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
