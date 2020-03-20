module github.com/hashicorp/nomad-autoscaler/target/plugins/nomad

go 1.13

require (
	github.com/hashicorp/go-plugin v1.0.1
	github.com/hashicorp/nomad-autoscaler v0.0.0-00010101000000-000000000000
	github.com/hashicorp/nomad/api v0.0.0-20200319143014-fbec08e1a687
)

replace github.com/hashicorp/nomad-autoscaler => ../../../
