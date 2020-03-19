module github.com/hashicorp/nomad-autoscaler/strategy/plugins/target-value

go 1.13

replace (
	github.com/hashicorp/nomad-autoscaler => ../../../
)

require (
	github.com/hashicorp/go-plugin v1.0.1
	github.com/hashicorp/nomad-autoscaler v0.0.0-00010101000000-000000000000
)
