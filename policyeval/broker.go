package policyeval

import (
	"container/heap"
	"context"
	"fmt"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/helper/uuid"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

type Broker struct {
	logger hclog.Logger

	l sync.RWMutex

	nackTimeout   time.Duration
	deliveryLimit int

	// h is a heap that holds evaluations that are ready to be evaluated.
	pendingEvals map[string]PendingEvaluations

	enqueuedEvals    map[string]int
	enqueuedPolicies map[string]string

	// unack is a map of evalID to an un-acknowledged evaluation
	unack map[string]*unackEval

	waiting map[string]chan struct{}
}

// unackEval tracks an unacknowledged evaluation along with the Nack timer
type unackEval struct {
	Eval      *sdk.ScalingEvaluation
	Token     string
	NackTimer *time.Timer
}

func NewBroker(l hclog.Logger, timeout time.Duration, deliveryLimit int) *Broker {
	return &Broker{
		logger:           l.Named("eval_broker"),
		nackTimeout:      timeout,
		deliveryLimit:    deliveryLimit,
		pendingEvals:     make(map[string]PendingEvaluations),
		enqueuedEvals:    make(map[string]int),
		enqueuedPolicies: make(map[string]string),
		unack:            make(map[string]*unackEval),
		waiting:          make(map[string]chan struct{}),
	}
}

func (b *Broker) Enqueue(eval *sdk.ScalingEvaluation) {
	b.l.Lock()
	defer b.l.Unlock()

	b.logger.Info("enqueue", "eval_id", eval.ID, "policy_id", eval.Policy.ID)

	// Check if eval is already enqueued.
	if _, ok := b.enqueuedEvals[eval.ID]; ok {
		return
	}

	b.enqueuedEvals[eval.ID] = 0

	// Get pending heap for the policy type.
	queue := eval.Policy.Type
	pending, ok := b.pendingEvals[eval.Policy.Type]
	if !ok {
		pending = PendingEvaluations{}
		if _, ok := b.waiting[queue]; !ok {
			b.waiting[queue] = make(chan struct{}, 1)
		}
	}

	// Check if an eval for the same is already enqueued.
	pendingEvalID := b.enqueuedPolicies[eval.Policy.ID]

	if pendingEvalID == "" {
		b.enqueuedPolicies[eval.Policy.ID] = eval.ID
	} else if pendingEvalID != eval.ID {
		// Policy is already in an eval waiting to be evaluated, but this is a
		// new eval, and the policy may have changed. So update the eval in the
		// pending heap.
		i := pending.IndexOfPolicyID(eval.Policy.ID)
		if i >= 0 {
			pending[i] = eval
			heap.Fix(&pending, i)
			return
		}
	}

	heap.Push(&pending, eval)
	b.pendingEvals[queue] = pending

	// Unblock any blocked dequeues
	select {
	case b.waiting[queue] <- struct{}{}:
	default:
	}
}

func (b *Broker) Dequeue(ctx context.Context, queue string) (*sdk.ScalingEvaluation, string, error) {
	for {
		pending, ok := b.pendingEvals[queue]
		if !ok {
			return nil, "", nil
		}

		// Pop heap and update reference.
		raw := heap.Pop(&pending)
		b.pendingEvals[queue] = pending
		eval := raw.(*sdk.ScalingEvaluation)

		// Generate a UUID for the token.
		token := uuid.Generate()

		// Setup Nack timer.
		nackTimer := time.AfterFunc(b.nackTimeout, func() {
			b.Nack(eval.ID, token)
		})

		// Add to the unack queue.
		b.unack[eval.ID] = &unackEval{
			Eval:      eval,
			Token:     token,
			NackTimer: nackTimer,
		}

		if eval != nil {
			return eval, token, nil
		}

		// Otherwise block until there's work or ctx is canceled.
		b.l.Lock()
		waitCh, ok := b.waiting[queue]
		if !ok {
			waitCh = make(chan struct{}, 1)
			b.waiting[queue] = waitCh
		}
		b.l.Unlock()

		select {
		case <-ctx.Done():
			return nil, "", nil
		case <-waitCh:
		}
	}
}

func (b *Broker) Ack(evalID, token string) error {
	b.l.Lock()
	defer b.l.Unlock()

	// Lookup the unack'd eval.
	unack, ok := b.unack[evalID]
	if !ok {
		return fmt.Errorf("Evaluation ID not found")
	}
	if unack.Token != token {
		return fmt.Errorf("Token does not match for Evaluation ID")
	}

	// Ensure we were able to stop the timer
	if !unack.NackTimer.Stop() {
		return fmt.Errorf("Evaluation ID Ack'd after Nack timer expiration")
	}

	// Cleanup
	delete(b.unack, evalID)
	delete(b.enqueuedEvals, evalID)
	delete(b.enqueuedPolicies, unack.Eval.Policy.ID)

	return nil
}

func (b *Broker) Nack(evalID, token string) error {
	b.l.Lock()
	defer b.l.Unlock()

	// Lookup the unack'd eval
	unack, ok := b.unack[evalID]
	if !ok {
		return fmt.Errorf("Evaluation ID not found")
	}
	if unack.Token != token {
		return fmt.Errorf("Token does not match for Evaluation ID")
	}

	// Stop the timer, doesn't matter if we've missed it
	unack.NackTimer.Stop()

	// Cleanup
	delete(b.unack, evalID)
	delete(b.enqueuedEvals, evalID)
	delete(b.enqueuedPolicies, unack.Eval.Policy.ID)

	// Check if we've hit the delivery limit
	if dequeues := b.enqueuedEvals[evalID]; dequeues >= b.deliveryLimit {
		// TODO(luiz): probably need a failed queue as well
		return fmt.Errorf("eval delivery retry limit reached")
	}

	go b.Enqueue(unack.Eval)

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

// Peek is used to peek at the next element that would be popped
func (p PendingEvaluations) Peek() *sdk.ScalingEvaluation {
	n := len(p)
	if n == 0 {
		return nil
	}
	return p[n-1]
}

// IndexOfPolicyID returns the index of the policy eval with a give policy ID.
func (p PendingEvaluations) IndexOfPolicyID(policyID string) int {
	for i, eval := range p {
		if eval.Policy.ID == policyID {
			return i
		}
	}

	return -1
}
