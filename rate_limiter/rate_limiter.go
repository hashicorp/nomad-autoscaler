// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package rate_limiter

import (
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/go-metrics"
	"golang.org/x/time/rate"
)

// CustomRoundTripper wraps http.RoundTripper to observe metrics and rate limit if necessary
type CustomRoundTripper struct {
	rateLimiter *rate.Limiter
	source      string
	rt          http.RoundTripper
}

func (crt *CustomRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	labels := []metrics.Label{
		{
			Name:  "method",
			Value: req.Method,
		},
		{
			Name:  "source",
			Value: crt.source,
		},
	}

	defer metrics.MeasureSinceWithLabels([]string{"http", "dur"}, time.Now(), labels)

	if crt.rateLimiter != nil {
		err := crt.rateLimiter.Wait(req.Context())
		if err != nil {
			return nil, fmt.Errorf("transport: unable to ratelimit: %w", err)
		}
	}

	resp, err := crt.rt.RoundTrip(req)
	if err == nil && resp != nil {
		metrics.IncrCounterWithLabels([]string{"http", "req"}, 1, labels)
	}

	return resp, err
}

// NewWrapper returns the provided http client with a rate limiter, if no client
// is provided, a new one will be created using github.com/hashicorp/go-cleanhttp.
// To disable rate limiting, set the ratePerSec to -1. Setting it to 0 blocks all
// requests. Source is used as a label for metrics.
func NewInstrumentedWrapper(source string, ratePerSec int, client *http.Client) *http.Client {
	httpClient := cleanhttp.DefaultPooledClient()
	if client != nil {
		httpClient = client
	}

	crt := &CustomRoundTripper{
		rt:     httpClient.Transport,
		source: source,
	}

	if ratePerSec != -1 {
		crt.rateLimiter = rate.NewLimiter(rate.Every(time.Second), ratePerSec)
	}

	httpClient.Transport = crt

	return httpClient
}
