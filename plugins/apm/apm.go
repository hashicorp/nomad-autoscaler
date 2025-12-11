// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package apm

import (
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

// APM is the interface that all APM plugins are required to implement and
// details the functions that are used to provide the Autoscaler with metrics
// to make scaling decisions on.
type APM interface {

	// Embed the base.Base ensuring that APM plugins implement this
	// interface.
	base.Base

	Looker
}

type Looker interface {
	// Query is used to ask the remote APM for timestamped metrics based on the
	// passed query and time range.
	Query(query string, timeRange sdk.TimeRange) (sdk.TimestampedMetrics, error)

	// QueryMultiple is used exclusively by Dynamic Application Sizing in order
	// to gather the metrics desired by the feature.
	QueryMultiple(query string, timeRange sdk.TimeRange) ([]sdk.TimestampedMetrics, error)
}
