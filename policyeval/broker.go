// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package policyeval

import (
	"container/heap"
	"context"
	"errors"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/uuid"
)

// Broker stores, dedups and control access to policy evaluation requests.
//
// A few notes on the inner workings of the broker:
//
//   - `pendingEvals` has a key for every policy type (queue) and a heap of
//     evals as value.
//
//   - `enqueuedEvals` has an eval ID as key and a dequeue counter as value.
//
//   - `enqueuedPolicies` has a policy ID as key and the corresponding eval ID
//     as value.
//
//   - evals are either in `pendingEvals` or `unack`, but never in both.
//
//   - evals start in `pendingEvals` and move to `unack` when dequeued.
//
//   - evals in `unack` are removed when the eval is ACK'd or the delivery
//     limit is reached.
//
//   - evals in `unack` are moved back to `pendingEvals` if the eval is NACK'd.
//
//   - the eval is updated if a new eval is enqueued for the same policy.
//
//   - eval IDs are stored as keys in `enqueuedEvals` when an eval is enqueued.
//
//   - the eval ID key remains in `enqueuedEvals` until the eval is ACK'd, the
//     delivery limit is reached or a newer eval for the same policy is
//     enqueued.
//
//   - the value for an entry in `enqueuedEvals` is incremented by one every
//     time the corresponding policy is dequeued.
//
//   - policy IDs are stored as keys in `enqueuedPolicies` when an eval is
//     enqueued.
//
//   - the policy ID remains in `enqueuedPolicies` until the eval is ACK'd or
//     the delivery limit is reached.
//
//   - the value for the policy ID is the ID of the eval that was enqueued.
//
//   - the value for the policy ID is updated if a newer eval for the policy is
//     enqueued.
type Broker struct {
	logger hclog.Logger

	// l is the lock used control access to all internal state of the broker.
	l sync.RWMutex

	// nackTimeout is the time required for a dequeued eval to be ack'd.
	nackTimeout time.Duration

	// deliveryLimit is the number of times an eval can be dequeued before
	// being considered as failed.
	deliveryLimit int

	// pendingEvals holds evaluations that are ready to be picked for
	// further evaluation.
	// Each key represents a different queue and the value is a heap holding
	// the evals in order of priority and create time.
	pendingEvals map[string]PendingEvaluations

	// enqueuedEvals and enqueuedPolicies are indexes to quickly find if an eval
	// or a policy is waiting be evaluated.
	enqueuedEvals    map[string]int
	enqueuedPolicies map[string]string

	// unack tracks evaluations that have not been ack'd yet.
	unack map[string]*unackEval

	// waiting tracks Dequeue requests that are blocked waiting for work.
	waiting map[string]chan struct{}
}

// unackEval tracks an unacknowledged evaluation along with the Nack timer
type unackEval struct {
	Eval      *sdk.ScalingEvaluation
	Token     string
	NackTimer *time.Timer
}

// NewBroker returns a new Broker object.
func NewBroker(l hclog.Logger, timeout time.Duration, deliveryLimit int) *Broker {
	return &Broker{
		logger:           l.Named("broker"),
		nackTimeout:      timeout,
		deliveryLimit:    deliveryLimit,
		pendingEvals:     make(map[string]PendingEvaluations),
		enqueuedEvals:    make(map[string]int),
		enqueuedPolicies: make(map[string]string),
		unack:            make(map[string]*unackEval),
		waiting:          make(map[string]chan struct{}),
	}
}

// Enqueue adds an eval to the broker.
func (b *Broker) Enqueue(eval *sdk.ScalingEvaluation) {
	b.l.Lock()
	defer b.l.Unlock()
	b.enqueueLocked(eval, "")
}

func (b *Broker) enqueueLocked(eval *sdk.ScalingEvaluation, token string) {
	logger := b.logger.With(
		"eval_id", eval.ID, "policy_id", eval.Policy.ID,
		"queue", eval.Policy.Type, "token", token)

	logger.Debug("enqueue eval")

	// Check if eval is already enqueued.
	if _, ok := b.enqueuedEvals[eval.ID]; ok {
		if token == "" {
			logger.Debug("eval already enqueued")
			return
		}
	} else {
		b.enqueuedEvals[eval.ID] = 0
	}

	queue := eval.Policy.Type

	// Get pending heap for the policy type.
	pending, ok := b.pendingEvals[queue]
	if !ok {
		// Initialize pending heap and waiting channel.
		pending = PendingEvaluations{}
		if _, ok := b.waiting[queue]; !ok {
			b.waiting[queue] = make(chan struct{}, 1)
		}
	}

	// Check if an eval for the same policy is already enqueued.
	pendingEvalID, ok := b.enqueuedPolicies[eval.Policy.ID]
	if !ok {
		b.enqueuedPolicies[eval.Policy.ID] = eval.ID
	} else if pendingEvalID != eval.ID {
		logger.Debug("policy already enqueued")

		// Policy is waiting to be evaluated, but this could be a newer
		// evaluation request and the policy could have changed. So update
		// the pending heap with the new eval.
		i, pendingEval := pending.GetEvaluation(pendingEvalID)
		if pendingEval != nil {
			if eval.CreateTime.After(pendingEval.CreateTime) {
				logger.Debug("new eval is newer, policy updated")
				b.enqueuedPolicies[eval.Policy.ID] = eval.ID
				delete(b.enqueuedEvals, eval.ID)
				pending[i] = eval
				heap.Fix(&pending, i)
			}
		}
		return
	}

	heap.Push(&pending, eval)
	b.pendingEvals[queue] = pending

	// Unblock any blocked dequeues.
	select {
	case b.waiting[queue] <- struct{}{}:
	default:
	}

	logger.Debug("eval enqueued")
}

// Dequeue is used to retrieve an eval from the broker.
func (b *Broker) Dequeue(ctx context.Context, queue string) (*sdk.ScalingEvaluation, string, error) {
	logger := b.logger.With("queue", queue)

	logger.Debug("dequeue eval")

	eval := b.findWork(queue)
	for eval == nil {
		proceed := b.waitForWork(ctx, queue)
		if !proceed {
			return nil, "", nil
		}

		eval = b.findWork(queue)
	}

	b.l.Lock()
	defer b.l.Unlock()

	// Generate an UUID for the token.
	token := uuid.Generate()

	// Setup Nack timer.
	// Eval needs to be Ack'd before this timer finishes.
	nackTimer := time.AfterFunc(b.nackTimeout, func() {
		if err := b.Nack(eval.ID, token); err != nil {
			logger.Warn("failed to nack eval", "error", err.Error())
		}
	})

	// Mark eval as not Ack'd yet.
	b.unack[eval.ID] = &unackEval{
		Eval:      eval,
		Token:     token,
		NackTimer: nackTimer,
	}

	// Increment dequeue counter.
	b.enqueuedEvals[eval.ID] += 1

	logger.Debug("eval dequeued",
		"eval_id", eval.ID, "policy_id", eval.Policy.ID, "token", token)
	return eval, token, nil
}

// findWork returns an eval from the queue heap or nil if there's no eval available.
func (b *Broker) findWork(queue string) *sdk.ScalingEvaluation {
	b.l.Lock()
	defer b.l.Unlock()

	pending, ok := b.pendingEvals[queue]
	if !ok {
		return nil
	}

	if pending.Len() == 0 {
		return nil
	}

	// Pop heap and update reference.
	raw := heap.Pop(&pending)
	b.pendingEvals[queue] = pending

	return raw.(*sdk.ScalingEvaluation)
}

// waitForWork blocks until queue receives an item or the context is canceled.
func (b *Broker) waitForWork(ctx context.Context, queue string) (proceed bool) {
	b.logger.Debug("waiting for eval", "queue", queue)

	b.l.Lock()
	waitCh, ok := b.waiting[queue]
	if !ok {
		waitCh = make(chan struct{}, 1)
		b.waiting[queue] = waitCh
	}
	b.l.Unlock()

	select {
	case <-ctx.Done():
		return false
	case <-waitCh:
		return true
	}
}

// Ack is used to acknowledge the successful completion of an eval.
func (b *Broker) Ack(evalID, token string) error {
	logger := b.logger.With("eval_id", evalID, "token", token)

	logger.Debug("ack eval", "eval_id", evalID, "token", token)

	b.l.Lock()
	defer b.l.Unlock()

	// Lookup the unack'd eval.
	unack, ok := b.unack[evalID]
	if !ok {
		return errors.New("evaluation ID not found")
	}
	if unack.Token != token {
		return errors.New("token does not match for evaluation ID")
	}

	// Ensure we were able to stop the timer.
	if !unack.NackTimer.Stop() {
		return errors.New("evaluation ID Ack'd after Nack timer expiration")
	}

	// Cleanup.
	delete(b.unack, evalID)
	delete(b.enqueuedEvals, evalID)
	delete(b.enqueuedPolicies, unack.Eval.Policy.ID)

	b.logger.Debug("eval ack'd", "policy_id", unack.Eval.Policy.ID)
	return nil
}

// Nack is used to mark an eval as not completed.
func (b *Broker) Nack(evalID, token string) error {
	logger := b.logger.With("eval_id", evalID, "token", token)

	logger.Debug("nack eval")

	b.l.Lock()
	defer b.l.Unlock()

	// Lookup the unack'd eval
	unack, ok := b.unack[evalID]
	if !ok {
		return errors.New("evaluation ID not found")
	}
	if unack.Token != token {
		return errors.New("token does not match for evaluation ID")
	}

	logger = logger.With("policy_id", unack.Eval.Policy.ID)

	// Stop the timer, doesn't matter if we've missed it.
	unack.NackTimer.Stop()

	// Cleanup.
	delete(b.unack, evalID)

	// Check if we've hit the delivery limit.
	if dequeues := b.enqueuedEvals[evalID]; dequeues >= b.deliveryLimit {
		logger.Warn("eval delivery limit reached", "count", dequeues, "limit", b.deliveryLimit)

		delete(b.enqueuedEvals, evalID)
		delete(b.enqueuedPolicies, unack.Eval.Policy.ID)
		return nil
	}

	// Re-enqueue eval to try again.
	b.enqueueLocked(unack.Eval, token)
	logger.Info("eval nack'd, retrying it")
	return nil
}

// PendingEvaluations is a list of waiting evaluations.
// We implement the container/heap interface so that this is a
// priority queue
type PendingEvaluations []*sdk.ScalingEvaluation

// Len is for the sorting interface
func (p PendingEvaluations) Len() int {
	return len(p)
}

// Less is for the sorting interface. We flip the check
// so that the "min" in the min-heap is the element with the
// highest priority
func (p PendingEvaluations) Less(i, j int) bool {
	if p[i].Policy.Priority != p[j].Policy.Priority {
		return !(p[i].Policy.Priority < p[j].Policy.Priority)
	}
	return p[i].CreateTime.Before(p[j].CreateTime)
}

// Swap is for the sorting interface
func (p PendingEvaluations) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

// Push is used to add a new evaluation to the slice
func (p *PendingEvaluations) Push(e interface{}) {
	*p = append(*p, e.(*sdk.ScalingEvaluation))
}

// Pop is used to remove an evaluation from the slice
func (p *PendingEvaluations) Pop() interface{} {
	n := len(*p)
	e := (*p)[n-1]
	(*p)[n-1] = nil
	*p = (*p)[:n-1]
	return e
}

func (p PendingEvaluations) GetEvaluation(evalID string) (int, *sdk.ScalingEvaluation) {
	for i, eval := range p {
		if eval.ID == evalID {
			return i, eval
		}
	}

	return -1, nil
}
