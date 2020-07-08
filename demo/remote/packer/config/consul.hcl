advertise_addr = "IP_ADDRESS"

bind_addr = "0.0.0.0"

bootstrap_expect = SERVER_COUNT

client_addr = "0.0.0.0"

data_dir = "/opt/consul/data"

log_level = "INFO"

retry_join = ["RETRY_JOIN"]

server = true

ui = true

service {
  name = "consul"
}
