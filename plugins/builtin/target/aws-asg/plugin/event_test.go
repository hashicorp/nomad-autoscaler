// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
)

func Test_eventWriter_buildTags(t *testing.T) {
	testCases := []struct {
		inputIDs       []string
		inputASGName   string
		inputEvent     scalingEvent
		expectedOutput []types.Tag
		name           string
	}{
		{
			inputIDs:     generateIDs(1),
			inputASGName: "test-test-asg",
			inputEvent:   scalingEventDrain,
			expectedOutput: []types.Tag{
				{
					Key:               aws.String("nomad_autoscaler_lifecycle_phase_1"),
					Value:             aws.String("drain_i-036e43a14e8f81001"),
					PropagateAtLaunch: aws.Bool(false),
					ResourceId:        aws.String("test-test-asg"),
					ResourceType:      aws.String("auto-scaling-group"),
				},
			},
			name: "single ID within event",
		},
		{
			inputIDs:     generateIDs(14),
			inputASGName: "test-test-asg",
			inputEvent:   scalingEventDrain,
			expectedOutput: []types.Tag{
				{
					Key:               aws.String("nomad_autoscaler_lifecycle_phase_1"),
					Value:             aws.String("drain_i-036e43a14e8f81001_i-036e43a14e8f81002_i-036e43a14e8f81003_i-036e43a14e8f81004_i-036e43a14e8f81005_i-036e43a14e8f81006_i-036e43a14e8f81007_i-036e43a14e8f81008_i-036e43a14e8f81009_i-036e43a14e8f81010_i-036e43a14e8f81011_i-036e43a14e8f81012"),
					PropagateAtLaunch: aws.Bool(false),
					ResourceId:        aws.String("test-test-asg"),
					ResourceType:      aws.String("auto-scaling-group"),
				},
				{
					Key:               aws.String("nomad_autoscaler_lifecycle_phase_2"),
					Value:             aws.String("drain_i-036e43a14e8f81013_i-036e43a14e8f81014"),
					PropagateAtLaunch: aws.Bool(false),
					ResourceId:        aws.String("test-test-asg"),
					ResourceType:      aws.String("auto-scaling-group"),
				},
			},
			name: "many IDs resulting in more than 1 tag",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ew := newEventWriter(hclog.NewNullLogger(), nil, tc.inputIDs, tc.inputASGName)
			actualOutput := ew.buildTags(tc.inputEvent)
			assert.Equal(t, tc.expectedOutput, actualOutput, tc.name)
		})
	}
}

func Test_chunkIDs(t *testing.T) {
	testCases := []struct {
		inputStrings   []string
		inputSize      int
		expectedOutput []string
		name           string
	}{
		{
			inputStrings: generateIDs(3),
			inputSize:    50,
			expectedOutput: []string{
				"i-036e43a14e8f81001_i-036e43a14e8f81002",
				"i-036e43a14e8f81003",
			},
			name: "3 items resulting in two array elements",
		},
		{
			inputStrings: generateIDs(2),
			inputSize:    50,
			expectedOutput: []string{
				"i-036e43a14e8f81001_i-036e43a14e8f81002",
			},
			name: "2 items resulting in single array element",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := chunkIDs(tc.inputStrings, tc.inputSize)
			assert.Equal(t, tc.expectedOutput, actualOutput, tc.name)
		})
	}
}

func generateIDs(num int) []string {

	if num > 8999 {
		panic("cannot generate more than 8999 IDs")
	}

	var ids []string

	for i := 1; i <= num; i++ {
		ids = append(ids, fmt.Sprintf("i-036e43a14e8f8%v", 1000+i))
	}
	return ids
}
