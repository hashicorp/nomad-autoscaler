package policystorage

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/hashicorp/nomad/api"
)

func Test_storagNomad_Validate(t *testing.T) {
	tests := []struct {
		name   string
		policy *api.ScalingPolicy
		want   []error
	}{
		{
			name:   "validate missing",
			policy: &api.ScalingPolicy{
				Policy: map[string]interface{}{},
			},
			want:   []error{fmt.Errorf("Policy.strategy (<nil>) is not a []interface{}")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validate(tt.policy); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("validate() = %v, want %v", got, tt.want)
			}
		})
	}
}