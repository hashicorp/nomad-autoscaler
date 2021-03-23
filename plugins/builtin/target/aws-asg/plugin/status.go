package plugin

import (
	"errors"
	"fmt"
	"strings"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/scaleutils"
)

// instanceTerminationResult tracks the state when performing a scale in action
// and helps account for the nature of the AWS API which only accepts a single
// instanceID per request.
type instanceTerminationResult struct {
	failed  []instanceFailure
	success []instanceSuccess
}

// instanceSuccess tracks the details of an instance whose terminate API call
// returned an error.
type instanceFailure struct {
	instance scaleutils.NodeResourceID
	err      error
}

// instanceSuccess tracks the details of an instance whose terminate API call
// completed successfully.
type instanceSuccess struct {
	instance   scaleutils.NodeResourceID
	activityID *string
}

// logResults is a convenience function that logs all the currently held status
// details to their appropriate logging level.
func (i *instanceTerminationResult) logResults(log hclog.Logger) {

	for _, success := range i.success {
		log.Debug("successfully terminated instance in ASG",
			"instance_id", success.instance.RemoteResourceID, "node_id", success.instance.NomadNodeID)
	}

	for _, failure := range i.failed {
		log.Error("failed to terminate instance in ASG",
			"instance_id", failure.instance.RemoteResourceID, "node_id", failure.instance.NomadNodeID,
			"error", failure.err)
	}
}

// Error satisfies the error interface, outputting all errors in a nicely
// formatted string.
func (i *instanceTerminationResult) Error() string {
	if i.lenFailure() < 1 {
		return ""
	}

	points := make([]string, i.lenFailure())
	for i, err := range i.failed {
		points[i] = fmt.Sprintf(
			"failed to terminate node %s with AWS ID %s: %v",
			err.instance.NomadNodeID, err.instance.RemoteResourceID, err.err)
	}
	return strings.Join(points, ", ")
}

// errorOrNil returns a new error if the result contains error entries, or nil
// if there are not any.
func (i *instanceTerminationResult) errorOrNil() error {
	if i.lenFailure() < 1 {
		return nil
	}
	return errors.New(i.Error())
}

// activityIDs returns a list of AWS AutoScaling activity IDs which were
// generated as a result of terminating instances.
func (i *instanceTerminationResult) activityIDs() []string {

	if i.lenSuccess() < 1 {
		return nil
	}

	activityIDs := make([]string, i.lenSuccess())
	for i, id := range i.success {
		activityIDs[i] = *id.activityID
	}
	return activityIDs
}

// successfulIDs returns the list of instances which were unsuccessfully
// terminated.
func (i *instanceTerminationResult) failedIDs() []scaleutils.NodeResourceID {

	if i.lenFailure() < 1 {
		return nil
	}

	ids := make([]scaleutils.NodeResourceID, i.lenFailure())
	for i, inst := range i.failed {
		ids[i] = inst.instance
	}
	return ids
}

// successfulIDs returns the list of instances which were successfully
// terminated.
func (i *instanceTerminationResult) successfulIDs() []scaleutils.NodeResourceID {

	if i.lenSuccess() < 1 {
		return nil
	}

	ids := make([]scaleutils.NodeResourceID, i.lenSuccess())
	for i, inst := range i.success {
		ids[i] = inst.instance
	}
	return ids
}

func (i *instanceTerminationResult) appendFailure(err instanceFailure) {
	i.failed = append(i.failed, err)
}

func (i *instanceTerminationResult) appendSuccess(inf instanceSuccess) {
	i.success = append(i.success, inf)
}

func (i *instanceTerminationResult) lenFailure() int { return len(i.failed) }
func (i *instanceTerminationResult) lenSuccess() int { return len(i.success) }
