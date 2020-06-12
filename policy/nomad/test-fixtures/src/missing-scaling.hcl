job "missing-scaling" {
  datacenters = ["dc1"]
  type        = "batch"

  group "test" {
    task "echo" {
      driver = "raw_exec"
      config {
        command = "echo"
        args    = ["hi"]
      }
    }
  }
}
