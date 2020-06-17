job "invalid-query" {
  datacenters = ["dc1"]
  type        = "batch"

  group "test" {
    scaling {
      max = 10

      policy {
        check "check" {
          query = 5

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
