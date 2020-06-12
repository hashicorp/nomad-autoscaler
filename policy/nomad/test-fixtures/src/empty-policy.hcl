job "empty-policy" {
  datacenters = ["dc1"]

  group "test" {
    scaling {
      min     = 0
      max     = 10
      enabled = false

      policy {}
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
