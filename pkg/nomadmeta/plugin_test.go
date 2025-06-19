package nomadmeta

import (
	"bytes"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func Test_toQuery(t *testing.T) {
	tests := []struct {
		name    string
		q       string
		want    Query
		wantErr bool
	}{
		{
			name: "query example",
			q:    `job="ktools-test-service" group="ktools-test-service" node_pool="jump" Meta contains "run_apigw-envoy-be"`,
			want: Query{
				Job:       "ktools-test-service",
				Group:     "ktools-test-service",
				NodePool:  "jump",
				MetaQuery: `Meta contains "run_apigw-envoy-be" and SchedulingEligibility == "eligible" and Attributes contains "driver.docker"`,
			},
		},
		{
			name: "legacy query example",
			q:    `job="ktools-test-service" group="ktools-test-service" Meta contains "run_apigw-envoy-be"`,
			want: Query{
				Job:       "ktools-test-service",
				Group:     "ktools-test-service",
				NodePool:  nodePoolDefault,
				MetaQuery: `Meta contains "run_apigw-envoy-be" and SchedulingEligibility == "eligible" and Attributes contains "driver.docker"`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToQuery(tt.q)
			if (err != nil) != tt.wantErr {
				t.Errorf("toQuery() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNomadMeta_Query(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	counter := NewMockNodeCounter(ctrl)

	buffer := &bytes.Buffer{}

	n := NewNomadMeta(hclog.New(&hclog.LoggerOptions{
		Output: buffer,
	}), counter)

	q := Query{
		Job:       "controplane-agent",
		MetaQuery: `Meta contains "run_apigw-envoy-be"`,
		NodePool:  nodePoolDefault,
	}

	counter.EXPECT().GetNodeNames(q.MetaQuery, q.NodePool).Return([]string{
		"c101.our1", "c102.our1",
	}, nil)
	counter.EXPECT().RunningOnIneligibleNodes("controplane-agent").Return(nil, nil).AnyTimes()

	res, err := n.Query(q, sdk.TimeRange{})
	assert.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, res[0].Value, float64(2))

	// second call to check diff
	counter.EXPECT().GetNodeNames(q.MetaQuery, q.NodePool).Return([]string{
		"c101.our1", "c102.our1", "c103.our1",
	}, nil)
	res, err = n.Query(q, sdk.TimeRange{})
	assert.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, res[0].Value, float64(3))

	assert.Contains(t, buffer.String(), "c103.our1")
	assert.Contains(t, buffer.String(), "nodes changed")
}
