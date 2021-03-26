module github.com/hashicorp/nomad-autoscaler/plugins/test/noop-target

go 1.14

require (
	github.com/hashicorp/go-hclog v0.15.0
	github.com/hashicorp/nomad-autoscaler v0.3.1
	github.com/stretchr/testify v1.6.1
)

replace github.com/hashicorp/nomad-autoscaler => ../../..
