module github.com/hashicorp/nomad-autoscaler

go 1.13

require (
	github.com/docker/go-units v0.4.0 // indirect
	github.com/hashicorp/go-plugin v1.0.1
	github.com/hashicorp/hcl/v2 v2.3.0
	github.com/hashicorp/nomad/api v0.0.0-20200124004857-fea44b0d8e20
	github.com/mitchellh/cli v1.0.0
	github.com/prometheus/client_golang v1.4.0 // indirect
)

replace github.com/hashicorp/nomad/api => ../nomad-enterprise/api
