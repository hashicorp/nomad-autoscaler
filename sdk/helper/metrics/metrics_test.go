package metrics

import (
	"fmt"
	"strings"
	"testing"
	"time"

	m "github.com/armon/go-metrics"
	"github.com/stretchr/testify/assert"
)

func Test_SetDefaultLabels(t *testing.T) {
	testCases := []struct {
		inputLabels           []Label
		expectedDefaultLabels []Label
		name                  string
	}{
		{
			inputLabels:           nil,
			expectedDefaultLabels: nil,
			name:                  "no default labels",
		},
		{
			inputLabels: []Label{
				{Name: "global_label_name_1", Value: "global_label_value_1"},
				{Name: "global_label_name_2", Value: "global_label_value_2"},
			},
			expectedDefaultLabels: []Label{
				{Name: "global_label_name_1", Value: "global_label_value_1"},
				{Name: "global_label_name_2", Value: "global_label_value_2"},
			},
			name: "default labels",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			SetDefaultLabels(tc.inputLabels)
			assert.ElementsMatch(t, tc.expectedDefaultLabels, defaultLabels.Load(), tc.name)
		})
	}
}

func Test_SetGauge(t *testing.T) {
	testCases := []struct {
		inputKey      []string
		inputVal      float32
		defaultLabels []Label
		name          string
	}{
		{
			inputKey:      []string{"system", "total_num"},
			inputVal:      13,
			defaultLabels: nil,
			name:          "no default labels",
		},
		{
			inputKey: []string{"system", "total_num"},
			inputVal: 13,
			defaultLabels: []Label{
				{Name: "global_label_name_1", Value: "global_label_value_1"},
			},
			name: "default labels",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			// Setup the inMemory sink and any default labels.
			sink := setupTestSink()
			SetDefaultLabels(tc.defaultLabels)

			// Set a metric value based on the test input.
			SetGauge(tc.inputKey, tc.inputVal)

			// Grab the metric data and ensure our intervals have not crossed.
			intervals := sink.Data()
			if len(intervals) > 1 {
				t.Skip("detected interval crossing")
			}

			g, ok := intervals[0].Gauges[generateExpectedName(tc.inputKey, tc.defaultLabels, nil)]
			assert.True(t, ok, tc.name)
			assert.Equal(t, tc.inputVal, g.Value, tc.name)
			assert.ElementsMatch(t, g.Labels, tc.defaultLabels, tc.name)
		})
	}
}

func Test_MeasureSinceWithLabels(t *testing.T) {
	testCases := []struct {
		inputKey      []string
		inputLabels   []Label
		defaultLabels []Label
		name          string
	}{
		{
			inputKey:      []string{"system", "invoke_ms"},
			inputLabels:   nil,
			defaultLabels: nil,
			name:          "no default labels no input labels",
		},
		{
			inputKey:    []string{"system", "invoke_ms"},
			inputLabels: nil,
			defaultLabels: []Label{
				{Name: "global_label_name_1", Value: "global_label_value_1"},
			},
			name: "default labels no input labels",
		},
		{
			inputKey: []string{"system", "invoke_ms"},
			inputLabels: []Label{
				{Name: "local_label_name_1", Value: "local_label_value_1"},
			},
			defaultLabels: nil,
			name:          "no default labels input labels",
		},
		{
			inputKey: []string{"system", "invoke_ms"},
			inputLabels: []Label{
				{Name: "local_label_name_1", Value: "local_label_value_1"},
			},
			defaultLabels: []Label{
				{Name: "global_label_name_1", Value: "global_label_value_1"},
			},
			name: "default labels and input labels",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			// Setup the inMemory sink and any default labels.
			sink := setupTestSink()
			SetDefaultLabels(tc.defaultLabels)

			// Set a metric value.
			MeasureSinceWithLabels(tc.inputKey, time.Now(), tc.inputLabels)

			// Grab the metric data and ensure our intervals have not crossed.
			intervals := sink.Data()
			if len(intervals) > 1 {
				t.Skip("detected interval crossing")
			}

			sample, ok := intervals[0].Samples[generateExpectedName(tc.inputKey, tc.defaultLabels, tc.inputLabels)]
			assert.True(t, ok, tc.name)
			assert.NotZero(t, sample.Sum, tc.name)
			assert.ElementsMatch(t, mergeLabelSets(tc.defaultLabels, tc.inputLabels), sample.Labels, tc.name)
		})
	}
}

func Test_IncrCounter(t *testing.T) {
	testCases := []struct {
		inputKey      []string
		inputVal      float32
		defaultLabels []Label
		name          string
	}{
		{
			inputKey:      []string{"critical_system", "error_count"},
			inputVal:      1,
			defaultLabels: nil,
			name:          "no default labels",
		},
		{
			inputKey: []string{"critical_system", "error_count"},
			inputVal: 1,
			defaultLabels: []Label{
				{Name: "global_label_name_1", Value: "global_label_value_1"},
			},
			name: "default labels",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			// Setup the inMemory sink and any default labels.
			sink := setupTestSink()
			SetDefaultLabels(tc.defaultLabels)

			// Set a metric value based on the test input.
			IncrCounter(tc.inputKey, tc.inputVal)

			// Grab the metric data and ensure our intervals have not crossed.
			intervals := sink.Data()
			if len(intervals) > 1 {
				t.Skip("detected interval crossing")
			}

			counter, ok := intervals[0].Counters[generateExpectedName(tc.inputKey, tc.defaultLabels, nil)]
			assert.True(t, ok, tc.name)
			assert.Equal(t, float64(tc.inputVal), counter.Sum, tc.name)
			assert.ElementsMatch(t, counter.Labels, tc.defaultLabels, tc.name)
		})
	}
}

func Test_IncrCounterWithLabels(t *testing.T) {
	testCases := []struct {
		inputKey      []string
		inputVal      float32
		inputLabels   []Label
		defaultLabels []Label
		name          string
	}{
		{
			inputKey:      []string{"critical_system", "error_count"},
			inputVal:      1,
			inputLabels:   nil,
			defaultLabels: nil,
			name:          "no default labels no input labels",
		},
		{
			inputKey:    []string{"critical_system", "error_count"},
			inputVal:    1,
			inputLabels: nil,
			defaultLabels: []Label{
				{Name: "global_label_name_1", Value: "global_label_value_1"},
			},
			name: "default labels no input labels",
		},
		{
			inputKey: []string{"critical_system", "error_count"},
			inputVal: 1,
			inputLabels: []Label{
				{Name: "local_label_name_1", Value: "local_label_value_1"},
			},
			defaultLabels: nil,
			name:          "no default labels input labels",
		},
		{
			inputKey: []string{"critical_system", "error_count"},
			inputVal: 1,
			inputLabels: []Label{
				{Name: "local_label_name_1", Value: "local_label_value_1"},
			},
			defaultLabels: []Label{
				{Name: "global_label_name_1", Value: "global_label_value_1"},
			},
			name: "default labels and input labels",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			// Setup the inMemory sink and any default labels.
			sink := setupTestSink()
			SetDefaultLabels(tc.defaultLabels)

			// Set a metric value.
			IncrCounterWithLabels(tc.inputKey, tc.inputVal, tc.inputLabels)

			// Grab the metric data and ensure our intervals have not crossed.
			intervals := sink.Data()
			if len(intervals) > 1 {
				t.Skip("detected interval crossing")
			}

			counter, ok := intervals[0].Counters[generateExpectedName(tc.inputKey, tc.defaultLabels, tc.inputLabels)]
			assert.True(t, ok, tc.name)
			assert.Equal(t, float64(tc.inputVal), counter.Sum, tc.name)
			assert.ElementsMatch(t, mergeLabelSets(tc.defaultLabels, tc.inputLabels), counter.Labels, tc.name)
		})
	}
}

func setupTestSink() *m.InmemSink {
	inMem := m.NewInmemSink(1000000*time.Hour, 2000000*time.Hour)
	cfg := m.DefaultConfig("")
	cfg.EnableHostname = false
	_, _ = m.NewGlobal(cfg, inMem)
	return inMem
}

func generateExpectedName(key []string, defaultLabels, additionalLabels []Label) string {
	expectedName := strings.Join(key, ".")
	for _, l := range additionalLabels {
		expectedName = fmt.Sprintf("%s;%s=%s", expectedName, l.Name, l.Value)
	}
	for _, l := range defaultLabels {
		expectedName = fmt.Sprintf("%s;%s=%s", expectedName, l.Name, l.Value)
	}
	return expectedName
}

func mergeLabelSets(a, b []Label) []Label {
	var out []Label
	out = append(out, a...)
	out = append(out, b...)
	return out
}
