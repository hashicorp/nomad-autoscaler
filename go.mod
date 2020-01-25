module github.com/hashicorp/nomad-autoscaler

go 1.13

require (
	github.com/docker/go-units v0.4.0 // indirect
	github.com/golang/protobuf v1.3.2 // indirect
	github.com/hashicorp/go-plugin v1.0.1
	github.com/hashicorp/hcl/v2 v2.3.0
	github.com/hashicorp/nomad/api v0.0.0-20200124004857-fea44b0d8e20
	github.com/mitchellh/cli v1.0.0
	golang.org/x/net v0.0.0-20190613194153-d28f0bde5980 // indirect
	golang.org/x/sys v0.0.0-20191220142924-d4481acd189f // indirect
)

replace github.com/hashicorp/nomad/api => ../nomad-enterprise/api
