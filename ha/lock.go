package ha

import (
	"context"
	"math/rand"
	"time"

	log "github.com/hashicorp/go-hclog"
)

type lock interface {
	Acquire(ctx context.Context) (bool, error)
	Release(ctx context.Context) error
	Renew(ctx context.Context) error
}

type HAController struct {
	renewalPeriod time.Duration
	waitPeriod    time.Duration

	logger       log.Logger
	lock         lock
	ranGenerator *rand.Rand
}

func NewHAController(l lock) {

}

func (hc *HAController) Start(ctx context.Context, protectedFunc func(ctx context.Context)) error {
	// To avoid collisions if all the instances start at the same time, wait
	// a random time before making the first call.
	hc.wait(ctx)

	waitTimer := time.NewTimer(hc.waitPeriod)
	defer waitTimer.Stop()

	for {

		acquired, err := hc.lock.Acquire(ctx)
		if err != nil {
			hc.logger.Error("unable to get lock", err)
		}

		if acquired {
			funcCtx, cancel := context.WithCancel(ctx)
			defer cancel()

			// Start running the lock protected function
			go protectedFunc(funcCtx)

			// Maintain lease is a blocking function, will only return in case
			// the lock is lost.
			err := hc.maintainLease(ctx)
			if err != nil {
				hc.logger.Debug("lease lost", err)
				cancel()

				// Give the protected function some time to return before potentially
				// running it again.
				hc.wait(ctx)
			}
		}

		if !waitTimer.Stop() {
			<-waitTimer.C
		}
		waitTimer.Reset(hc.waitPeriod)

		select {
		case <-ctx.Done():
			return nil

		case <-waitTimer.C:
			waitTimer.Reset(hc.waitPeriod)
		}
	}
}

func (hc *HAController) maintainLease(ctx context.Context) error {
	renewTimer := time.NewTimer(hc.renewalPeriod)
	defer renewTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-renewTimer.C:
			err := hc.lock.Renew(ctx)
			if err != nil {
				return err
			}
			renewTimer.Reset(hc.renewalPeriod)
		}
	}

}

func (hc *HAController) wait(ctx context.Context) {
	t := time.NewTimer(time.Duration(hc.ranGenerator.Intn(100)) * time.Millisecond)
	defer t.Stop()

	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
