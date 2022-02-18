job "full-scaling" {
  datacenters = ["dc1"]
  type        = "batch"

  group "test" {
    scaling {
      min     = 2
      max     = 10
      enabled = false

      policy {
        evaluation_interval = "5s"
        cooldown            = "5m"
        on_check_error      = "fail"

        target "target" {
          int_config  = 2
          bool_config = true
          str_config  = "str"
        }

        check "check-1" {
          source       = "source-1"
          query        = "query-1"
          query_window = "1m"
          on_error     = "ignore"

          strategy "strategy-1" {
            int_config  = 2
            bool_config = true
            str_config  = "str"
          }
        }

        check "check-2" {
          source = "source-2"
          query  = "query-2"
          group  = "group-2"

          strategy "strategy-2" {
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
