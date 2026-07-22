// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
	"net/http"
	"testing"

	"github.com/shoenig/test/must"
)

func Test_zonalInstanceGroup_resizeAdvancedUsesBetaEndpoint(t *testing.T) {
	var gotURL string

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			gotURL = req.URL.String()
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       http.NoBody,
				Header:     make(http.Header),
				Request:    req,
			}, nil
		}),
	}

	ig := &zonalInstanceGroup{
		project: "my-project",
		zone:    "us-central1-a",
		name:    "my-mig",
	}

	err := ig.resize(context.Background(), nil, client, 3, true)

	must.NoError(t, err)
	must.Eq(t, "https://compute.googleapis.com/compute/beta/projects/my-project/zones/us-central1-a/instanceGroupManagers/my-mig/resizeAdvanced", gotURL)
}

func Test_regionalInstanceGroup_resizeAdvancedUsesBetaEndpoint(t *testing.T) {
	var gotURL string

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			gotURL = req.URL.String()
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       http.NoBody,
				Header:     make(http.Header),
				Request:    req,
			}, nil
		}),
	}

	ig := &regionalInstanceGroup{
		project: "my-project",
		region:  "us-central1",
		name:    "my-mig",
	}

	err := ig.resize(context.Background(), nil, client, 3, true)

	must.NoError(t, err)
	must.Eq(t, "https://compute.googleapis.com/compute/beta/projects/my-project/regions/us-central1/instanceGroupManagers/my-mig/resizeAdvanced", gotURL)
}
