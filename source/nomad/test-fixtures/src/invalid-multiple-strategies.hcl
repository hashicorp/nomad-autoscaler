# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "invalid-multiple-strategies" {
  datacenters = ["dc1"]
  type        = "batch"

  group "test" {
    scaling {
      max = 10

      policy {
        check "check" {
          query = "query"

          strategy "strategy-1" {}
          strategy "strategy-2" {}
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
