# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "scaling-policy-check-with-min-max-scaling" {
  datacenters = ["dc1"]
  type        = "batch"

  group "test" {
    scaling {
      max = 10

      policy {
        check "check" {
          query = "query"

          scaling_min = 3
          scaling_max = 9
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
