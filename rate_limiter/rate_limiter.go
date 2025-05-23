// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package rate_limiter

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-cleanhttp"
	"golang.org/x/time/rate"
)

// CustomRoundTRipper wraps http.RoundTripper to observe metrics and rate limit if necessary
type CustomRoundTRipper struct {
	rateLimiter *rate.Limiter
	source      string
	rt          http.RoundTripper
}

func (irt *CustomRoundTRipper) RoundTrip(req *http.Request) (*http.Response, error) {
	if irt.rateLimiter != nil {
		ctx := context.Background()
		err := irt.rateLimiter.Wait(ctx)
		if err != nil {
			return nil, fmt.Errorf("transport: unable to ratelimit: %w", err)
		}
	}

	labels := []metrics.Label{
		{
			Name:  "method",
			Value: req.Method,
		},
		{
			Name:  "source",
			Value: irt.source,
		},
	}

	defer metrics.MeasureSinceWithLabels([]string{"http", "dur"}, time.Now(), labels)

	resp, err := irt.rt.RoundTrip(req)
	if err == nil && resp != nil {
		metrics.IncrCounterWithLabels([]string{"http", "req"}, 1, labels)
	}

	return resp, err
}

// NewWarpper returns the provided http client with a rate limiter, if no client
// is provided, a new one will be created using github.com/hashicorp/go-cleanhttp.
// To disable rate limiting, set the ratePerSec to -1. Setting it to 0 blocks all
// requests. Source is used as a label for metrics.
func NewInstrumentedWrapper(source string, ratePerSec int, client *http.Client) *http.Client {
	httpClient := cleanhttp.DefaultPooledClient()
	if client != nil {
		httpClient = client
	}

	httpClient.Transport.(*http.Transport).MaxConnsPerHost = 50

	crt := &CustomRoundTRipper{
		rt:     httpClient.Transport,
		source: source,
	}

	if ratePerSec != -1 {
		crt.rateLimiter = rate.NewLimiter(rate.Every(time.Second), ratePerSec)
	}

	httpClient.Transport = crt

	return httpClient
}
