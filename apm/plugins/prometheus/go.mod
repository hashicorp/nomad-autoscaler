module github.com/hashicorp/nomad-autoscaler/apm/plugins/prometheus

go 1.13

require (
	github.com/hashicorp/go-plugin v1.0.1
	github.com/hashicorp/nomad-autoscaler v0.0.0-00010101000000-000000000000
	github.com/prometheus/client_golang v1.4.0
	github.com/prometheus/common v0.9.1
)

replace github.com/hashicorp/nomad-autoscaler => ../../../
