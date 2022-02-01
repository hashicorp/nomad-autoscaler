package plugin

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	hclog "github.com/hashicorp/go-hclog"
)

// scalingEvent represents an individual task within a long running cluster
// scaling event. Once we start to build more infrastructure provider target
// plugins we may wish to move this to plugins/target for public consumption.
type scalingEvent string

const (
	scalingEventDrain     scalingEvent = "drain"
	scalingEventTerminate scalingEvent = "terminate"
)

const (
	tagKey          = "nomad_autoscaler_lifecycle_phase"
	tagResourceType = "auto-scaling-group"

	// tagValueCharLimit is the size limit of an AWS AutoScaling Group tag and
	// is calculated using the current autoscaling limit, taking into account
	// that the tag will have the scalingEvent along with an underscore
	// prefixed on every write.
	tagValueCharLimit = 265 - len(scalingEventTerminate) - 1
)

type eventWriter struct {
	logger  hclog.Logger
	asg     *autoscaling.Client
	ids     []string
	asgName string
}

func newEventWriter(log hclog.Logger, asgClient *autoscaling.Client, ids []string, asg string) *eventWriter {
	return &eventWriter{
		logger:  log,
		asg:     asgClient,
		ids:     chunkIDs(ids, tagValueCharLimit),
		asgName: asg,
	}
}

// write creates or updates the AutoScaling Group with the appropriate event
// tags.
func (e *eventWriter) write(ctx context.Context, event scalingEvent) {

	input := autoscaling.CreateOrUpdateTagsInput{Tags: e.buildTags(event)}

	// Call the AWS API. If we get an error when creating/updating the tag we
	// do not bail on the whole process. It does inhibit our ability to perform
	// reconciliation, but not necessarily scaling actions. This could fail if
	// the AWS credentials are missing the autoscaling:CreateOrUpdateTags IAM
	// action.
	if _, err := e.asg.CreateOrUpdateTags(ctx, &input); err != nil {
		e.logger.Error("failed to update AutoScaling Group tag", "error", err, "event", event)
	}
	e.logger.Trace("successfully updated AutoScaling Group tag", "event", event)
}

// buildTags iterates the eventWriters ID chunks and creates a list of AWS
// autoscaling tags for the specified event.
func (e *eventWriter) buildTags(event scalingEvent) []types.Tag {

	var tags []types.Tag

	for i, chunk := range e.ids {
		tags = append(tags, types.Tag{
			Key:               aws.String(fmt.Sprintf("%v_%v", tagKey, i+1)),
			Value:             aws.String(fmt.Sprintf("%v_%v", event, chunk)),
			PropagateAtLaunch: aws.Bool(false),
			ResourceId:        aws.String(e.asgName),
			ResourceType:      aws.String(tagResourceType),
		})
	}
	return tags
}

// chunkIDs is used to format the ID strings used when creating tag ensuring
// each string of concatenated IDs does not exceed the limit.
func chunkIDs(s []string, size int) []string {

	index := 0

	// This feels wrong, but I(jrasell) have not found an alternate way to get
	// this to work. This at least works.
	values := []string{""}

	for _, val := range s {

		if len(values[index]) == 0 {
			values[index] = val
			continue
		}

		if len(values[index])+len(val)+1 > size {
			values = append(values, val)
			index++
			continue
		}
		values[index] = fmt.Sprintf("%v_%s", values[index], val)
	}

	return values
}
