module github.com/hashicorp/nomad-autoscaler/apm/plugins/nomad-apm

go 1.13

require (
	github.com/hashicorp/go-hclog v0.12.0
	github.com/hashicorp/go-plugin v1.0.1
	github.com/hashicorp/nomad-autoscaler v0.0.0-00010101000000-000000000000
	github.com/hashicorp/nomad/api v0.0.0-20200130144102-d82904e54e39
	github.com/opentracing/opentracing-go v1.1.0 // indirect
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common v0.9.1
	github.com/prometheus/prometheus v2.5.0+incompatible
	github.com/prometheus/tsdb v0.10.0 // indirect
	github.com/tidwall/gjson v1.4.0
)

replace (
	github.com/hashicorp/nomad-autoscaler => ../../../
	github.com/hashicorp/nomad/api => ../../../../nomad/api
)
