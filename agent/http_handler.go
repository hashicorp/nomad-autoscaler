// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package agent

import "net/http"

// The methods in this file implement in the http.AgentHTTP interface.

func (a *Agent) DisplayMetrics(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	return a.inMemSink.DisplayMetrics(resp, req)
}

func (a *Agent) ReloadAgent(_ http.ResponseWriter, _ *http.Request) (interface{}, error) {
	a.reload()
	return nil, nil
}
