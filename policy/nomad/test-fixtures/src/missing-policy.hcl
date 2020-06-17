job "missing-policy" {
  datacenters = ["dc1"]
  type        = "batch"

  group "test" {
    scaling {
      min     = 2
      max     = 10
      enabled = false
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
