# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

log_level    = "TRACE"
log_json     = true
enable_debug = true
plugin_dir   = "./plugin_dir_from_file"

http {
  bind_address = "10.0.0.2"
  bind_port    = 8888
}

nomad {
  address         = "http://nomad_from_file.example.com:4646"
  region          = "file"
  namespace       = "staging"
  token           = "TOKEN_FROM_FILE"
  http_auth       = "user:file"
  ca_cert         = "./ca-cert-from-file.pem"
  ca_path         = "./ca-cert-from-file"
  client_cert     = "./client-cert-from-file.pem"
  client_key      = "./client-key-from-file.pem"
  tls_server_name = "tls_from_file"
  skip_verify     = true
}

policy {
  dir                         = "./policy-dir-from-file"
  default_cooldown            = "12s"
  default_evaluation_interval = "50m"

  source "file" {
    enabled = false
  }

  source "nomad" {
    enabled = false
  }
}

policy_eval {
  delivery_limit = 10
  ack_timeout    = "3m"

  workers = {
    cluster    = 3
    horizontal = 1
  }
}
