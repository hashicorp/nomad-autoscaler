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

	// enqueuedEvals and enqueuedPolicies are indexes to quick find if an eval
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
	b.enqueueLocked(eval, "")
}

func (b *Broker) enqueueLocked(eval *sdk.ScalingEvaluation, token string) {
	b.logger.Debug("enqueue", "eval_id", eval.ID, "policy_id", eval.Policy.ID, "token", token)

	// Check if eval is already enqueued.
	if _, ok := b.enqueuedEvals[eval.ID]; ok {
		if token == "" {
			b.logger.Debug("already enqueued")
			return
		}
	} else {
		b.enqueuedEvals[eval.ID] = 0
	}

	// Get pending heap for the policy type.
	queue := eval.Policy.Type
	pending, ok := b.pendingEvals[eval.Policy.Type]
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
	} else {
		if pendingEvalID != eval.ID {
			// Policy is waiting to be evaluated, but this could be a newer
			// evaluation request and the policy could have changed.
			// So update the pending heap with the new eval.
			i, pendingEval := pending.GetEvaluation(pendingEvalID)
			if pendingEval != nil && eval.CreateTime.After(pendingEval.CreateTime) {
				pending[i] = eval
				heap.Fix(&pending, i)
				return
			}
		}
	}

	heap.Push(&pending, eval)
	b.pendingEvals[queue] = pending

	// Unblock any blocked dequeues
	select {
	case b.waiting[queue] <- struct{}{}:
	default:
	}

	b.logger.Debug("enqueued", "eval_id", eval.ID, "policy_id", eval.Policy.ID)
}

func (b *Broker) Dequeue(ctx context.Context, queue string) (*sdk.ScalingEvaluation, string, error) {
	b.logger.Debug("dequeue", "queue", queue)

	eval := b.findWork(queue)
	for eval == nil {
		b.logger.Debug("eval is nil")
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
		b.Nack(eval.ID, token)
	})

	// Mark eval as not Ack'd yet.
	b.unack[eval.ID] = &unackEval{
		Eval:      eval,
		Token:     token,
		NackTimer: nackTimer,
	}

	// Increment dequeue counter.
	b.enqueuedEvals[eval.ID] += 1

	b.logger.Debug("dequeued", "eval_id", eval.ID, "queue", queue, "token", token)
	return eval, token, nil
}

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

func (b *Broker) waitForWork(ctx context.Context, queue string) bool {
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

func (b *Broker) Ack(evalID, token string) error {
	b.l.Lock()
	defer b.l.Unlock()

	b.logger.Debug("ack", "eval_id", evalID, "token", token)

	// Lookup the unack'd eval.
	unack, ok := b.unack[evalID]
	if !ok {
		return fmt.Errorf("evaluation ID not found")
	}
	if unack.Token != token {
		return fmt.Errorf("token does not match for evaluation ID")
	}

	// Ensure we were able to stop the timer.
	if !unack.NackTimer.Stop() {
		return fmt.Errorf("evaluation ID Ack'd after Nack timer expiration")
	}

	// Cleanup.
	delete(b.unack, evalID)
	delete(b.enqueuedEvals, evalID)
	delete(b.enqueuedPolicies, unack.Eval.Policy.ID)

	b.logger.Debug("ack'd", "eval_id", evalID, "policy_id", unack.Eval.Policy.ID, "token", token)
	return nil
}

func (b *Broker) Nack(evalID, token string) error {
	b.l.Lock()
	defer b.l.Unlock()

	b.logger.Debug("nack", "eval_id", evalID)

	// Lookup the unack'd eval
	unack, ok := b.unack[evalID]
	if !ok {
		return fmt.Errorf("evaluation ID not found")
	}
	if unack.Token != token {
		return fmt.Errorf("token does not match for evaluation ID")
	}

	// Stop the timer, doesn't matter if we've missed it.
	unack.NackTimer.Stop()

	// Cleanup.
	delete(b.unack, evalID)

	// Check if we've hit the delivery limit.
	if dequeues := b.enqueuedEvals[evalID]; dequeues >= b.deliveryLimit {
		b.logger.Debug("eval delivery limit reached", "eval_id", evalID, "policy_id", unack.Eval.Policy.ID, "count", dequeues, "limit", b.deliveryLimit)
		// TODO(luiz): probably need a failed queue as well
		return nil
	}

	// Re-enqueue eval to try again.
	b.enqueueLocked(unack.Eval, token)
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

func (p PendingEvaluations) GetEvaluation(evalID string) (int, *sdk.ScalingEvaluation) {
	for i, eval := range p {
		if eval.ID == evalID {
			return i, eval
		}
	}

	return -1, nil
}
