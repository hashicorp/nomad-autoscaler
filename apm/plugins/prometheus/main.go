package main

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad-autoscaler/apm"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

var handshakeConfig = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "magic",
	MagicCookieValue: "magic",
}

type PrometheusAPM struct {
	client api.Client
	config map[string]string
}

func (p *PrometheusAPM) SetConfig(c map[string]string) error {
	config := api.Config{
		Address: c["address"],
	}

	// create Prometheus client
	client, err := api.NewClient(config)
	if err != nil {
		return fmt.Errorf("failed to initialize Prometheus client: %v", err)
	}

	// store config and client in plugin instance
	p.config = c
	p.client = client

	return nil
}

func (p *PrometheusAPM) Query(q string) (float64, error) {
	v1api := v1.NewAPI(p.client)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, warnings, err := v1api.Query(ctx, q, time.Now())
	if err != nil {
		return 0, fmt.Errorf("failed to query: %v", err)
	}
	for _, w := range warnings {
		fmt.Printf("[WARN] %s", w)
	}

	// only support scalar types for now
	t := result.Type()
	if t != model.ValScalar {
		return 0, fmt.Errorf("result type (`%v`) is not `scalar`", t)
	}

	s := result.(*model.Scalar)
	return float64(s.Value), nil
}

func main() {
	p := &PrometheusAPM{}
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: handshakeConfig,
		Plugins: map[string]plugin.Plugin{
			"apm": &apm.Plugin{Impl: p},
		},
	})

}
