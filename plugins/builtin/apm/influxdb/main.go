// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package main

import (
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	influxdb "github.com/hashicorp/nomad-autoscaler/plugins/builtin/apm/influxdb/plugin"
)

func main() {
	plugins.Serve(factory)
}

// factory returns a new instance of the InfluxDB APM plugin.
func factory(log hclog.Logger) interface{} {
	return influxdb.NewInfluxDBPlugin(log)
}
