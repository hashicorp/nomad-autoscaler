// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	ociip "github.com/hashicorp/nomad-autoscaler/plugins/builtin/target/oci-instance-pool/plugin"
)

func main() {
	plugins.Serve(factory)
}

// factory returns a new instance of the OCI Instance Pool plugin.
func factory(log hclog.Logger) interface{} {
	return ociip.NewOCIInstancePoolPlugin(log)
}
