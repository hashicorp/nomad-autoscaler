datacenter       = "dc1"
advertise_addr   = "127.0.0.1"
client_addr      = "0.0.0.0"
data_dir         = "/opt/consul"
server           = true
bootstrap_expect = 1
ui               = true

telemetry {
  prometheus_retention_time = "30s"
}
