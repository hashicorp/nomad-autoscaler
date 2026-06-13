package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// instanaClient handles HTTP communication with the Instana REST API.
type instanaClient struct {
	baseURL  *url.URL
	apiToken string
	http     *http.Client
}

func newInstanaClient() *instanaClient {
	return &instanaClient{
		http: &http.Client{},
	}
}

// getInfrastructureMetrics sends a POST request to the Instana infrastructure metrics endpoint.
func (c *instanaClient) getInfrastructureMetrics(ctx context.Context, request instanaMetricsRequest) (*instanaMetricsResponse, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal instana query request: %w", err)
	}

	metricsURL := *c.baseURL // take a copy
	metricsURL.Path = strings.TrimSuffix(metricsURL.Path, "/") + metricsPath

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, metricsURL.String(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to build instana query request: %w", err)
	}

	req.Header.Set("Authorization", "apiToken "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error querying metrics from instana: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("metric queries are ratelimited by instana, resets at %s",
			resp.Header.Get(rateLimitResetHdr))
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("instana query failed with status %d: %s",
			resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var metricsResp instanaMetricsResponse
	if err := json.NewDecoder(resp.Body).Decode(&metricsResp); err != nil {
		return nil, fmt.Errorf("failed to decode instana response: %w", err)
	}

	return &metricsResp, nil
}
