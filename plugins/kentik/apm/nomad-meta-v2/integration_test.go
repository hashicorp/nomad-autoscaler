//go:build integration_test

package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/pkg/nomadmeta"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/stretchr/testify/require"
)

func Test_IntegrationNomadMetaV2(t *testing.T) {
	n := &NomadMeta{
		logger: hclog.Default(),
	}

	cfg := map[string]string{
		nomadmeta.ConfigKeyNomadAddress: "https://nomad.our1.kentik.com",
		nomadmeta.ConfigKeyPageSize:     "100",
	}
	require.NoError(t, n.SetConfig(cfg))

	query := `job="controlplane-agent" group="controlplane-agent" node_pool="default" Meta contains "run_controlplane-agent"`

	for i := 0; i < 10; i++ {
		response, err := n.Query(query,
			sdk.TimeRange{From: time.Now().Add(-time.Minute * 5), To: time.Now()},
		)
		require.NoError(t, err)
		fmt.Printf("val: %d", int(response[0].Value))
	}
}
