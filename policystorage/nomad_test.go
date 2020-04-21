package policystorage

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
)

func Test_storagNomad_Validate(t *testing.T) {
	tests := []struct {
		name   string
		policy *api.ScalingPolicy
		want   []error
	}{
		{
			name: "validate missing",
			policy: &api.ScalingPolicy{
				Policy: map[string]interface{}{},
			},
			want: []error{fmt.Errorf("Policy.strategy (<nil>) is not a []interface{}")},
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

func Test_parseTarget(t *testing.T) {
	testCases := []struct {
		inputTarget    interface{}
		expectedOutput *Target
		expectedError  error
		name           string
	}{
		{
			inputTarget:    nil,
			expectedOutput: &Target{},
			expectedError:  nil,
			name:           "input target is nil",
		},
		{
			inputTarget:    []interface{}{map[string]interface{}{}},
			expectedOutput: &Target{Config: map[string]string{}},
			expectedError:  nil,
			name:           "input target is empty slice of maps",
		},
		{
			inputTarget: []interface{}{map[string]interface{}{
				"config": []interface{}{map[string]interface{}{"dry-run": "true"}},
			}},
			expectedOutput: &Target{Config: map[string]string{"dry-run": "true"}},
			expectedError:  nil,
			name:           "input target with config but without name",
		},
		{
			inputTarget: []interface{}{map[string]interface{}{
				"name": map[string]string{"foo": "bar"},
			}},
			expectedOutput: nil,
			expectedError:  fmt.Errorf("target name is map[string]string not string"),
			name:           "input target with name of the wrong type",
		},
		{
			inputTarget:    "foo",
			expectedOutput: nil,
			expectedError:  fmt.Errorf("target block is string not interface slice"),
			name:           "input target with wrong type",
		},

		{
			inputTarget:    []interface{}{[]string{}},
			expectedOutput: nil,
			expectedError:  fmt.Errorf("target block first item is []string not map"),
			name:           "input target interface slice is wrong type",
		},
	}

	for _, tc := range testCases {
		actualOutput, actualError := parseTarget(tc.inputTarget)
		assert.Equal(t, tc.expectedError, actualError, tc.name)
		assert.Equal(t, tc.expectedOutput, actualOutput, tc.name)
	}
}

func Test_parseConfig(t *testing.T) {
	testCases := []struct {
		inputConfig    interface{}
		expectedOutput map[string]string
		expectedError  error
		name           string
	}{
		{
			inputConfig:    nil,
			expectedOutput: map[string]string{},
			expectedError:  nil,
			name:           "input config is nil",
		},
		{
			inputConfig:    "foo",
			expectedOutput: nil,
			expectedError:  fmt.Errorf("config block is string not interface slice"),
			name:           "input config with wrong type",
		},
		{
			inputConfig:    []interface{}{[]string{}},
			expectedOutput: nil,
			expectedError:  fmt.Errorf("config block first item is []string not map"),
			name:           "input config interface slice is wrong type",
		},
		{
			inputConfig: []interface{}{map[string]interface{}{
				"dry-run": []string{"oh-no", "this-is-wrong"},
				"feature": []int{13, 04}},
			},
			expectedOutput: map[string]string{},
			expectedError: &multierror.Error{Errors: []error{
				fmt.Errorf("config key dry-run value is []string not string"),
				fmt.Errorf("config key feature value is []int not string")},
			},
			name: "input config parsable but incorrect map value types",
		},
		{
			inputConfig: []interface{}{map[string]interface{}{
				"dry-run": "true",
				"feature": "fancy-feature-toggle"},
			},
			expectedOutput: map[string]string{"dry-run": "true", "feature": "fancy-feature-toggle"},
			expectedError:  nil,
			name:           "input config parsable and correctly formatted",
		},
	}

	for _, tc := range testCases {
		actualOutput, actualError := parseConfig(tc.inputConfig)
		assert.Equal(t, tc.expectedError, actualError, tc.name)
		assert.Equal(t, tc.expectedOutput, actualOutput, tc.name)
	}
}

func Test_parseGenericBlock(t *testing.T) {
	testCases := []struct {
		input          interface{}
		expectedOutput map[string]interface{}
		expectedError  error
		name           string
	}{
		{
			input:          "foo",
			expectedOutput: nil,
			expectedError:  fmt.Errorf("block is string not interface slice"),
			name:           "input with wrong type",
		},
		{
			input:          []interface{}{[]string{}},
			expectedOutput: nil,
			expectedError:  fmt.Errorf("block first item is []string not map"),
			name:           "input interface slice is wrong type",
		},
		{
			input: []interface{}{map[string]interface{}{
				"config": []interface{}{map[string]interface{}{"dry-run": "true"}},
			}},
			expectedOutput: map[string]interface{}{"config": []interface{}{map[string]interface{}{"dry-run": "true"}}},
			expectedError:  nil,
			name:           "input is parsable and correctly formatted",
		},
	}

	for _, tc := range testCases {
		actualOutput, actualError := parseGenericBlock(tc.input)
		assert.Equal(t, tc.expectedError, actualError, tc.name)
		assert.Equal(t, tc.expectedOutput, actualOutput, tc.name)
	}
}
