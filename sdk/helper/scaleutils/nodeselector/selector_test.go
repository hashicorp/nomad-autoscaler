// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package nodeselector

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSelector(t *testing.T) {
	testCases := []struct {
		inputCfg             map[string]string
		expectedSelectorName string
		expectedOutputError  error
		name                 string
	}{
		{
			inputCfg:             map[string]string{},
			expectedSelectorName: "least_busy",
			expectedOutputError:  nil,
			name:                 "empty input config",
		},
		{
			inputCfg: map[string]string{
				"node_selector_strategy": "least_busy",
			},
			expectedSelectorName: "least_busy",
			expectedOutputError:  nil,
			name:                 "least busy configured",
		},
		{
			inputCfg: map[string]string{
				"node_selector_strategy": "empty",
			},
			expectedSelectorName: "empty",
			expectedOutputError:  nil,
			name:                 "empty configured",
		},
		{
			inputCfg: map[string]string{
				"node_selector_strategy": "newest_create_index",
			},
			expectedSelectorName: "newest_create_index",
			expectedOutputError:  nil,
			name:                 "newest create index nodes configured",
		},
		{
			inputCfg: map[string]string{
				"node_selector_strategy": "aliens",
			},
			expectedSelectorName: "",
			expectedOutputError:  errors.New("unsupported node selector strategy: aliens"),
			name:                 "unsupported config option",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualSelector, actualError := NewSelector(tc.inputCfg, nil, nil)
			if tc.expectedOutputError != nil {
				assert.Equal(t, tc.expectedOutputError, actualError, tc.name)
			} else {
				assert.NotNil(t, actualSelector, tc.name)
				assert.Equal(t, tc.expectedSelectorName, actualSelector.Name(), tc.name)
			}
		})
	}
}
