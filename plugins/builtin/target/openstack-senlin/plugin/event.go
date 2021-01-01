package plugin

import (
	"context"
	"fmt"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack/clustering/v1/clusters"
	hclog "github.com/hashicorp/go-hclog"
)

// scalingEvent represents an individual task within a long running cluster
// scaling event. Once we start to build more infrastructure provider target
// plugins we may wish to move this to plugins/target for public consumption.
type scalingEvent string

const (
	scalingEventDrain     scalingEvent = "drain"
	scalingEventDetach    scalingEvent = "detach"
	scalingEventTerminate scalingEvent = "terminate"
)

const (
	tagKey          = "nomad_autoscaler_lifecycle_phase"
	tagResourceType = "auto-scaling-group"

	// tagValueCharLimit is the size limit of an Senlin cluster tag and
	// is calculated using the current autoscaling limit, taking into account
	// that the tag will have the scalingEvent along with an underscore
	// prefixed on every write.
	tagValueCharLimit = 265 - len(scalingEventTerminate) - 1
)

type eventWriter struct {
	logger    hclog.Logger
	client    *gophercloud.ServiceClient
	ids       []string
	clusterID string
}

func newEventWriter(log hclog.Logger, client *gophercloud.ServiceClient, ids []string, clusterID string) *eventWriter {
	return &eventWriter{
		logger:    log,
		client:    client,
		ids:       chunkIDs(ids, tagValueCharLimit),
		clusterID: clusterID,
	}
}

// write creates or updates the Senlin Cluster with the appropriate event
// tags.
func (e *eventWriter) write(ctx context.Context, event scalingEvent) {

	tags := e.buildTags(event)

	opts := clusters.UpdateOpts{
		Metadata: tags,
	}

	if _, err := clusters.Update(e.client, e.clusterID, opts).Extract(); err != nil {
		e.logger.Error("failed to update Senlin Cluster metadata", "error", err, "event", event)
	}
	e.logger.Trace("successfully updated Senlin Cluster metadata", "event", event)
}

// buildTags iterates the eventWriters ID chunks and creates a map for the specified event.
func (e *eventWriter) buildTags(event scalingEvent) map[string]interface{} {

	var tags map[string]interface{}

	for i, chunk := range e.ids {
		tags[fmt.Sprintf("%v_%v", tagKey, i+1)] = fmt.Sprintf("%v_%v", event, chunk)
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
