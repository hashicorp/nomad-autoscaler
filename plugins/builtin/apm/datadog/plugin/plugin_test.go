package plugin

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_parseDatadogRawQuery(t *testing.T) {
	testCases := []struct {
		name               string
		rawQuery           string
		expectedQuery      string
		expectedTimeWindow time.Duration
		expectError        bool
	}{
		{
			name:               "simple query",
			rawQuery:           "FROM=1m;TO=0m;QUERY=foo",
			expectedQuery:      "foo",
			expectedTimeWindow: time.Minute,
			expectError:        false,
		},
		{
			name:               "change order of components",
			rawQuery:           "TO=0m;QUERY=foo;FROM=1m",
			expectedQuery:      "foo",
			expectedTimeWindow: time.Minute,
			expectError:        false,
		},
		{
			name:               "error: trailing delimiter",
			rawQuery:           "FROM=1m;TO=0m;QUERY=foo;;",
			expectedQuery:      "foo",
			expectedTimeWindow: time.Minute,
			expectError:        true,
		},
		{
			name:               "error: from after to",
			rawQuery:           "FROM=1m;TO=3m;QUERY=foo",
			expectedQuery:      "foo",
			expectedTimeWindow: time.Minute,
			expectError:        true,
		},
		{
			name:               "error: empty query",
			rawQuery:           "FROM=1m;TO=3m;QUERY=",
			expectedQuery:      "",
			expectedTimeWindow: time.Minute,
			expectError:        true,
		},
		{
			name:               "error: missing query",
			rawQuery:           "FROM=1m;TO=3m",
			expectedQuery:      "",
			expectedTimeWindow: time.Minute,
			expectError:        true,
		},
		{
			name:               "error: missing from",
			rawQuery:           "TO=3m;QUERY=foo",
			expectedQuery:      "",
			expectedTimeWindow: time.Minute,
			expectError:        true,
		},
		{
			name:               "error: missing from and to",
			rawQuery:           "QUERY=foo",
			expectedQuery:      "",
			expectedTimeWindow: time.Minute,
			expectError:        true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput, err := parseRawQuery(tc.rawQuery)
			if tc.expectError {
				assert.NotNil(t, err, tc.rawQuery, tc.name)
				return
			}
			assert.Nil(t, err, tc.name)
			assert.Equal(t, tc.expectedQuery, actualOutput.query, tc.name)
			assert.Equal(t, tc.expectedTimeWindow, actualOutput.to.Sub(actualOutput.from), tc.name)
		})
	}
}
