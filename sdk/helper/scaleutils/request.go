package scaleutils

import (
	"errors"
	"time"

	multierror "github.com/hashicorp/go-multierror"
)

const (
	// DefaultDrainDeadline is the drainSpec deadline used if one is not
	// specified by an operator.
	DefaultDrainDeadline = 15 * time.Minute
)

// ScaleInReq represents an individual cluster scaling request and encompasses
// all the information needed to perform the pre-termination tasks.
type ScaleInReq struct {

	// Num is the number of nodes we should select and prepare for termination.
	Num int

	// DrainDeadline is the deadline used within the DrainSpec when performing
	// Nomad Node drain.
	DrainDeadline time.Duration

	PoolIdentifier *PoolIdentifier
	RemoteProvider RemoteProvider
	NodeIDStrategy NodeIDStrategy
}

// validate is used to ensure that ScaleInReq is correctly populated.
func (sr *ScaleInReq) validate() error {

	var err *multierror.Error

	if sr.Num < 1 {
		err = multierror.Append(errors.New("num should be positive and non-zero"), err)
	}

	if sr.DrainDeadline == 0 {
		err = multierror.Append(errors.New("deadline should be non-zero"), err)
	}

	if sr.PoolIdentifier == nil {
		err = multierror.Append(errors.New("pool identifier should be non-nil"), err)
	}

	if sr.RemoteProvider == "" {
		err = multierror.Append(errors.New("remote provider should be set"), err)
	}

	if sr.NodeIDStrategy == "" {
		err = multierror.Append(errors.New("node ID strategy should be set"), err)
	}

	return err.ErrorOrNil()
}
